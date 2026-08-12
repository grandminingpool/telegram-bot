package bot_notify

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pool_miners_proto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	format_utils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/hashicorp/go-set/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const removedWorkersTempTableName = "wallet_workers_to_be_removed"

type WorkerInfo struct {
	worker      string
	solo        bool
	connectedAt time.Time
}

func (w WorkerInfo) Hash() string {
	return w.worker
}

type PoolWorkersRequests struct {
	client  pool_miners_proto.PoolMinersServiceClient
	wallets [][]string
}

type RemovalWorkerDB struct {
	WalletID int64  `json:"wallet_id"`
	Worker   string `json:"worker"`
}

type WorkerDB struct {
	RemovalWorkerDB
	Solo        bool      `json:"solo"`
	ConnectedAt time.Time `json:"connected_at"`
}

type ChangedWorkersDB struct {
	added   []WorkerDB
	removed []RemovalWorkerDB
}

type UserInfo struct {
	userID int64
	chatID int64
	lang   string
}

type WalletInfo struct {
	id         int64
	wallet     string
	blockchain *blockchains.BlockchainInfo
}

type UserWalletWorkers struct {
	userInfo UserInfo
	id       int64
	workers  *set.HashSet[WorkerInfo, string]
}

type UserChangedWorkers struct {
	added   []WorkerInfo
	removed []WorkerInfo
}

type PoolWorkers struct {
	groupNum int
	coin     string
	workers  map[string]*pool_miners_proto.MinerWorkers
	err      error
}

type ChangedUserWorker struct {
	wallet WalletInfo
	worker WorkerInfo
}

type ChangedUserWorkers struct {
	userInfo UserInfo
	added    []ChangedUserWorker
	removed  []ChangedUserWorker
}

type Workers struct {
	pgConn             *pgxpool.Pool
	blockchainsService *blockchains.Service
	b                  *bot.Bot
	languages          *languages.Languages
	config             *bot_config.NotifyConfig
}

// getWorkersMap returns wallets grouped as [coin][wallet][]UserWalletWorkers
// so a single address can be tracked by multiple users — each user gets
// their own diff state (workers set) and their own wallet_workers.wallet_id.
func (w *Workers) getWorkersMap(ctx context.Context) (map[string]map[string][]UserWalletWorkers, error) {
	workersMap := make(map[string]map[string][]UserWalletWorkers)
	rows, err := w.pgConn.Query(ctx, `SELECT
		user_wallets.user_id,
		users.chat_id,
		users.lang,
		user_wallets.blockchain_coin,
		user_wallets.id,
		user_wallets.wallet,
		wallet_workers.worker,
		wallet_workers.solo,
		wallet_workers.connected_at
	FROM user_wallets
	LEFT JOIN users ON users.id = user_wallets.user_id
	LEFT JOIN wallet_workers ON wallet_workers.wallet_id = user_wallets.id`)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			userID, chatID, walletID int64
			userLang, coin, wallet   string
			worker                   *string
			solo                     *bool
			connectedAt              *time.Time
		)

		if err := rows.Scan(
			&userID,
			&chatID,
			&userLang,
			&coin,
			&walletID,
			&wallet,
			&worker,
			&solo,
			&connectedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workers columns: %w", err)
		}

		if _, ok := workersMap[coin]; !ok {
			workersMap[coin] = make(map[string][]UserWalletWorkers)
		}

		states := workersMap[coin][wallet]
		stateIdx := -1
		for i := range states {
			if states[i].userInfo.userID == userID {
				stateIdx = i
				break
			}
		}
		if stateIdx == -1 {
			states = append(states, UserWalletWorkers{
				userInfo: UserInfo{
					userID: userID,
					chatID: chatID,
					lang:   userLang,
				},
				id:      walletID,
				workers: set.NewHashSet[WorkerInfo, string](0),
			})
			workersMap[coin][wallet] = states
			stateIdx = len(states) - 1
		}

		if worker != nil {
			// .workers is *HashSet — pointer is shared across copies,
			// so Insert through the slice element mutates the same set.
			states[stateIdx].workers.Insert(WorkerInfo{
				worker:      *worker,
				solo:        *solo,
				connectedAt: *connectedAt,
			})
		}
	}

	return workersMap, nil
}

