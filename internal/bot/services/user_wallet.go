package services

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"time"

	poolProto "github.com/grandminingpool/pool-api-proto/generated/pool"
	poolMinersProto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	poolPayoutsProto "github.com/grandminingpool/pool-api-proto/generated/pool_payouts"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/jmoiron/sqlx"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PoolInfo struct {
	Blockchain *blockchains.BlockchainInfo
	Host       string
	MinPayout  *uint64
}

type UserWalletInfo struct {
	ID      int64
	Wallet  string
	AddedAt time.Time
}

type UserPoolWallets struct {
	Pool    *PoolInfo
	Wallets []UserWalletInfo
}

type UserPoolBalances struct {
	Coin     string
	Balances map[string]*poolPayoutsProto.MinerBalance
}

type UserPoolWorkers struct {
	Coin    string
	Workers map[string]*poolMinersProto.MinerWorkers
}

type UserPoolWallet struct {
	Pool    *PoolInfo
	Wallet  string
	Balance uint64
	AddedAt time.Time
}

type UserPoolWorker struct {
	Pool        *PoolInfo
	Wallet      string
	Worker      string
	Region      string
	Solo        bool
	Hashrate    *big.Int
	ConnectedAt time.Time
}

type UserWalletService struct {
	pgConn             *sqlx.DB
	blockchainsService *blockchains.Service
}

func (w *UserWalletService) FindBlockchains(ctx context.Context, userID int64) ([]blockchains.BlockchainInfo, error) {
	rows, err := w.pgConn.QueryContext(ctx, "SELECT DISTINCT blockchain_coin FROM user_wallets WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d) blockchains: %w", userID, err)
	}

	blockchainsInfo := w.blockchainsService.GetBlockchainsInfo()
	userBlockchains := []blockchains.BlockchainInfo{}

	for rows.Next() {
		var coin string
		if err := rows.Scan(&coin); err == nil {
			idx := slices.IndexFunc(blockchainsInfo, func(blockchain blockchains.BlockchainInfo) bool {
				return blockchain.Coin == coin
			})

			if idx != -1 {
				userBlockchains = append(userBlockchains, blockchainsInfo[idx])
			}
		}
	}

	return userBlockchains, nil
}

func (w *UserWalletService) getPoolsInfoMap(ctx context.Context, coins []string) (map[string]PoolInfo, error) {
	poolsInfoMap := make(map[string]PoolInfo)
	resultCh := make(chan PoolInfo, len(coins))
	errCh := make(chan error, len(coins))
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(resultCh)
	defer close(errCh)

	for _, coin := range coins {
		blockchain, err := w.blockchainsService.GetInfo(coin)
		if err != nil {
			return nil, err
		}

		conn, err := w.blockchainsService.GetConnection(blockchain.Coin)
		if err != nil {
			return nil, err
		}

		client := poolProto.NewPoolServiceClient(conn)

		go func(c context.Context, b *blockchains.BlockchainInfo, cl poolProto.PoolServiceClient) {
			select {
			case <-c.Done():
				return
			default:
				poolInfo, err := cl.GetPoolInfo(ctx, &emptypb.Empty{})
				if err != nil {
					errCh <- fmt.Errorf("failed to get blockchain (coin: %s) pool info: %w", b.Coin, err)
				} else {
					resultCh <- PoolInfo{
						Blockchain: b,
						Host:       poolInfo.Host,
						MinPayout:  poolInfo.PayoutsInfo.MinPayout,
					}
				}
			}
		}(newCtx, blockchain, client)
	}

	for i := 0; i < len(coins); i++ {
		select {
		case err := <-errCh:
			return nil, err
		case poolInfo := <-resultCh:
			poolsInfoMap[poolInfo.Blockchain.Coin] = poolInfo
		default:
		}
	}

	return poolsInfoMap, nil
}

