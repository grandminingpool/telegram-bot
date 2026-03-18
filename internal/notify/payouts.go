package bot_notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pool_payouts_proto "github.com/grandminingpool/pool-api-proto/generated/pool_payouts"
	filters_proto "github.com/grandminingpool/pool-api-proto/generated/utils/filters"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	format_utils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PayoutInfo struct {
	amount uint64
	txHash string
	paidAt time.Time
}

type SoloPayoutInfo struct {
	reward    uint64
	blockHash string
	txHash    string
	paidAt    time.Time
	minedAt   time.Time
}

type UserPayouts struct {
	userInfo   *UserInfo
	walletInfo *WalletInfo
}

type UserWalletPayouts struct {
	UserPayouts
	payouts []*PayoutInfo
}

type UserWalletSoloPayouts struct {
	UserPayouts
	payouts []*SoloPayoutInfo
}

type PoolPayouts struct {
	groupNum int
	coin     string
	payouts  map[string]*pool_payouts_proto.Payouts
	err      error
}

type PoolSoloPayouts struct {
	groupNum int
	coin     string
	payouts  map[string]*pool_payouts_proto.MinedSoloBlocks
	err      error
}

type UserWallet struct {
	userInfo *UserInfo
	id       int64
	payouts  bool
	blocks   bool
}

type PoolPayoutsRequests struct {
	client      pool_payouts_proto.PoolPayoutsServiceClient
	wallets     [][]string
	soloWallets [][]string
}

type Payouts struct {
	pgConn             *pgxpool.Pool
	blockchainsService *blockchains.Service
	languages          *languages.Languages
	b                  *bot.Bot
	config             *bot_config.NotifyConfig
}

func (p *Payouts) getLastExecutionTime(ctx context.Context) (*time.Time, error) {
	var lastExecutionTime time.Time
	err := p.pgConn.QueryRow(ctx, `SELECT
		executed_at FROM payouts_notifications
	ORDER BY executed_at DESC LIMIT 1`).Scan(&lastExecutionTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query last payouts notification executed time: %w", err)
	}

	return &lastExecutionTime, nil
}

func (p *Payouts) addNotification(ctx context.Context) error {
	if _, err := p.pgConn.Exec(ctx, "INSERT INTO payouts_notifications (executed_at) VALUES (NOW())"); err != nil {
		return fmt.Errorf("failed to create new payouts notification: %w", err)
	}

	return nil
}

func (p *Payouts) getWalletsMap(ctx context.Context) (map[string]map[string]*UserWallet, error) {
	walletsMap := make(map[string]map[string]*UserWallet)
	rows, err := p.pgConn.Query(ctx, `SELECT
		user_wallets.user_id,
		users.chat_id,
		users.lang,
		users.payouts_notify,
		users.blocks_notify,
		user_wallets.blockchain_coin,
		user_wallets.id,
		user_wallets.wallet
	FROM user_wallets
	LEFT JOIN users ON users.id = user_wallets.user_id
	WHERE users.blocks_notify = true OR users.payouts_notify = true`)
	if err != nil {
		return nil, fmt.Errorf("failed to query wallets for payouts notifications: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID, chatID, walletID    int64
			userLang, coin, wallet      string
			payoutsNotify, blocksNotify bool
		)

		if err := rows.Scan(
			&userID,
			&chatID,
			&userLang,
			&payoutsNotify,
			&blocksNotify,
			&coin,
			&walletID,
			&wallet,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payouts wallets columns: %w", err)
		}

		_, ok := walletsMap[coin]
		if !ok {
			walletsMap[coin] = make(map[string]*UserWallet)
		}

		walletsMap[coin][wallet] = &UserWallet{
			userInfo: &UserInfo{
				userID: userID,
				chatID: chatID,
				lang:   userLang,
			},
			id:      walletID,
			payouts: payoutsNotify,
			blocks:  blocksNotify,
		}
	}

	return walletsMap, nil
}

