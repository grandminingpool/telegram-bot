package botNotify

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	poolMinersProto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	formatUtils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/hashicorp/go-set/v2"
	"github.com/jmoiron/sqlx"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const REMOWED_WORKERS_TEMP_TABLE_NAME = "wallet_workers_to_be_removed"

type WorkerInfo struct {
	worker      string
	region      string
	solo        bool
	connectedAt time.Time
}

func (w WorkerInfo) Hash() string {
	return w.worker
}

type PoolWorkersRequests struct {
	client  poolMinersProto.PoolMinersServiceClient
	wallets [][]string
}

type RemovalWorkerDB struct {
	WalletID int64  `json:"wallet_id"`
	Worker   string `json:"worker"`
}

type WorkerDB struct {
	RemovalWorkerDB
	Region      string    `json:"region"`
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
	userInfo *UserInfo
	id       int64
	workers  *set.HashSet[*WorkerInfo, string]
}

type UserChangedWorkers struct {
	added   []*WorkerInfo
	removed []*WorkerInfo
}

type PoolWorkers struct {
	groupNum int
	coin     string
	workers  map[string]*poolMinersProto.MinerWorkers
	err      error
}

type ChangedUserWorker struct {
	wallet *WalletInfo
	worker *WorkerInfo
}

type ChangedUserWorkers struct {
	userInfo *UserInfo
	added    []ChangedUserWorker
	removed  []ChangedUserWorker
}

type Workers struct {
	pgConn             *sqlx.DB
	blockchainsService *blockchains.Service
	b                  *bot.Bot
	languages          *languages.Languages
	config             *botConfig.NotifyConfig
}

func (w *Workers) getWorkersMap(ctx context.Context) (map[string]map[string]*UserWalletWorkers, error) {
	workersMap := make(map[string]map[string]*UserWalletWorkers)
	rows, err := w.pgConn.QueryContext(ctx, `SELECT
		user_wallets.user_id,
		users.chat_id,
		users.lang,
		user_wallets.blockchain_coin,
		user_wallets.id,
		user_wallets.wallet,
		wallet_workers.worker,
		wallet_workers.region,
		wallet_workers.solo,
		wallet_workers.connected_at
	FROM wallet_workers
	LEFT JOIN users ON users.id = wallet_workers.user_id
	LEFT JOIN user_wallets ON user_wallets.id = wallet_workers.wallet_id`)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %w", err)
	}
	set.New[string](10)
	for rows.Next() {
		var (
			userID, chatID, walletID               int64
			userLang, coin, wallet, worker, region string
			solo                                   bool
			connectedAt                            time.Time
		)

		if err := rows.Scan(
			&userID,
			&chatID,
			&userLang,
			&coin,
			&walletID,
			&wallet,
			&worker,
			&region,
			&solo,
			&connectedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workers columns: %w", err)
		}

		_, ok := workersMap[coin]
		if !ok {
			workersMap[coin] = make(map[string]*UserWalletWorkers)
		}

		workerInfo := &WorkerInfo{
			worker,
			region,
			solo,
			connectedAt,
		}
		coinWorkersWalletsMap, ok := workersMap[coin][wallet]
		if !ok {
			set.NewHashSet[*WorkerInfo, string](0)
			workersMap[coin][wallet] = &UserWalletWorkers{
				userInfo: &UserInfo{
					userID: userID,
					chatID: chatID,
					lang:   userLang,
				},
				id:      walletID,
				workers: set.HashSetFrom[*WorkerInfo, string]([]*WorkerInfo{workerInfo}),
			}
		} else {
			coinWorkersWalletsMap.workers.Insert(workerInfo)
		}
	}

	return workersMap, nil
}

