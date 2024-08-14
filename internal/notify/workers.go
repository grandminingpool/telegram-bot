package botNotify

import (
	"context"
	"fmt"
	"time"

	poolMinersProto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/hashicorp/go-set/v2"
	"github.com/jmoiron/sqlx"
)

const REMOWED_WORKERS_TEMP_TABLE_NAME = "wallet_workers_to_be_removed"

type Workers struct {
	pgConn             *sqlx.DB
	blockchainsService *blockchains.Service
	config             *botConfig.NotifyConfig
}

type WorkerInfo struct {
	worker      string
	region      string
	solo        bool
	connectedAt time.Time
}

func (w WorkerInfo) Hash() string {
	return w.worker
}

type PoolRequests struct {
	client  poolMinersProto.PoolMinersServiceClient
	wallets [][]string
}

type RemovalUserWorkerDB struct {
	UserID int64  `json:"user_id"`
	Wallet string `json:"wallet"`
	Worker string `json:"worker"`
}

type UserWorkerDB struct {
	RemovalUserWorkerDB
	Region      string    `json:"region"`
	Solo        bool      `json:"solo"`
	ConnectedAt time.Time `json:"connected_at"`
}

type ChangedWorkersDB struct {
	added   []UserWorkerDB
	removed []RemovalUserWorkerDB
}

type UserInfo struct {
	userID int64
	chatID int64
}

type UserWalletWorkers struct {
	UserInfo
	workers *set.HashSet[*WorkerInfo, string]
}

type UserChangedWorkers struct {
	coin     string
	active   []*WorkerInfo
	inactive []*WorkerInfo
}

type PoolWorkers struct {
	groupNum int
	coin     string
	workers  map[string]*poolMinersProto.MinerWorkers
	err      error
}

func (w *Workers) getWorkersMap(ctx context.Context) (map[string]map[string]*UserWalletWorkers, error) {
	workersMap := make(map[string]map[string]*UserWalletWorkers)
	rows, err := w.pgConn.QueryContext(ctx, `SELECT 
		user_wallets.user_id,
		users.chat_id,
		user_wallets.blockchain_coin, 
		user_wallets.wallet,
		wallet_workers.worker,
		wallet_workers.region,
		wallet_workers.solo,
		wallet_workers.connected_at
	FROM wallet_workers
	LEFT JOIN users ON users.id = wallet_workers.user_id
	LEFT JOIN user_wallets ON user_wallets.wallet = wallet_workers.wallet`)
	if err != nil {

	}
	set.New[string](10)
	for rows.Next() {
		var (
			userID, chatID               int64
			coin, wallet, worker, region string
			solo                         bool
			connectedAt                  time.Time
		)

		if err := rows.Scan(&userID, &chatID, &coin, &wallet, &worker, &region, &solo, &connectedAt); err != nil {

		}

		coinWorkersMap, ok := workersMap[coin]
		if !ok {
			workersMap[coin] = make(map[string]*UserWalletWorkers)
		} else {
			workerInfo := &WorkerInfo{
				worker,
				region,
				solo,
				connectedAt,
			}
			coinWorkersWalletsMap, ok := coinWorkersMap[wallet]
			if !ok {
				set.NewHashSet[*WorkerInfo, string](0)
				coinWorkersMap[wallet] = &UserWalletWorkers{
					UserInfo: UserInfo{
						userID,
						chatID,
					},
					workers: set.HashSetFrom[*WorkerInfo, string]([]*WorkerInfo{workerInfo}),
				}
			} else {
				coinWorkersWalletsMap.workers.Insert(workerInfo)
			}
		}
	}

	return workersMap, nil
}

func (w *Workers) getPoolRequestsMap(workersMap map[string]map[string]*UserWalletWorkers) (map[string]*PoolRequests, int, error) {
	poolRequestsMap := make(map[string]*PoolRequests)
	requestsCount := 0
	for coin, coinWorkersMap := range workersMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {

		}

		client := poolMinersProto.NewPoolMinersServiceClient(conn)
		poolRequests := &PoolRequests{
			client:  client,
			wallets: [][]string{},
		}

		groupNum := 0
		requestsCount++
		i := 0
		for wallet := range coinWorkersMap {
			if i > w.config.MaxWalletsInRequest {
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
			result.err = fmt.Errorf("failed to get pool (coin: %w) workers for group: %d, error: %w", coin, groupNum, err)
		} else {
			result.workers = workers.Workers
		}

		resultCh <- result
	}
}

func (w *Workers) addWorkers(ctx context.Context, tx *sqlx.Tx, groupNum int, addedWorkers []UserWorkerDB, errCh chan<- error) {
	select {
	case <-ctx.Done():
		return
	default:
		if _, err := tx.NamedExecContext(ctx, `INSERT INTO wallet_workers (
		   user_id, 
		   wallet, 
		   worker, 
		   region, 
		   solo, 
		   connected_at
	   ) VALUES (:user_id, :wallet, :worker, :region, :solo, :connected_at)`, addedWorkers); err != nil {
			errCh <- fmt.Errorf("failed to insert added workers batch (group num: %d, batch length: %d), error: %w", groupNum, len(addedWorkers), err)
		}
	}
}