func (w *Payouts) getPoolRequestsMap(walletsMap map[string]map[string]*UserWallet) (map[string]*PoolPayoutsRequests, int, int, error) {
	poolRequestsMap := make(map[string]*PoolPayoutsRequests)
	requestsCount := 0
	soloRequestsCount := 0
	for coin, coinWalletsMap := range walletsMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, 0, 0, err
		}

		client := pool_payouts_proto.NewPoolPayoutsServiceClient(conn)
		poolRequests := &PoolPayoutsRequests{
			client:      client,
			wallets:     [][]string{{}},
			soloWallets: [][]string{{}},
		}

		groupNum, soloGroupNum := 0, 0
		requestsCount++
		soloRequestsCount++
		for wallet, userWallet := range coinWalletsMap {
			if userWallet.payouts {
				if len(poolRequests.wallets[groupNum]) >= w.config.MaxWalletsInPayoutsRequest {
					groupNum++
					requestsCount++
					poolRequests.wallets = append(poolRequests.wallets, []string{})
				}

				poolRequests.wallets[groupNum] = append(poolRequests.wallets[groupNum], wallet)
			}

			if userWallet.blocks {
				if len(poolRequests.soloWallets[soloGroupNum]) >= w.config.MaxWalletsInWorkersRequest {
					soloGroupNum++
					soloRequestsCount++
					poolRequests.soloWallets = append(poolRequests.soloWallets, []string{})
				}

				poolRequests.soloWallets[soloGroupNum] = append(poolRequests.soloWallets[soloGroupNum], wallet)
			}
		}

		poolRequestsMap[coin] = poolRequests
	}

	return poolRequestsMap, requestsCount, soloRequestsCount, nil
}

func (p *Payouts) getSoloPayouts(
	ctx context.Context,
	client pool_payouts_proto.PoolPayoutsServiceClient,
	coin string,
	groupNum int,
	wallets []string,
	paidFrom time.Time,
	resultCh chan<- PoolSoloPayouts,
) {
	select {
	case <-ctx.Done():
		return
	default:
		result := PoolSoloPayouts{
			groupNum: groupNum,
			coin:     coin,
			payouts:  nil,
			err:      nil,
		}

		soloPayouts, err := client.GetSoloBlocksFromList(ctx, &pool_payouts_proto.GetSoloBlocksFromListRequest{
			Miners: wallets,
			Filters: &pool_payouts_proto.MinedSoloBlocksFilters{
				MinedAt: &filters_proto.DateTimeRangeFilter{
					Start: timestamppb.New(paidFrom),
				},
			},
		})
		if err != nil {
			result.err = fmt.Errorf("failed to get pool solo payouts: %w", err)
		} else {
			result.payouts = soloPayouts.Blocks
		}

		resultCh <- result
	}
}

func (p *Payouts) getPayouts(
	ctx context.Context,
	client pool_payouts_proto.PoolPayoutsServiceClient,
	coin string,
	groupNum int,
	wallets []string,
	paidFrom time.Time,
	resultCh chan<- PoolPayouts,
) {
	select {
	case <-ctx.Done():
		return
	default:
		result := PoolPayouts{
			groupNum: groupNum,
			coin:     coin,
			payouts:  nil,
			err:      nil,
		}

		payouts, err := client.GetPayoutsFromList(ctx, &pool_payouts_proto.GetPayoutsFromListRequest{
			Miners: wallets,
			Filters: &pool_payouts_proto.PayoutsFilters{
				PaidAt: &filters_proto.DateTimeRangeFilter{
					Start: timestamppb.New(paidFrom),
				},
			},
		})
		if err != nil {
			result.err = fmt.Errorf("failed to get pool payouts: %w", err)
		} else {
			result.payouts = payouts.Payouts
		}

		resultCh <- result
	}
}