func (w *UserWalletService) getWalletsMap(ctx context.Context, userID int64) (map[string]UserPoolWallets, error) {
	walletsMap := make(map[string]UserPoolWallets)
	rows, err := w.pgConn.QueryContext(ctx, "SELECT id, blockchain_coin, wallet, added_at from user_wallets WHERE user_id = $1 ORDER BY added_at", userID)
	coins := []string{}
	if err != nil {
		return nil, fmt.Errorf("failed to query user (id: %d) wallets: %w", userID, err)
	}

	for rows.Next() {
		var (
			id           int64
			coin, wallet string
			addedAt      time.Time
		)
		if err := rows.Scan(&id, &coin, &wallet, &addedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user (id: %d) wallets columns: %w", userID, err)
		}

		coins = append(coins, coin)
		walletItem := UserWalletInfo{
			ID:      id,
			Wallet:  wallet,
			AddedAt: addedAt,
		}
		wallets, ok := walletsMap[coin]
		if ok {
			wallets.Wallets = append(wallets.Wallets, walletItem)
		} else {
			walletsMap[coin] = UserPoolWallets{
				Pool:    nil,
				Wallets: []UserWalletInfo{walletItem},
			}
		}
	}

	if len(coins) > 0 {
		poolsInfoMap, err := w.getPoolsInfoMap(ctx, coins)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) wallets pools info: %w", userID, err)
		}

		for coin, wallets := range walletsMap {
			poolInfo, ok := poolsInfoMap[coin]
			if ok {
				wallets.Pool = &poolInfo
			}
		}
	}

	return walletsMap, nil
}

func (w *UserWalletService) FindWallets(ctx context.Context, userID int64) ([]UserPoolWallet, error) {
	walletsMap, err := w.getWalletsMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	balancesMap := make(map[string]map[string]uint64)
	defer clear(balancesMap)

	resultCh := make(chan UserPoolBalances, len(walletsMap))
	errCh := make(chan error, len(walletsMap))
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(resultCh)
	defer close(errCh)

	for coin, userWallets := range walletsMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) wallets blockchain connection: %w", userID, err)
		}

		client := poolPayoutsProto.NewPoolPayoutsServiceClient(conn)
		addresses := make([]string, 0, len(userWallets.Wallets))
		for _, wi := range userWallets.Wallets {
			addresses = append(addresses, wi.Wallet)
		}

		go func(c context.Context, cn string, adds []string, cl poolPayoutsProto.PoolPayoutsServiceClient) {
			select {
			case <-c.Done():
				return
			default:
				balances, err := client.GetBalances(ctx, &poolMinersProto.MinerAddressesRequest{
					Addresses: adds,
				})
				if err != nil {
					errCh <- fmt.Errorf("failed to get user (id: %d) blockchain (coin: %s) wallets balances: %w", userID, cn, err)
				} else {
					resultCh <- UserPoolBalances{
						Coin:     cn,
						Balances: balances.Balances,
					}
				}
			}
		}(newCtx, coin, addresses, client)
	}

	wallets := []UserPoolWallet{}
	for i := 0; i < len(walletsMap); i++ {
		select {
		case err := <-errCh:
			return nil, err
		case userBalances := <-resultCh:
			userWallets, ok := walletsMap[userBalances.Coin]
			if ok {
				for _, wi := range userWallets.Wallets {
					balance, ok := userBalances.Balances[wi.Wallet]

					if ok {
						wallets = append(wallets, UserPoolWallet{
							Pool:    userWallets.Pool,
							Wallet:  wi.Wallet,
							Balance: balance.Balance,
							AddedAt: wi.AddedAt,
						})
					}
				}
			}
		default:
		}
	}

	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].AddedAt.Before(wallets[j].AddedAt)
	})

	return wallets, nil
}