func (w *Workers) getPoolRequestsMap(workersMap map[string]map[string][]UserWalletWorkers) (map[string]*PoolWorkersRequests, int, error) {
	poolRequestsMap := make(map[string]*PoolWorkersRequests)
	requestsCount := 0
	for coin, coinWorkersMap := range workersMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, 0, err
		}

		client := pool_miners_proto.NewPoolMinersServiceClient(conn)
		poolRequests := &PoolWorkersRequests{
			client:  client,
			wallets: [][]string{{}},
		}

		groupNum := 0
		requestsCount++
		for wallet := range coinWorkersMap {
			if len(poolRequests.wallets[groupNum]) >= w.config.MaxWalletsInWorkersRequest {
				groupNum++
				requestsCount++
				poolRequests.wallets = append(poolRequests.wallets, []string{})
			}

			poolRequests.wallets[groupNum] = append(poolRequests.wallets[groupNum], wallet)
		}

		poolRequestsMap[coin] = poolRequests
	}

	return poolRequestsMap, requestsCount, nil
}

func (w *Workers) getWorkers(
	ctx context.Context,
	client pool_miners_proto.PoolMinersServiceClient,
	coin string,
	groupNum int,
	wallets []string,
	resultCh chan<- PoolWorkers,
) {
	result := PoolWorkers{
		groupNum: groupNum,
		coin:     coin,
	}
	workers, err := client.GetMinersWorkersFromList(ctx, &pool_miners_proto.MinerAddressesRequest{
		Addresses: wallets,
	})
	if err != nil {
		result.err = fmt.Errorf("failed to get pool (coin: %s) workers for group: %d, error: %w", coin, groupNum, err)
	} else {
		result.workers = workers.Workers
	}

	resultCh <- result
}


func (w *Workers) notifyUsers(
	ctx context.Context,
	changedUsersWorkers []ChangedUserWorkers,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	select {
	case <-ctx.Done():
		return
	default:
		var msgBuf bytes.Buffer
		for _, changedUserWorkers := range changedUsersWorkers {
			userLocalizer := w.languages.GetLocalizer(changedUserWorkers.userInfo.lang)

			for _, addedWorker := range changedUserWorkers.added {
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WorkerActive",
					TemplateData: map[string]string{
						"Worker":             addedWorker.worker.worker,
						"PoolBlockchainName": addedWorker.wallet.blockchain.Name,
					},
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WorkerInfoShort",
					TemplateData: map[string]string{
						"Solo":        format_utils.BoolText(addedWorker.worker.solo, userLocalizer),
						"ConnectedAt": addedWorker.worker.connectedAt.Format(time.Kitchen),
					},
				}))

				w.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    changedUserWorkers.userInfo.chatID,
					ParseMode: models.ParseModeHTML,
					Text:      msgBuf.String(),
				})

				msgBuf.Reset()
			}

			for _, removedWorker := range changedUserWorkers.removed {
				w.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    changedUserWorkers.userInfo.chatID,
					ParseMode: models.ParseModeHTML,
					Text: userLocalizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "WorkerInactive",
						TemplateData: map[string]string{
							"Worker":             removedWorker.worker.worker,
							"PoolBlockchainName": removedWorker.wallet.blockchain.Name,
						},
					}),
				})
			}
		}
	}
}