func (w *Workers) removeWorkers(ctx context.Context, tx *sqlx.Tx, groupNum int, removedWorkers []RemovalUserWorkerDB, errCh chan<- error) {
	select {
	case <-ctx.Done():
		return
	default:
		if _, err := tx.NamedExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (
			user_id, 
			wallet, 
			worker
		) VALUES (:user_id, :wallet, :worker)`, REMOWED_WORKERS_TEMP_TABLE_NAME), removedWorkers); err != nil {
			errCh <- fmt.Errorf("failed to insert removed workers batch to temp table (group num: %d, batch length: %d), error: %w", groupNum, len(removedWorkers), err)
		}
	}
}

func (w *Workers) Check(ctx context.Context) {
	workersMap, err := w.getWorkersMap(ctx)
	defer clear(workersMap)
	if err != nil {

	}

	poolRequestsMap, requestsCount, err := w.getPoolRequestsMap(workersMap)
	defer clear(poolRequestsMap)
	if err != nil {

	}

	poolWorkersCh := make(chan PoolWorkers, requestsCount)
	close(poolWorkersCh)
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for coin, poolRequests := range poolRequestsMap {
		client := poolRequests.client

		for groupNum := 0; groupNum < len(poolRequests.wallets); groupNum++ {
			go w.getWorkers(newCtx, client, coin, groupNum, poolRequests.wallets[groupNum], poolWorkersCh)
		}
	}

	changedWorkersMap := make(map[UserInfo]map[string]*UserChangedWorkers)
	defer clear(changedWorkersMap)
	for i := 0; i < requestsCount; i++ {
		select {
		case poolWorkers := <-poolWorkersCh:
			if poolWorkers.err != nil {
				//return poolWorkers.err
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

						userInfo := UserInfo{
							userID: userWalletWorkers.userID,
							chatID: userWalletWorkers.chatID,
						}
						userChangedWorkers := &UserChangedWorkers{
							coin:     poolWorkers.coin,
							active:   walletWorkersSet.Difference(userWalletWorkers.workers).Slice(),
							inactive: userWalletWorkers.workers.Difference(walletWorkersSet).Slice(),
						}

						changedUserWorkersMap, ok := changedWorkersMap[userInfo]
						if ok {
							changedUserWorkersMap[wallet] = userChangedWorkers
						} else {
							changedWorkersMap[userInfo][wallet] = userChangedWorkers
						}
					}
				}
			}
		default:
		}

		tx, err := w.pgConn.BeginTxx(ctx, nil)
		if err != nil {

		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TEMP TABLE %s (
			user_id BIGINT NOT NULL,
			wallet TEXT NOT NULL,
			worker TEXT NOT NULL,
			PRIMARY KEY(user_id, wallet, worker)
		)`, REMOWED_WORKERS_TEMP_TABLE_NAME)); err != nil {
			tx.Rollback()
		}

		changedWorkersGroups := []*ChangedWorkersDB{}
		groupNum := 0
		i := 0
		j := 0

		for user, changedUserWorkersMap := range changedWorkersMap {
			if groupNum > w.config.MaxUsersChangesLimit {
				groupNum++
				i = 0
				j = 0
			}

			for wallet, userChangedWorkers := range changedUserWorkersMap {
				for _, workerInfo := range userChangedWorkers.active {
					changedWorkersGroups[groupNum].added[i] = UserWorkerDB{
						RemovalUserWorkerDB: RemovalUserWorkerDB{
							UserID: user.userID,
							Wallet: wallet,
							Worker: workerInfo.worker,
						},
						Region:      workerInfo.region,
						Solo:        workerInfo.solo,
						ConnectedAt: workerInfo.connectedAt,
					}

					i++
				}

				for _, workerInfo := range userChangedWorkers.inactive {
					changedWorkersGroups[groupNum].removed[j] = RemovalUserWorkerDB{
						UserID: user.userID,
						Wallet: wallet,
						Worker: workerInfo.worker,
					}

					j++
				}
			}
		}

		changedWorkersGroupsLen := len(changedWorkersGroups)
		errCh := make(chan error, 2*changedWorkersGroupsLen)
		defer close(errCh)
		for groupNum, changedWorkers := range changedWorkersGroups {
			go w.addWorkers(newCtx, tx, groupNum, changedWorkers.added, errCh)
			go w.removeWorkers(newCtx, tx, groupNum, changedWorkers.removed, errCh)
		}

		for i := 0; i < changedWorkersGroupsLen; i++ {
			select {
			case err := <-errCh:
				tx.Rollback()

				return
			}
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", REMOWED_WORKERS_TEMP_TABLE_NAME)); err != nil {
			tx.Rollback()
		}

		tx.ExecContext(ctx, `DELETE FROM wallet_workers`)

		if err := tx.Commit(); err != nil {

		}
	}
}