func (w *UserWalletService) FindWorkers(ctx context.Context, userID int64) ([]UserPoolWorker, error) {
	walletsMap, err := w.getWalletsMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	workersMap := make(map[string]UserPoolWorkers)
	defer clear(workersMap)

	resultCh := make(chan UserPoolWorkers, len(walletsMap))
	errCh := make(chan error, len(walletsMap))
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(resultCh)
	defer close(errCh)

	for coin, userWallets := range walletsMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) workers blockchain connection: %w", userID, err)
		}

		client := poolMinersProto.NewPoolMinersServiceClient(conn)
		addresses := make([]string, 0, len(userWallets.Wallets))
		for _, wi := range userWallets.Wallets {
			addresses = append(addresses, wi.Wallet)
		}

		go func(c context.Context, cn string, adds []string, cl poolMinersProto.PoolMinersServiceClient) {
			select {
			case <-c.Done():
				return
			default:
				workers, err := client.GetWorkers(ctx, &poolMinersProto.MinerAddressesRequest{
					Addresses: adds,
				})
				if err != nil {
					errCh <- fmt.Errorf("failed to get user (id: %d) blockchain (coin: %s) wallets workers: %w", userID, cn, err)
				} else {
					resultCh <- UserPoolWorkers{
						Coin:    cn,
						Workers: workers.Workers,
					}
				}
			}
		}(newCtx, coin, addresses, client)
	}

	workers := []UserPoolWorker{}
	for i := 0; i < len(walletsMap); i++ {
		select {
		case err := <-errCh:
			return nil, err
		case userWorkers := <-resultCh:
			userWallets, ok := walletsMap[userWorkers.Coin]
			if ok {
				for _, wi := range userWallets.Wallets {
					wks, ok := userWorkers.Workers[wi.Wallet]
					if ok {
						for _, wk := range wks.Workers {
							workers = append(workers, UserPoolWorker{
								Pool:        userWallets.Pool,
								Wallet:      wi.Wallet,
								Worker:      wk.Worker,
								Region:      wk.Region,
								Solo:        wk.Solo,
								Hashrate:    new(big.Int).SetBytes(wk.Hashrate),
								ConnectedAt: wk.ConnectedAt.AsTime(),
							})
						}
					}
				}
			}
		default:
		}
	}

	sort.Slice(workers, func(i, j int) bool {
		return workers[i].ConnectedAt.Before(workers[j].ConnectedAt)
	})

	return workers, nil
}

func (w *UserWalletService) FindBlockchainWallets(ctx context.Context, userID int64, coin string) ([]UserWalletInfo, error) {
	wallets := []UserWalletInfo{}
	rows, err := w.pgConn.QueryContext(ctx, "SELECT id, wallet FROM user_wallets WHERE user_id = $1 AND blockchain_coin = $2", userID, coin)
	if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d) blockchain (coin: %s) wallets: %w", userID, coin, err)
	}

	for rows.Next() {
		var (
			id     int64
			wallet string
		)
		if err := rows.Scan(&id, &wallet); err == nil {
			wallets = append(wallets, UserWalletInfo{
				ID:     id,
				Wallet: wallet,
			})
		}
	}

	return wallets, nil
}

func (w *UserWalletService) Count(ctx context.Context, userID int64, coin string) (int, error) {
	var count int
	if err := w.pgConn.GetContext(ctx, &count, `SELECT COUNT(*) 
		FROM user_wallets 
		WHERE user_id = $1 AND blockchain_coin = $2`, userID, coin); err != nil {
		return 0, fmt.Errorf("failed to count user wallets (id: %d, coin: %s), error: %w", userID, coin, err)
	}

	return count, nil
}

func (w *UserWalletService) CheckDuplicates(ctx context.Context, userID int64, coin, wallet string) (bool, error) {
	var count int
	if err := w.pgConn.GetContext(ctx, &count, `SELECT COUNT(*) 
		FROM user_wallets 
		WHERE user_id = $1 AND blockchain_coin = $2 AND wallet = $3`,
		userID, coin, wallet); err != nil {
		return false, fmt.Errorf("failed to count duplicate user wallets (id: %d, coin: %s, wallet: %s), error: %w", userID, coin, wallet, err)
	}

	return count > 0, nil
}

func (w *UserWalletService) Add(ctx context.Context, userID int64, coin, wallet string) error {
	if _, err := w.pgConn.ExecContext(ctx, `INSERT INTO user_wallets (user_id, blockchain_coin, wallet) VALUES ($1, $2, $3)`, userID, coin, wallet); err != nil {
		return fmt.Errorf("failed to add user wallet (id: %d, coin: %s,  wallet: %s), error: %w", userID, coin, wallet, err)
	}

	return nil
}

func (w *UserWalletService) Remove(ctx context.Context, id int64) error {
	if _, err := w.pgConn.ExecContext(ctx, `DELETE FROM user_wallets WHERE id = $1`, id); err != nil {
		return fmt.Errorf("failed to remove user wallet (id: %d), error: %w", id, err)
	}

	return nil
}

func NewUserWalletService(pgConn *sqlx.DB, blockchainsService *blockchains.Service) *UserWalletService {
	return &UserWalletService{
		pgConn:             pgConn,
		blockchainsService: blockchainsService,
	}
}
