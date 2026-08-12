package bot_notify

import (
	"bytes"
	"context"
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

// ImmatureBlockInfo represents a solo-mined block that hasn't yet reached
// payout maturity. Pool API returns it without tx_hash / paid_at.
type ImmatureBlockInfo struct {
	reward    uint64
	blockHash string
	minedAt   time.Time
}

type UserPayouts struct {
	userInfo   UserInfo
	walletInfo WalletInfo
}

type UserWalletPayouts struct {
	UserPayouts
	payouts []PayoutInfo
}

type UserWalletSoloPayouts struct {
	UserPayouts
	payouts []SoloPayoutInfo
}

type UserWalletImmatureBlocks struct {
	UserPayouts
	blocks []ImmatureBlockInfo
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
	userInfo UserInfo
	id       int64
	payouts  bool
	blocks   bool
}

type PoolPayoutsRequests struct {
	client      pool_payouts_proto.PoolPayoutsServiceClient
	wallets     [][]string
	soloWallets [][]string
}

type LastNotifiedAt struct {
	payouts     time.Time
	soloPayouts time.Time
}

// sentNotification is the dedupe key for any per-(coin, user, hash)
// notification. `hash` is tx_hash for mature payouts and block_hash for
// immature solo blocks — the same logical shape, two different tables.
// Same tx_hash / block_hash may go to multiple users if they share a wallet,
// so user_id is part of the key.
type sentNotification struct {
	coin   string
	userID int64
	hash   string
}

type Payouts struct {
	pgConn             *pgxpool.Pool
	blockchainsService *blockchains.Service
	languages          *languages.Languages
	b                  *bot.Bot
	config             *bot_config.NotifyConfig
}

// getLastNotifiedAt returns the per-coin timestamps from which the next
// payouts / solo blocks queries should start. Missing rows are inserted with
// NOW() so a freshly added coin doesn't re-process historical payouts on its
// first tick.
func (p *Payouts) getLastNotifiedAt(ctx context.Context, coins []string) (map[string]LastNotifiedAt, error) {
	if _, err := p.pgConn.Exec(ctx, `
		INSERT INTO payouts_notifications (coin)
		SELECT unnest($1::text[])
		ON CONFLICT (coin) DO NOTHING`, coins); err != nil {
		return nil, fmt.Errorf("failed to upsert payouts_notifications rows: %w", err)
	}

	rows, err := p.pgConn.Query(ctx, `SELECT coin, last_payouts_at, last_solo_payouts_at
		FROM payouts_notifications
		WHERE coin = ANY($1)`, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to query payouts_notifications: %w", err)
	}
	defer rows.Close()

	result := make(map[string]LastNotifiedAt, len(coins))
	for rows.Next() {
		var coin string
		var ts LastNotifiedAt
		if err := rows.Scan(&coin, &ts.payouts, &ts.soloPayouts); err != nil {
			return nil, fmt.Errorf("failed to scan payouts_notifications row: %w", err)
		}
		result[coin] = ts
	}

	return result, nil
}

// filterAndMarkSent inserts the given notification keys into
// sent_payout_notifications (ON CONFLICT DO NOTHING) and returns the subset
// of keys that were actually inserted — i.e., keys that hadn't been sent
// before. Only those should be delivered to users now.
//
// This is "at most once" with respect to crashes: if the bot dies between
// marking and sending, the affected user misses a single notification. The
// alternative ("send then mark") guarantees delivery but can re-send many
// notifications when the bot is restarted mid-cycle.
//
// Keys with an empty hash are skipped — they can't be deduplicated.
// `table` is the dedupe table; `hashCol` is the name of its hash column
// (`tx_hash` for mature payouts, `block_hash` for immature blocks). Table
// and column names come from compile-time constants, so the dynamic SQL
// has no injection surface.
func (p *Payouts) filterAndMarkSent(
	ctx context.Context,
	table, hashCol string,
	keys []sentNotification,
) (map[sentNotification]struct{}, error) {
	if len(keys) == 0 {
		return map[sentNotification]struct{}{}, nil
	}

	coins := make([]string, 0, len(keys))
	userIDs := make([]int64, 0, len(keys))
	hashes := make([]string, 0, len(keys))
	for _, k := range keys {
		if k.hash == "" {
			continue
		}
		coins = append(coins, k.coin)
		userIDs = append(userIDs, k.userID)
		hashes = append(hashes, k.hash)
	}
	if len(coins) == 0 {
		return map[sentNotification]struct{}{}, nil
	}

	sql := fmt.Sprintf(`INSERT INTO %s (coin, user_id, %s)
		SELECT * FROM unnest($1::text[], $2::bigint[], $3::text[])
		ON CONFLICT (coin, user_id, %s) DO NOTHING
		RETURNING coin, user_id, %s`, table, hashCol, hashCol, hashCol)

	rows, err := p.pgConn.Query(ctx, sql, coins, userIDs, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to mark sent notifications in %s: %w", table, err)
	}
	defer rows.Close()

	newKeys := make(map[sentNotification]struct{})
	for rows.Next() {
		var k sentNotification
		if err := rows.Scan(&k.coin, &k.userID, &k.hash); err != nil {
			return nil, fmt.Errorf("failed to scan %s row: %w", table, err)
		}
		newKeys[k] = struct{}{}
	}

	return newKeys, nil
}

// pruneSentNotifications drops dedupe records older than the given retention
// window from the specified table.
func (p *Payouts) pruneSentNotifications(ctx context.Context, table string, retentionDays int) error {
	sql := fmt.Sprintf(`DELETE FROM %s WHERE sent_at < NOW() - make_interval(days => $1)`, table)
	if _, err := p.pgConn.Exec(ctx, sql, retentionDays); err != nil {
		return fmt.Errorf("failed to prune %s: %w", table, err)
	}

	return nil
}

// updateLastNotifiedAt moves last_payouts_at / last_solo_payouts_at to NOW()
// for the coins whose requests succeeded in the current tick. A coin listed
// only in one slice keeps the other column unchanged.
func (p *Payouts) updateLastNotifiedAt(ctx context.Context, payoutCoins, soloPayoutCoins []string) error {
	batch := &pgx.Batch{}
	if len(payoutCoins) > 0 {
		batch.Queue(`UPDATE payouts_notifications SET last_payouts_at = NOW() WHERE coin = ANY($1)`, payoutCoins)
	}
	if len(soloPayoutCoins) > 0 {
		batch.Queue(`UPDATE payouts_notifications SET last_solo_payouts_at = NOW() WHERE coin = ANY($1)`, soloPayoutCoins)
	}
	if batch.Len() == 0 {
		return nil
	}

	br := p.pgConn.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to update payouts_notifications timestamps: %w", err)
	}

	return nil
}