func (w *Workers) getPoolRequestsMap(workersMap map[string]map[string]*UserWalletWorkers) (map[string]*PoolWorkersRequests, int, error) {
	poolRequestsMap := make(map[string]*PoolWorkersRequests)
	requestsCount := 0
	for coin, coinWorkersMap := range workersMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, 0, err
		}

		client := poolMinersProto.NewPoolMinersServiceClient(conn)
		poolRequests := &PoolWorkersRequests{
			client:  client,
			wallets: [][]string{},
		}

		groupNum := 0
		requestsCount++
		i := 0
		for wallet := range coinWorkersMap {
			if i > w.config.MaxWalletsInWorkersRequest {
				groupNum++
				requestsCount++
				i = 0
			}

			poolRequests.wallets[groupNum][i] = wallet

			i++
		}

		poolRequestsMap[coin] = poolRequests
	}

	return poolRequestsMap, requestsCount, nil
}

func (w *Workers) getWorkers(
	ctx context.Context,
	client poolMinersProto.PoolMinersServiceClient,
	coin string,
	groupNum int,
	wallets []string,
	resultCh chan<- PoolWorkers,
) {
	select {
	case <-ctx.Done():
		return
	default:
		result := PoolWorkers{
			groupNum: groupNum,
			coin:     coin,
			workers:  nil,
			err:      nil,
		}
		workers, err := client.GetWorkers(ctx, &poolMinersProto.MinerAddressesRequest{
			Addresses: wallets,
		})
		if err != nil {
			result.err = fmt.Errorf("failed to get pool (coin: %s) workers for group: %d, error: %w", coin, groupNum, err)
		} else {
			result.workers = workers.Workers
		}

		resultCh <- result
	}
}

func (w *Workers) addWorkers(ctx context.Context, tx *sqlx.Tx, groupNum int, addedWorkers []WorkerDB, errCh chan<- error) {
	select {
	case <-ctx.Done():
		return
	default:
		if _, err := tx.NamedExecContext(ctx, `INSERT INTO wallet_workers (
		   wallet_id, 
		   worker, 
		   region, 
		   solo, 
		   connected_at
	   ) VALUES (:wallet_id, :worker, :region, :solo, :connected_at)`, addedWorkers); err != nil {
			errCh <- fmt.Errorf("failed to insert added workers batch (group num: %d, batch length: %d), error: %w", groupNum, len(addedWorkers), err)
		}
	}
}