func (w *Workers) Check(ctx context.Context) {
	zap.L().Debug("workers check started")

	workersMap, err := w.getWorkersMap(ctx)
	defer clear(workersMap)
	if err != nil {
		zap.L().Error("failed to create workers map", zap.Error(err))

		return
	}

	zap.L().Debug("workers check: loaded workers map", zap.Int("coins_count", len(workersMap)))

	poolRequestsMap, requestsCount, err := w.getPoolRequestsMap(workersMap)
	defer clear(poolRequestsMap)
	if err != nil {
		zap.L().Error("failed to create pool requests map", zap.Error(err))

		return
	}

	zap.L().Debug("workers check: pool requests prepared", zap.Int("requests_count", requestsCount))

	poolWorkersCh := make(chan PoolWorkers, requestsCount)
	newCtx, cancel := w.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	for coin, poolRequests := range poolRequestsMap {
		client := poolRequests.client

		for groupNum := 0; groupNum < len(poolRequests.wallets); groupNum++ {
			go w.getWorkers(newCtx, client, coin, groupNum, poolRequests.wallets[groupNum], poolWorkersCh)
		}
	}

	changedWorkersMap := make(map[UserInfo]map[WalletInfo]UserChangedWorkers)
	defer clear(changedWorkersMap)
	for i := 0; i < requestsCount; i++ {
		select {
		case <-ctx.Done():
			return
		case poolWorkers := <-poolWorkersCh:
			if poolWorkers.err != nil {
				zap.L().Error("get pool workers error",
					zap.String("coin", poolWorkers.coin),
					zap.Int("group_num", poolWorkers.groupNum),
					zap.Error(poolWorkers.err),
				)

				continue
			}

			blockchain, err := w.blockchainsService.GetInfo(poolWorkers.coin)
			if err != nil {
				zap.L().Error("get blockchain info for workers error",
					zap.String("coin", poolWorkers.coin),
					zap.Int("group_num", poolWorkers.groupNum),
					zap.Error(err),
				)

				continue
			}

			coinWorkersMap, ok := workersMap[poolWorkers.coin]
			coinRequests, hasRequests := poolRequestsMap[poolWorkers.coin]
			if ok && hasRequests && poolWorkers.groupNum < len(coinRequests.wallets) {
				// Iterate the wallets WE requested in this group, not the
				// keys returned by the pool. If a wallet has zero active
				// workers, the pool may omit it from the response entirely
				// — we still need to compute the diff (and emit "removed"
				// notifications) for those.
				for _, wallet := range coinRequests.wallets[poolWorkers.groupNum] {
					userWalletWorkersSlice, ok := coinWorkersMap[wallet]
					if !ok {
						continue
					}

					// Build the pool's snapshot of this wallet's workers
					// (empty set if the pool didn't include it). Reuse
					// across all users that track this wallet.
					var walletWorkersSet *set.HashSet[WorkerInfo, string]
					if poolWalletWorkers, ok := poolWorkers.workers[wallet]; ok {
						walletWorkersSet = set.NewHashSet[WorkerInfo, string](len(poolWalletWorkers.Workers))
						for _, mw := range poolWalletWorkers.Workers {
							walletWorkersSet.Insert(WorkerInfo{
								worker:      mw.Worker,
								solo:        mw.Solo,
								connectedAt: mw.ConnectedAt.AsTime(),
							})
						}
					} else {
						walletWorkersSet = set.NewHashSet[WorkerInfo, string](0)
					}

					for _, userWalletWorkers := range userWalletWorkersSlice {
						walletInfo := WalletInfo{
							id:         userWalletWorkers.id,
							wallet:     wallet,
							blockchain: &blockchain,
						}
						userChangedWorkers := UserChangedWorkers{
							added:   walletWorkersSet.Difference(userWalletWorkers.workers).Slice(),
							removed: userWalletWorkers.workers.Difference(walletWorkersSet).Slice(),
						}

						changedUserWorkersMap, ok := changedWorkersMap[userWalletWorkers.userInfo]
						if !ok {
							changedUserWorkersMap = make(map[WalletInfo]UserChangedWorkers)
							changedWorkersMap[userWalletWorkers.userInfo] = changedUserWorkersMap
						}
						changedUserWorkersMap[walletInfo] = userChangedWorkers
					}
				}
			}
		}
	}

	totalAdded, totalRemoved := 0, 0
	for _, changedUserWorkersMap := range changedWorkersMap {
		for _, userChangedWorkers := range changedUserWorkersMap {
			totalAdded += len(userChangedWorkers.added)
			totalRemoved += len(userChangedWorkers.removed)
		}
	}

	zap.L().Debug("workers check: changes detected",
		zap.Int("added", totalAdded),
		zap.Int("removed", totalRemoved),
		zap.Int("users_affected", len(changedWorkersMap)),
	)

	tx, err := w.pgConn.Begin(ctx)
	if err != nil {
		zap.L().Error("failed to create transaction to update workers in database", zap.Error(err))

		return
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TEMP TABLE %s (
			wallet_id BIGINT NOT NULL,
			worker TEXT NOT NULL,
			PRIMARY KEY(wallet_id, worker)
		)`, removedWorkersTempTableName)); err != nil {
		tx.Rollback(ctx)

		zap.L().Error("failed to create temp table for removed workers",
			zap.String("temp_table_name", removedWorkersTempTableName),
			zap.Error(err),
		)

		return
	}

	changedWorkersGroups := []ChangedWorkersDB{{
		added:   []WorkerDB{},
		removed: []RemovalWorkerDB{},
	}}
	defer func() {
		changedWorkersGroups = nil
	}()
	groupNum := 0

	for _, changedUserWorkersMap := range changedWorkersMap {
		if len(changedWorkersGroups[groupNum].added)+len(changedWorkersGroups[groupNum].removed) >= w.config.MaxUsersDBChangesLimit {
			groupNum++
			changedWorkersGroups = append(changedWorkersGroups, ChangedWorkersDB{
				added:   []WorkerDB{},
				removed: []RemovalWorkerDB{},
			})
		}

		for walletInfo, userChangedWorkers := range changedUserWorkersMap {
			for _, workerInfo := range userChangedWorkers.added {
				changedWorkersGroups[groupNum].added = append(changedWorkersGroups[groupNum].added, WorkerDB{
					RemovalWorkerDB: RemovalWorkerDB{
						WalletID: walletInfo.id,
						Worker:   workerInfo.worker,
					},
					Solo:        workerInfo.solo,
					ConnectedAt: workerInfo.connectedAt,
				})
			}

			for _, workerInfo := range userChangedWorkers.removed {
				changedWorkersGroups[groupNum].removed = append(changedWorkersGroups[groupNum].removed, RemovalWorkerDB{
					WalletID: walletInfo.id,
					Worker:   workerInfo.worker,
				})
			}
		}
	}

	batch := &pgx.Batch{}
	for _, changedWorkers := range changedWorkersGroups {
		for _, worker := range changedWorkers.added {
			batch.Queue(`INSERT INTO wallet_workers (
				wallet_id,
				worker,
				solo,
				connected_at
			) VALUES ($1, $2, $3, $4)`,
				worker.WalletID, worker.Worker, worker.Solo, worker.ConnectedAt,
			)
		}

		for _, worker := range changedWorkers.removed {
			batch.Queue(fmt.Sprintf(`INSERT INTO %s (
				wallet_id,
				worker
			) VALUES ($1, $2)`, removedWorkersTempTableName),
				worker.WalletID, worker.Worker,
			)
		}
	}

	if batch.Len() > 0 {
		br := tx.SendBatch(newCtx, batch)
		if err := br.Close(); err != nil {
			tx.Rollback(ctx)

			zap.L().Error("failed to change workers rows in database", zap.Error(err))

			return
		}
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM wallet_workers 
			USING %s
			WHERE wallet_workers.wallet_id = %s.wallet_id AND wallet_workers.worker = %s.worker`,
		removedWorkersTempTableName,
		removedWorkersTempTableName,
		removedWorkersTempTableName,
	)); err != nil {
		tx.Rollback(ctx)

		zap.L().Error("failed to delete removed workers rows from database", zap.Error(err))

		return
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf("DROP TABLE %s", removedWorkersTempTableName)); err != nil {
		tx.Rollback(ctx)

		zap.L().Error("failed to drop temp table for removed workers",
			zap.String("temp_table_name", removedWorkersTempTableName),
			zap.Error(err),
		)

		return
	}

	if err := tx.Commit(ctx); err != nil {
		zap.L().Error("failed to commit workers changes in database", zap.Error(err))

		return
	}

	changedUsersWorkersGroups := [][]ChangedUserWorkers{{}}
	defer func() {
		changedUsersWorkersGroups = nil
	}()
	groupNum = 0

	for userInfo, changedUserWorkersMap := range changedWorkersMap {
		if len(changedUsersWorkersGroups[groupNum]) >= w.config.ParallelNotificationsCount {
			groupNum++
			changedUsersWorkersGroups = append(changedUsersWorkersGroups, []ChangedUserWorkers{})
		}

		for walletInfo, userChangedWorkers := range changedUserWorkersMap {
			perUser := ChangedUserWorkers{
				userInfo: userInfo,
				added:    make([]ChangedUserWorker, 0, len(userChangedWorkers.added)),
				removed:  make([]ChangedUserWorker, 0, len(userChangedWorkers.removed)),
			}

			for _, workerInfo := range userChangedWorkers.added {
				perUser.added = append(perUser.added, ChangedUserWorker{
					wallet: walletInfo,
					worker: workerInfo,
				})
			}

			for _, workerInfo := range userChangedWorkers.removed {
				perUser.removed = append(perUser.removed, ChangedUserWorker{
					wallet: walletInfo,
					worker: workerInfo,
				})
			}

			changedUsersWorkersGroups[groupNum] = append(changedUsersWorkersGroups[groupNum], perUser)
		}
	}

	wg := sync.WaitGroup{}
	for _, changedUsersWorkers := range changedUsersWorkersGroups {
		wg.Add(1)
		go w.notifyUsers(ctx, changedUsersWorkers, &wg)
	}

	wg.Wait()

	zap.L().Debug("workers check completed",
		zap.Int("notifications_sent_added", totalAdded),
		zap.Int("notifications_sent_removed", totalRemoved),
	)
}