// getWalletsMap returns wallets grouped as [coin][wallet][]UserWallet so a
// single address can be tracked by multiple users — each user gets notified
// independently.
func (p *Payouts) getWalletsMap(ctx context.Context) (map[string]map[string][]UserWallet, error) {
	walletsMap := make(map[string]map[string][]UserWallet)
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

		if _, ok := walletsMap[coin]; !ok {
			walletsMap[coin] = make(map[string][]UserWallet)
		}

		walletsMap[coin][wallet] = append(walletsMap[coin][wallet], UserWallet{
			userInfo: UserInfo{
				userID: userID,
				chatID: chatID,
				lang:   userLang,
			},
			id:      walletID,
			payouts: payoutsNotify,
			blocks:  blocksNotify,
		})
	}

	return walletsMap, nil
}

func (w *Payouts) getPoolRequestsMap(walletsMap map[string]map[string][]UserWallet) (map[string]*PoolPayoutsRequests, int, int, error) {
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
		for wallet, userWallets := range coinWalletsMap {
			needsPayouts := false
			needsBlocks := false
			for _, uw := range userWallets {
				if uw.payouts {
					needsPayouts = true
				}
				if uw.blocks {
					needsBlocks = true
				}
				if needsPayouts && needsBlocks {
					break
				}
			}

			if needsPayouts {
				if len(poolRequests.wallets[groupNum]) >= w.config.MaxWalletsInPayoutsRequest {
					groupNum++
					requestsCount++
					poolRequests.wallets = append(poolRequests.wallets, []string{})
				}

				poolRequests.wallets[groupNum] = append(poolRequests.wallets[groupNum], wallet)
			}

			if needsBlocks {
				if len(poolRequests.soloWallets[soloGroupNum]) >= w.config.MaxWalletsInPayoutsRequest {
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
	result := PoolSoloPayouts{
		groupNum: groupNum,
		coin:     coin,
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

func (p *Payouts) getPayouts(
	ctx context.Context,
	client pool_payouts_proto.PoolPayoutsServiceClient,
	coin string,
	groupNum int,
	wallets []string,
	paidFrom time.Time,
	resultCh chan<- PoolPayouts,
) {
	result := PoolPayouts{
		groupNum: groupNum,
		coin:     coin,
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

func (p *Payouts) notifyUsersPayments(
	ctx context.Context,
	usersWalletsPayouts []UserWalletPayouts,
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
						"PaidAt": userPayoutInfo.paidAt.Format("2006-01-02 15:04:05 MST"),
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
	usersWalletsSoloPayouts []UserWalletSoloPayouts,
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
						"PaidAt":    userSoloPayoutInfo.paidAt.Format("2006-01-02 15:04:05 MST"),
						"MinedAt":   userSoloPayoutInfo.minedAt.Format("2006-01-02 15:04:05 MST"),
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

func (p *Payouts) notifyUsersImmatureBlocks(
	ctx context.Context,
	usersWalletsImmatures []UserWalletImmatureBlocks,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	select {
	case <-ctx.Done():
		return
	default:
		var msgBuf bytes.Buffer
		for _, userWalletImmatures := range usersWalletsImmatures {
			userLocalizer := p.languages.GetLocalizer(userWalletImmatures.userInfo.lang)

			for _, info := range userWalletImmatures.blocks {
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "NewImmatureBlockFound",
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WalletInfo",
					TemplateData: map[string]string{
						"Wallet":             userWalletImmatures.walletInfo.wallet,
						"PoolBlockchainName": userWalletImmatures.walletInfo.blockchain.Name,
					},
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ImmatureBlockDetails",
					TemplateData: map[string]string{
						"Reward":    format_utils.WalletBalance(info.reward, userWalletImmatures.walletInfo.blockchain.AtomicUnit),
						"Ticker":    userWalletImmatures.walletInfo.blockchain.Ticker,
						"BlockHash": info.blockHash,
						"MinedAt":   info.minedAt.Format("2006-01-02 15:04:05 MST"),
					},
				}))

				p.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    userWalletImmatures.userInfo.chatID,
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

	walletsMap, err := p.getWalletsMap(ctx)
	defer clear(walletsMap)
	if err != nil {
		zap.L().Error("failed to get wallets map", zap.Error(err))

		return
	}

	if len(walletsMap) == 0 {
		zap.L().Debug("payouts check: no wallets to check")

		return
	}

	coins := make([]string, 0, len(walletsMap))
	for coin := range walletsMap {
		coins = append(coins, coin)
	}

	lastNotifiedAt, err := p.getLastNotifiedAt(ctx, coins)
	if err != nil {
		zap.L().Error("failed to get last notified timestamps", zap.Error(err))

		return
	}

	zap.L().Debug("payouts check: loaded wallets map",
		zap.Int("coins_count", len(walletsMap)),
	)

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
	newCtx, cancel := p.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	for coin, poolRequests := range poolRequestsMap {
		ts := lastNotifiedAt[coin]
		client := poolRequests.client

		for groupNum := 0; groupNum < len(poolRequests.wallets); groupNum++ {
			go p.getPayouts(
				newCtx,
				client,
				coin,
				groupNum,
				poolRequests.wallets[groupNum],
				ts.payouts,
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
				ts.soloPayouts,
				poolSoloPayoutsCh,
			)
		}
	}

	failedPayouts := make(map[string]bool)
	failedSoloPayouts := make(map[string]bool)

	payoutsMap := make(map[UserInfo]map[WalletInfo][]PayoutInfo)
	soloPayoutsMap := make(map[UserInfo]map[WalletInfo][]SoloPayoutInfo)
	immatureBlocksMap := make(map[UserInfo]map[WalletInfo][]ImmatureBlockInfo)
	defer clear(payoutsMap)
	defer clear(soloPayoutsMap)
	defer clear(immatureBlocksMap)
	for i := 0; i < (requestsCount + soloRequestsCount); i++ {
		select {
		case <-ctx.Done():
			return
		case poolPayouts := <-poolPayoutsCh:
			if poolPayouts.err != nil {
				failedPayouts[poolPayouts.coin] = true
				zap.L().Error("get pool payouts error",
					zap.String("coin", poolPayouts.coin),
					zap.Int("group_num", poolPayouts.groupNum),
					zap.Error(poolPayouts.err),
				)

				continue
			}

			blockchain, err := p.blockchainsService.GetInfo(poolPayouts.coin)
			if err != nil {
				failedPayouts[poolPayouts.coin] = true
				zap.L().Error("get blockchain info for payouts error",
					zap.String("coin", poolPayouts.coin),
					zap.Int("group_num", poolPayouts.groupNum),
					zap.Error(err),
				)

				continue
			}

			coinWalletsMap, ok := walletsMap[poolPayouts.coin]
			if ok {
				for wallet, walletPayouts := range poolPayouts.payouts {
					userWallets, ok := coinWalletsMap[wallet]
					if !ok {
						continue
					}

					// Build once, share across all users with this wallet.
					payoutInfos := make([]PayoutInfo, 0, len(walletPayouts.Payouts))
					for _, walletPayout := range walletPayouts.Payouts {
						payoutInfos = append(payoutInfos, PayoutInfo{
							amount: walletPayout.Amount,
							txHash: walletPayout.TxHash,
							paidAt: walletPayout.PaidAt.AsTime(),
						})
					}

					for _, userWallet := range userWallets {
						if !userWallet.payouts {
							continue
						}

						walletInfo := WalletInfo{
							id:         userWallet.id,
							wallet:     wallet,
							blockchain: &blockchain,
						}

						userPayoutsMap, ok := payoutsMap[userWallet.userInfo]
						if !ok {
							userPayoutsMap = make(map[WalletInfo][]PayoutInfo)
							payoutsMap[userWallet.userInfo] = userPayoutsMap
						}
						userPayoutsMap[walletInfo] = payoutInfos
					}
				}
			}
		case poolSoloPayouts := <-poolSoloPayoutsCh:
			if poolSoloPayouts.err != nil {
				failedSoloPayouts[poolSoloPayouts.coin] = true
				zap.L().Error("get pool solo payouts error",
					zap.String("coin", poolSoloPayouts.coin),
					zap.Int("group_num", poolSoloPayouts.groupNum),
					zap.Error(poolSoloPayouts.err),
				)

				continue
			}

			blockchain, err := p.blockchainsService.GetInfo(poolSoloPayouts.coin)
			if err != nil {
				failedSoloPayouts[poolSoloPayouts.coin] = true
				zap.L().Error("get blockchain info for solo payouts error",
					zap.String("coin", poolSoloPayouts.coin),
					zap.Int("group_num", poolSoloPayouts.groupNum),
					zap.Error(err),
				)

				continue
			}

			coinWalletsMap, ok := walletsMap[poolSoloPayouts.coin]
			if ok {
				for wallet, walletSoloPayouts := range poolSoloPayouts.payouts {
					userWallets, ok := coinWalletsMap[wallet]
					if !ok {
						continue
					}

					// Split blocks into mature (have tx_hash + paid_at) and
					// immature (block found but not yet paid out). Build once,
					// share across all users with this wallet.
					var soloPayoutInfos []SoloPayoutInfo
					var immatureInfos []ImmatureBlockInfo
					for _, b := range walletSoloPayouts.Blocks {
						if b.TxHash == "" {
							immatureInfos = append(immatureInfos, ImmatureBlockInfo{
								reward:    b.Reward,
								blockHash: b.BlockHash,
								minedAt:   b.MinedAt.AsTime(),
							})
						} else {
							soloPayoutInfos = append(soloPayoutInfos, SoloPayoutInfo{
								reward:    b.Reward,
								blockHash: b.BlockHash,
								txHash:    b.TxHash,
								paidAt:    b.PaidAt.AsTime(),
								minedAt:   b.MinedAt.AsTime(),
							})
						}
					}

					for _, userWallet := range userWallets {
						if !userWallet.blocks {
							continue
						}

						walletInfo := WalletInfo{
							id:         userWallet.id,
							wallet:     wallet,
							blockchain: &blockchain,
						}

						if len(soloPayoutInfos) > 0 {
							userSoloPayoutsMap, ok := soloPayoutsMap[userWallet.userInfo]
							if !ok {
								userSoloPayoutsMap = make(map[WalletInfo][]SoloPayoutInfo)
								soloPayoutsMap[userWallet.userInfo] = userSoloPayoutsMap
							}
							userSoloPayoutsMap[walletInfo] = soloPayoutInfos
						}

						if len(immatureInfos) > 0 {
							userImmatureMap, ok := immatureBlocksMap[userWallet.userInfo]
							if !ok {
								userImmatureMap = make(map[WalletInfo][]ImmatureBlockInfo)
								immatureBlocksMap[userWallet.userInfo] = userImmatureMap
							}
							userImmatureMap[walletInfo] = immatureInfos
						}
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

	// Collect dedupe keys for mature (tx_hash) and immature (block_hash)
	// notifications into separate slices. filterAndMarkSent returns only
	// the keys that weren't already in the corresponding dedupe table —
	// those are the ones to actually deliver.
	matureKeys := make([]sentNotification, 0, totalPayouts+totalSoloPayouts)
	for userInfo, userPayoutsMap := range payoutsMap {
		for walletInfo, walletPayouts := range userPayoutsMap {
			for _, payout := range walletPayouts {
				matureKeys = append(matureKeys, sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   payout.txHash,
				})
			}
		}
	}
	for userInfo, userSoloPayoutsMap := range soloPayoutsMap {
		for walletInfo, walletSoloPayouts := range userSoloPayoutsMap {
			for _, soloPayout := range walletSoloPayouts {
				matureKeys = append(matureKeys, sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   soloPayout.txHash,
				})
			}
		}
	}

	immatureKeys := make([]sentNotification, 0)
	for userInfo, userImmatureMap := range immatureBlocksMap {
		for walletInfo, walletImmatures := range userImmatureMap {
			for _, b := range walletImmatures {
				immatureKeys = append(immatureKeys, sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   b.blockHash,
				})
			}
		}
	}

	newMature, err := p.filterAndMarkSent(ctx, "sent_payout_notifications", "tx_hash", matureKeys)
	if err != nil {
		zap.L().Error("failed to filter and mark sent payout notifications", zap.Error(err))

		return
	}

	newImmature, err := p.filterAndMarkSent(ctx, "sent_immature_block_notifications", "block_hash", immatureKeys)
	if err != nil {
		zap.L().Error("failed to filter and mark sent immature block notifications", zap.Error(err))

		return
	}

	totalImmature := 0
	for _, userImmatureMap := range immatureBlocksMap {
		for _, blocks := range userImmatureMap {
			totalImmature += len(blocks)
		}
	}

	zap.L().Debug("payouts check: changes detected",
		zap.Int("payouts", totalPayouts),
		zap.Int("solo_payouts", totalSoloPayouts),
		zap.Int("immature_blocks", totalImmature),
		zap.Int("new_mature_after_dedupe", len(newMature)),
		zap.Int("new_immature_after_dedupe", len(newImmature)),
		zap.Int("users_with_payouts", len(payoutsMap)),
		zap.Int("users_with_solo_payouts", len(soloPayoutsMap)),
		zap.Int("users_with_immature_blocks", len(immatureBlocksMap)),
		zap.Int("failed_payout_coins", len(failedPayouts)),
		zap.Int("failed_solo_payout_coins", len(failedSoloPayouts)),
	)

	usersWalletsPayoutsGroups := [][]UserWalletPayouts{{}}
	usersWalletsSoloPayoutsGroups := [][]UserWalletSoloPayouts{{}}
	usersWalletsImmatureBlocksGroups := [][]UserWalletImmatureBlocks{{}}
	defer func() {
		usersWalletsPayoutsGroups = nil
		usersWalletsSoloPayoutsGroups = nil
		usersWalletsImmatureBlocksGroups = nil
	}()
	groupNum := 0

	for userInfo, userPayoutsMap := range payoutsMap {
		if len(usersWalletsPayoutsGroups[groupNum]) >= p.config.ParallelNotificationsCount {
			groupNum++
			usersWalletsPayoutsGroups = append(usersWalletsPayoutsGroups, []UserWalletPayouts{})
		}

		for walletInfo, userWalletPayouts := range userPayoutsMap {
			filtered := make([]PayoutInfo, 0, len(userWalletPayouts))
			for _, payout := range userWalletPayouts {
				key := sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   payout.txHash,
				}
				if _, ok := newMature[key]; ok {
					filtered = append(filtered, payout)
				}
			}
			if len(filtered) == 0 {
				continue
			}

			usersWalletsPayoutsGroups[groupNum] = append(usersWalletsPayoutsGroups[groupNum], UserWalletPayouts{
				UserPayouts: UserPayouts{
					userInfo:   userInfo,
					walletInfo: walletInfo,
				},
				payouts: filtered,
			})
		}
	}

	soloGroupNum := 0
	for userInfo, userSoloPayoutsMap := range soloPayoutsMap {
		if len(usersWalletsSoloPayoutsGroups[soloGroupNum]) >= p.config.ParallelNotificationsCount {
			soloGroupNum++
			usersWalletsSoloPayoutsGroups = append(usersWalletsSoloPayoutsGroups, []UserWalletSoloPayouts{})
		}

		for walletInfo, userWalletSoloPayouts := range userSoloPayoutsMap {
			filtered := make([]SoloPayoutInfo, 0, len(userWalletSoloPayouts))
			for _, soloPayout := range userWalletSoloPayouts {
				key := sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   soloPayout.txHash,
				}
				if _, ok := newMature[key]; ok {
					filtered = append(filtered, soloPayout)
				}
			}
			if len(filtered) == 0 {
				continue
			}

			usersWalletsSoloPayoutsGroups[soloGroupNum] = append(usersWalletsSoloPayoutsGroups[soloGroupNum], UserWalletSoloPayouts{
				UserPayouts: UserPayouts{
					userInfo:   userInfo,
					walletInfo: walletInfo,
				},
				payouts: filtered,
			})
		}
	}

	immatureGroupNum := 0
	for userInfo, userImmatureMap := range immatureBlocksMap {
		if len(usersWalletsImmatureBlocksGroups[immatureGroupNum]) >= p.config.ParallelNotificationsCount {
			immatureGroupNum++
			usersWalletsImmatureBlocksGroups = append(usersWalletsImmatureBlocksGroups, []UserWalletImmatureBlocks{})
		}

		for walletInfo, userWalletImmatures := range userImmatureMap {
			filtered := make([]ImmatureBlockInfo, 0, len(userWalletImmatures))
			for _, b := range userWalletImmatures {
				key := sentNotification{
					coin:   walletInfo.blockchain.Coin,
					userID: userInfo.userID,
					hash:   b.blockHash,
				}
				if _, ok := newImmature[key]; ok {
					filtered = append(filtered, b)
				}
			}
			if len(filtered) == 0 {
				continue
			}

			usersWalletsImmatureBlocksGroups[immatureGroupNum] = append(usersWalletsImmatureBlocksGroups[immatureGroupNum], UserWalletImmatureBlocks{
				UserPayouts: UserPayouts{
					userInfo:   userInfo,
					walletInfo: walletInfo,
				},
				blocks: filtered,
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

	for _, usersWalletsImmatures := range usersWalletsImmatureBlocksGroups {
		wg.Add(1)
		go p.notifyUsersImmatureBlocks(ctx, usersWalletsImmatures, &wg)
	}

	wg.Wait()

	payoutCoins := make([]string, 0, len(coins))
	soloPayoutCoins := make([]string, 0, len(coins))
	for _, coin := range coins {
		if !failedPayouts[coin] {
			payoutCoins = append(payoutCoins, coin)
		}
		if !failedSoloPayouts[coin] {
			soloPayoutCoins = append(soloPayoutCoins, coin)
		}
	}

	if err := p.updateLastNotifiedAt(ctx, payoutCoins, soloPayoutCoins); err != nil {
		zap.L().Error("failed to update last notified timestamps",
			zap.Strings("payout_coins", payoutCoins),
			zap.Strings("solo_payout_coins", soloPayoutCoins),
			zap.Error(err),
		)
	}

	if err := p.pruneSentNotifications(ctx, "sent_payout_notifications", p.config.SentNotificationsRetentionDays); err != nil {
		zap.L().Error("failed to prune sent payout notifications", zap.Error(err))
	}
	if err := p.pruneSentNotifications(ctx, "sent_immature_block_notifications", p.config.ImmatureBlockNotificationsRetentionDays); err != nil {
		zap.L().Error("failed to prune sent immature block notifications", zap.Error(err))
	}

	zap.L().Debug("payouts check completed",
		zap.Int("payouts_collected", totalPayouts),
		zap.Int("solo_payouts_collected", totalSoloPayouts),
		zap.Int("immature_blocks_collected", totalImmature),
		zap.Int("mature_notifications_sent", len(newMature)),
		zap.Int("immature_notifications_sent", len(newImmature)),
		zap.Int("advanced_payout_coins", len(payoutCoins)),
		zap.Int("advanced_solo_payout_coins", len(soloPayoutCoins)),
	)
}