func (w *Workers) removeWorkers(ctx context.Context, tx *sqlx.Tx, groupNum int, removedWorkers []RemovalWorkerDB, errCh chan<- error) {
	select {
	case <-ctx.Done():
		return
	default:
		if _, err := tx.NamedExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (
			wallet_id, 
			worker
		) VALUES (:wallet_id, :worker)`, REMOWED_WORKERS_TEMP_TABLE_NAME), removedWorkers); err != nil {
			errCh <- fmt.Errorf("failed to insert removed workers batch to temp table (group num: %d, batch length: %d), error: %w", groupNum, len(removedWorkers), err)
		}
	}
}

func (w *Workers) notifyUsers(
	ctx context.Context,
	changedUsersWorkers []*ChangedUserWorkers,
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
						"Worker": addedWorker.worker.worker,
					},
				}))
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(userLocalizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WorkerInfoShort",
					TemplateData: map[string]string{
						"Region":      addedWorker.worker.region,
						"Solo":        formatUtils.BoolText(addedWorker.worker.solo, userLocalizer),
						"ConnectedAt": addedWorker.worker.connectedAt.Format(time.Kitchen),
					},
				}))

				w.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: changedUserWorkers.userInfo.chatID,
					Text:   msgBuf.String(),
				})

				msgBuf.Reset()
			}

			for _, removedWorker := range changedUserWorkers.removed {
				w.b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: changedUserWorkers.userInfo.chatID,
					Text: userLocalizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "WorkerInactive",
						TemplateData: map[string]string{
							"Worker": removedWorker.worker.worker,
						},
					}),
				})
			}
		}
	}
}

func (w *Workers) Check(ctx context.Context) {
	workersMap, err := w.getWorkersMap(ctx)
	defer clear(workersMap)
	if err != nil {
		zap.L().Error("failed to create workers map", zap.Error(err))

		return
	}

	poolRequestsMap, requestsCount, err := w.getPoolRequestsMap(workersMap)
	defer clear(poolRequestsMap)
	if err != nil {
		zap.L().Error("failed to create pool requests map", zap.Error(err))

		return
	}

	poolWorkersCh := make(chan PoolWorkers, requestsCount)
	defer close(poolWorkersCh)
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for coin, poolRequests := range poolRequestsMap {
		client := poolRequests.client

		for groupNum := 0; groupNum < len(poolRequests.wallets); groupNum++ {
			go w.getWorkers(newCtx, client, coin, groupNum, poolRequests.wallets[groupNum], poolWorkersCh)
		}
	}

	changedWorkersMap := make(map[UserInfo]map[WalletInfo]*UserChangedWorkers)
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

				return
			}

			blockchain, err := w.blockchainsService.GetInfo(poolWorkers.coin)
			if err != nil {
				zap.L().Error("get blockchain info for workers error",
					zap.String("coin", poolWorkers.coin),
					zap.Int("group_num", poolWorkers.groupNum),
					zap.Error(err),
				)

				return
			}

			coinWorkersMap, ok := workersMap[poolWorkers.coin]
			if ok {
				for wallet, walletWorkers := range poolWorkers.workers {
					userWalletWorkers, ok := coinWorkersMap[wallet]
					if ok {
						walletWorkersSet := set.NewHashSet[*WorkerInfo, string](len(walletWorkers.Workers))
						for _, mw := range walletWorkers.Workers {
							walletWorkersSet.Insert(&WorkerInfo{
								worker:      mw.Worker,
								region:      mw.Region,
								solo:        mw.Solo,
								connectedAt: mw.ConnectedAt.AsTime(),
							})
						}

						walletInfo := WalletInfo{
							id:         userWalletWorkers.id,
							wallet:     wallet,
							blockchain: blockchain,
						}
						userChangedWorkers := &UserChangedWorkers{
							added:   walletWorkersSet.Difference(userWalletWorkers.workers).Slice(),
							removed: userWalletWorkers.workers.Difference(walletWorkersSet).Slice(),
						}

						changedUserWorkersMap, ok := changedWorkersMap[*userWalletWorkers.userInfo]
						if ok {
							changedUserWorkersMap[walletInfo] = userChangedWorkers
						} else {
							changedWorkersMap[*userWalletWorkers.userInfo][walletInfo] = userChangedWorkers
						}
					}
				}
			}
		default:
		}
	}

	tx, err := w.pgConn.BeginTxx(ctx, nil)
	if err != nil {
		zap.L().Error("failed to create transaction to update workers in database", zap.Error(err))

		return
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TEMP TABLE %s (
			wallet_id BIGINT NOT NULL,
			worker TEXT NOT NULL,
			PRIMARY KEY(wallet_id, worker)
		)`, REMOWED_WORKERS_TEMP_TABLE_NAME)); err != nil {
		tx.Rollback()

		zap.L().Error("failed to create temp table for removed workers",
			zap.String("temp_table_name", REMOWED_WORKERS_TEMP_TABLE_NAME),
			zap.Error(err),
		)

		return
	}

	changedWorkersGroups := []*ChangedWorkersDB{}
	defer func() {
		changedWorkersGroups = nil
	}()
	groupNum := 0
	i := 0
	j := 0

	for _, changedUserWorkersMap := range changedWorkersMap {
		if groupNum > w.config.MaxUsersDBChangesLimit {
			groupNum++
			i = 0
			j = 0
		}

		for walletInfo, userChangedWorkers := range changedUserWorkersMap {
			for _, workerInfo := range userChangedWorkers.added {
				changedWorkersGroups[groupNum].added[i] = WorkerDB{
					RemovalWorkerDB: RemovalWorkerDB{
						WalletID: walletInfo.id,
						Worker:   workerInfo.worker,
					},
					Region:      workerInfo.region,
					Solo:        workerInfo.solo,
					ConnectedAt: workerInfo.connectedAt,
				}

				i++
			}

			for _, workerInfo := range userChangedWorkers.removed {
				changedWorkersGroups[groupNum].removed[j] = RemovalWorkerDB{
					WalletID: walletInfo.id,
					Worker:   workerInfo.worker,
				}

				j++
			}
		}
	}

	changedWorkersGroupsLen := len(changedWorkersGroups)
	changeWorkersErrCh := make(chan error, 2*changedWorkersGroupsLen)
	defer close(changeWorkersErrCh)
	for groupNum, changedWorkers := range changedWorkersGroups {
		go w.addWorkers(newCtx, tx, groupNum, changedWorkers.added, changeWorkersErrCh)
		go w.removeWorkers(newCtx, tx, groupNum, changedWorkers.removed, changeWorkersErrCh)
	}

	for i := 0; i < changedWorkersGroupsLen; i++ {
		select {
		case <-ctx.Done():
			tx.Rollback()

			return
		case err := <-changeWorkersErrCh:
			tx.Rollback()

			zap.L().Error("failed to change workers rows in database", zap.Error(err))

			return
		default:
		}
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM wallet_workers 
			USING %s
			WHERE wallet_workers.wallet_id = %s.wallet_id AND wallet_workers.worker = %s.worker`,
		REMOWED_WORKERS_TEMP_TABLE_NAME,
		REMOWED_WORKERS_TEMP_TABLE_NAME,
		REMOWED_WORKERS_TEMP_TABLE_NAME,
	)); err != nil {
		tx.Rollback()

		zap.L().Error("failed to delete removed workers rows from database", zap.Error(err))

		return
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", REMOWED_WORKERS_TEMP_TABLE_NAME)); err != nil {
		tx.Rollback()

		zap.L().Error("failed to drop temp table for removed workers",
			zap.String("temp_table_name", REMOWED_WORKERS_TEMP_TABLE_NAME),
			zap.Error(err),
		)

		return
	}

	if err := tx.Commit(); err != nil {
		zap.L().Error("failed to commit workers changes in database", zap.Error(err))

		return
	}

	changedUsersWorkersGroups := [][]*ChangedUserWorkers{}
	defer func() {
		changedUsersWorkersGroups = nil
	}()
	groupNum = 0
	i = 0

	for userInfo, changedUserWorkersMap := range changedWorkersMap {
		if groupNum > w.config.ParallelNotificationsCount {
			groupNum++
			i = 0
		}

		for walletInfo, userChangedWorkers := range changedUserWorkersMap {
			changedUserWorkers := &ChangedUserWorkers{
				userInfo: &userInfo,
				added:    make([]ChangedUserWorker, 0, len(userChangedWorkers.added)),
				removed:  make([]ChangedUserWorker, 0, len(userChangedWorkers.removed)),
			}

			for _, workerInfo := range userChangedWorkers.added {
				changedUserWorkers.added = append(changedUserWorkers.added, ChangedUserWorker{
					wallet: &walletInfo,
					worker: workerInfo,
				})
			}

			for _, workerInfo := range userChangedWorkers.removed {
				changedUserWorkers.removed = append(changedUserWorkers.removed, ChangedUserWorker{
					wallet: &walletInfo,
					worker: workerInfo,
				})
			}

			changedUsersWorkersGroups[groupNum][i] = changedUserWorkers
			i++
		}
	}

	wg := sync.WaitGroup{}
	for _, changedUsersWorkers := range changedUsersWorkersGroups {
		wg.Add(1)
		go w.notifyUsers(ctx, changedUsersWorkers, &wg)
	}

	wg.Wait()
}