func (p *Payouts) notifyUsersPayments(
	ctx context.Context,
	usersWalletsPayouts []*UserWalletPayouts,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	select {
	case <-ctx.Done():
		return
	default:
		var msgBuf bytes.Buffer
		for _, userWalletPayouts := range usersWalletsPayouts {
			userLocalizer := p.languages.GetLocalizer(userWalletPayouts.userInfo.lang)

			for _, userPayoutInfo := range userWalletPayouts.payouts {
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "NewPayoutReceived",
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WalletInfo",
					TemplateData: map[string]string{
						"Wallet":             userWalletPayouts.walletInfo.wallet,
						"PoolBlockchainName": userWalletPayouts.walletInfo.blockchain.Name,
					},
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "PayoutInfo",
					TemplateData: map[string]string{
						"Amount": format_utils.WalletBalance(userPayoutInfo.amount, userWalletPayouts.walletInfo.blockchain.AtomicUnit),
						"Ticker": userWalletPayouts.walletInfo.blockchain.Ticker,
						"TxHash": userPayoutInfo.txHash,
						"PaidAt": userPayoutInfo.paidAt.Format(time.RFC3339),
					},
				}))

				p.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    userWalletPayouts.userInfo.chatID,
					ParseMode: models.ParseModeHTML,
					Text:      msgBuf.String(),
				})

				msgBuf.Reset()
			}
		}
	}
}

func (p *Payouts) notifyUsersSoloPayments(
	ctx context.Context,
	usersWalletsSoloPayouts []*UserWalletSoloPayouts,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	select {
	case <-ctx.Done():
		return
	default:
		var msgBuf bytes.Buffer
		for _, userWalletSoloPayouts := range usersWalletsSoloPayouts {
			userLocalizer := p.languages.GetLocalizer(userWalletSoloPayouts.userInfo.lang)

			for _, userSoloPayoutInfo := range userWalletSoloPayouts.payouts {
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "NewBlockFound",
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WalletInfo",
					TemplateData: map[string]string{
						"Wallet":             userWalletSoloPayouts.walletInfo.wallet,
						"PoolBlockchainName": userWalletSoloPayouts.walletInfo.blockchain.Name,
					},
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "SoloPayoutInfo",
					TemplateData: map[string]string{
						"Reward":    format_utils.WalletBalance(userSoloPayoutInfo.reward, userWalletSoloPayouts.walletInfo.blockchain.AtomicUnit),
						"Ticker":    userWalletSoloPayouts.walletInfo.blockchain.Ticker,
						"BlockHash": userSoloPayoutInfo.blockHash,
						"TxHash":    userSoloPayoutInfo.txHash,
						"PaidAt":    userSoloPayoutInfo.paidAt.Format(time.RFC3339),
						"MinedAt":   userSoloPayoutInfo.minedAt.Format(time.RFC3339),
					},
				}))

				p.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    userWalletSoloPayouts.userInfo.chatID,
					ParseMode: models.ParseModeHTML,
					Text:      msgBuf.String(),
				})

				msgBuf.Reset()
			}
		}
	}
}

func (p *Payouts) Check(ctx context.Context) {
	zap.L().Debug("payouts check started")

	lastExecutionTime, err := p.getLastExecutionTime(ctx)
	if err != nil {
		zap.L().Error("failed to get last payments notification executed time", zap.Error(err))

		return
	}

	if lastExecutionTime == nil {
		zap.L().Debug("payouts check: first run, initializing notification timestamp")

		if err := p.addNotification(ctx); err != nil {
			zap.L().Error("failed to add first payments notification to db", zap.Error(err))
		}

		return
	}

	zap.L().Debug("payouts check: last execution time", zap.Time("last_execution", *lastExecutionTime))

	walletsMap, err := p.getWalletsMap(ctx)
	defer clear(walletsMap)
	if err != nil {
		zap.L().Error("failed to get wallets map", zap.Error(err))

		return
	}

	zap.L().Debug("payouts check: loaded wallets map", zap.Int("coins_count", len(walletsMap)))

	poolRequestsMap, requestsCount, soloRequestsCount, err := p.getPoolRequestsMap(walletsMap)
	defer clear(poolRequestsMap)
	if err != nil {
		zap.L().Error("failed to create pool requests map", zap.Error(err))

		return
	}

	zap.L().Debug("payouts check: pool requests prepared",
		zap.Int("payout_requests", requestsCount),
		zap.Int("solo_requests", soloRequestsCount),
	)

	poolPayoutsCh := make(chan PoolPayouts, requestsCount)
	poolSoloPayoutsCh := make(chan PoolSoloPayouts, soloRequestsCount)
	defer close(poolPayoutsCh)
	defer close(poolSoloPayoutsCh)
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for coin, poolRequests := range poolRequestsMap {
		client := poolRequests.client

		for groupNum := 0; groupNum < len(poolRequests.wallets); groupNum++ {
			go p.getPayouts(
				newCtx,
				client,
				coin,
				groupNum,
				poolRequests.wallets[groupNum],
				*lastExecutionTime,
				poolPayoutsCh,
			)
		}

		for soloGroupNum := 0; soloGroupNum < len(poolRequests.soloWallets); soloGroupNum++ {
			go p.getSoloPayouts(
				newCtx,
				client,
				coin,
				soloGroupNum,
				poolRequests.soloWallets[soloGroupNum],
				*lastExecutionTime,
				poolSoloPayoutsCh,
			)
		}
	}

	payoutsMap := make(map[UserInfo]map[WalletInfo][]*PayoutInfo)
	soloPayoutsMap := make(map[UserInfo]map[WalletInfo][]*SoloPayoutInfo)
	defer clear(payoutsMap)
	defer clear(soloPayoutsMap)
	for i := 0; i < (requestsCount + soloRequestsCount); i++ {
		select {
		case <-ctx.Done():
			return
		case poolPayouts := <-poolPayoutsCh:
			if poolPayouts.err != nil {
				zap.L().Error("get pool payouts error",
					zap.String("coin", poolPayouts.coin),
					zap.Int("group_num", poolPayouts.groupNum),
					zap.Error(poolPayouts.err),
				)

				return
			}

			blockchain, err := p.blockchainsService.GetInfo(poolPayouts.coin)
			if err != nil {
				zap.L().Error("get blockchain info for payouts error",
					zap.String("coin", poolPayouts.coin),
					zap.Int("group_num", poolPayouts.groupNum),
					zap.Error(err),
				)

				return
			}

			coinWalletsMap, ok := walletsMap[poolPayouts.coin]
			if ok {
				for wallet, walletPayouts := range poolPayouts.payouts {
					userWallet, ok := coinWalletsMap[wallet]
					if ok {
						walletInfo := WalletInfo{
							id:         userWallet.id,
							wallet:     wallet,
							blockchain: &blockchain,
						}
						userWalletPayouts := make([]*PayoutInfo, 0, len(walletPayouts.Payouts))
						for _, walletPayout := range walletPayouts.Payouts {
							userWalletPayouts = append(userWalletPayouts, &PayoutInfo{
								amount: walletPayout.Amount,
								txHash: walletPayout.TxHash,
								paidAt: walletPayout.PaidAt.AsTime(),
							})
						}

						userPayoutsMap, ok := payoutsMap[*userWallet.userInfo]
						if !ok {
							userPayoutsMap = make(map[WalletInfo][]*PayoutInfo)
							payoutsMap[*userWallet.userInfo] = userPayoutsMap
						}
						userPayoutsMap[walletInfo] = userWalletPayouts
					}
				}
			}
		case poolSoloPayouts := <-poolSoloPayoutsCh:
			if poolSoloPayouts.err != nil {
				zap.L().Error("get pool solo payouts error",
					zap.String("coin", poolSoloPayouts.coin),
					zap.Int("group_num", poolSoloPayouts.groupNum),
					zap.Error(poolSoloPayouts.err),
				)

				return
			}

			blockchain, err := p.blockchainsService.GetInfo(poolSoloPayouts.coin)
			if err != nil {
				zap.L().Error("get blockchain info for solo payouts error",
					zap.String("coin", poolSoloPayouts.coin),
					zap.Int("group_num", poolSoloPayouts.groupNum),
					zap.Error(err),
				)

				return
			}

			coinWalletsMap, ok := walletsMap[poolSoloPayouts.coin]
			if ok {
				for wallet, walletSoloPayouts := range poolSoloPayouts.payouts {
					userWallet, ok := coinWalletsMap[wallet]
					if ok {
						walletInfo := WalletInfo{
							id:         userWallet.id,
							wallet:     wallet,
							blockchain: &blockchain,
						}
						userWalletSoloPayouts := make([]*SoloPayoutInfo, 0, len(walletSoloPayouts.Blocks))
						for _, walletSoloPayout := range walletSoloPayouts.Blocks {
							userWalletSoloPayouts = append(userWalletSoloPayouts, &SoloPayoutInfo{
								reward:    walletSoloPayout.Reward,
								blockHash: walletSoloPayout.BlockHash,
								txHash:    walletSoloPayout.TxHash,
								paidAt:    walletSoloPayout.PaidAt.AsTime(),
								minedAt:   walletSoloPayout.MinedAt.AsTime(),
							})
						}

						userSoloPayoutsMap, ok := soloPayoutsMap[*userWallet.userInfo]
						if !ok {
							userSoloPayoutsMap = make(map[WalletInfo][]*SoloPayoutInfo)
							soloPayoutsMap[*userWallet.userInfo] = userSoloPayoutsMap
						}
						userSoloPayoutsMap[walletInfo] = userWalletSoloPayouts
					}
				}
			}
		}
	}

	totalPayouts, totalSoloPayouts := 0, 0
	for _, userPayoutsMap := range payoutsMap {
		for _, payouts := range userPayoutsMap {
			totalPayouts += len(payouts)
		}
	}
	for _, userSoloPayoutsMap := range soloPayoutsMap {
		for _, soloPayouts := range userSoloPayoutsMap {
			totalSoloPayouts += len(soloPayouts)
		}
	}

	zap.L().Debug("payouts check: changes detected",
		zap.Int("payouts", totalPayouts),
		zap.Int("solo_payouts", totalSoloPayouts),
		zap.Int("users_with_payouts", len(payoutsMap)),
		zap.Int("users_with_solo_payouts", len(soloPayoutsMap)),
	)

	usersWalletsPayoutsGroups := [][]*UserWalletPayouts{{}}
	usersWalletsSoloPayoutsGroups := [][]*UserWalletSoloPayouts{{}}
	defer func() {
		usersWalletsPayoutsGroups, usersWalletsSoloPayoutsGroups = nil, nil
	}()
	groupNum := 0

	for userInfo, userPayoutsMap := range payoutsMap {
		if len(usersWalletsPayoutsGroups[groupNum]) >= p.config.ParallelNotificationsCount {
			groupNum++
			usersWalletsPayoutsGroups = append(usersWalletsPayoutsGroups, []*UserWalletPayouts{})
		}

		for walletInfo, userWalletPayouts := range userPayoutsMap {
			usersWalletsPayoutsGroups[groupNum] = append(usersWalletsPayoutsGroups[groupNum], &UserWalletPayouts{
				UserPayouts: UserPayouts{
					userInfo:   &userInfo,
					walletInfo: &walletInfo,
				},
				payouts: userWalletPayouts,
			})
		}
	}

	soloGroupNum := 0
	for userInfo, userSoloPayoutsMap := range soloPayoutsMap {
		if len(usersWalletsSoloPayoutsGroups[soloGroupNum]) >= p.config.ParallelNotificationsCount {
			soloGroupNum++
			usersWalletsSoloPayoutsGroups = append(usersWalletsSoloPayoutsGroups, []*UserWalletSoloPayouts{})
		}

		for walletInfo, userWalletSoloPayouts := range userSoloPayoutsMap {
			usersWalletsSoloPayoutsGroups[soloGroupNum] = append(usersWalletsSoloPayoutsGroups[soloGroupNum], &UserWalletSoloPayouts{
				UserPayouts: UserPayouts{
					userInfo:   &userInfo,
					walletInfo: &walletInfo,
				},
				payouts: userWalletSoloPayouts,
			})
		}
	}

	wg := sync.WaitGroup{}
	for _, usersWalletsPayouts := range usersWalletsPayoutsGroups {
		wg.Add(1)
		go p.notifyUsersPayments(ctx, usersWalletsPayouts, &wg)
	}

	for _, usersWalletsSoloPayouts := range usersWalletsSoloPayoutsGroups {
		wg.Add(1)
		go p.notifyUsersSoloPayments(ctx, usersWalletsSoloPayouts, &wg)
	}

	wg.Wait()

	if err := p.addNotification(ctx); err != nil {
		zap.L().Error("failed to add payments notification to db", zap.Error(err))
	}

	zap.L().Debug("payouts check completed",
		zap.Int("payouts_notified", totalPayouts),
		zap.Int("solo_payouts_notified", totalSoloPayouts),
	)
}
