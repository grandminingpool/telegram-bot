package services

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"time"

	pool_proto "github.com/grandminingpool/pool-api-proto/generated/pool"
	pool_miners_proto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	pool_payouts_proto "github.com/grandminingpool/pool-api-proto/generated/pool_payouts"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/jackc/pgx/v5/pgxpool"
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
	Balances map[string]*pool_payouts_proto.MinerBalance
}

type UserPoolWorkers struct {
	Coin    string
	Workers map[string]*pool_miners_proto.MinerWorkers
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
	Solo        bool
	Hashrate    *big.Int
	ConnectedAt time.Time
}

type UserWalletService struct {
	pgConn             *pgxpool.Pool
	blockchainsService *blockchains.Service
}

func (w *UserWalletService) FindBlockchains(ctx context.Context, userID int64) ([]blockchains.BlockchainInfo, error) {
	rows, err := w.pgConn.Query(ctx, "SELECT DISTINCT blockchain_coin FROM user_wallets WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d) blockchains: %w", userID, err)
	}
	defer rows.Close()

	blockchainsInfo := w.blockchainsService.GetBlockchainsInfo()
	userBlockchains := []blockchains.BlockchainInfo{}

	for rows.Next() {
		var coin string
		if err := rows.Scan(&coin); err != nil {
			return nil, fmt.Errorf("failed to scan user (id: %d) blockchain coin: %w", userID, err)
		} else {
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
	newCtx, cancel := w.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	for _, coin := range coins {
		blockchain, err := w.blockchainsService.GetInfo(coin)
		if err != nil {
			return nil, err
		}

		conn, err := w.blockchainsService.GetConnection(blockchain.Coin)
		if err != nil {
			return nil, err
		}

		client := pool_proto.NewPoolServiceClient(conn)

		go func(c context.Context, b *blockchains.BlockchainInfo, cl pool_proto.PoolServiceClient) {
			poolInfo, err := cl.GetPoolInfo(c, &emptypb.Empty{})
			if err != nil {
				errCh <- fmt.Errorf("failed to get blockchain (coin: %s) pool info: %w", b.Coin, err)
				return
			}
			resultCh <- PoolInfo{
				Blockchain: b,
				Host:       poolInfo.Host,
				MinPayout:  &poolInfo.PayoutsInfo.MinPayout,
			}
		}(newCtx, &blockchain, client)
	}

	for i := 0; i < len(coins); i++ {
		select {
		case err := <-errCh:
			return nil, err
		case poolInfo := <-resultCh:
			poolsInfoMap[poolInfo.Blockchain.Coin] = poolInfo
		}
	}

	return poolsInfoMap, nil
}

func (w *UserWalletService) getWalletsMap(ctx context.Context, userID int64) (map[string]UserPoolWallets, error) {
	walletsMap := make(map[string]UserPoolWallets)
	rows, err := w.pgConn.Query(ctx, "SELECT id, blockchain_coin, wallet, added_at from user_wallets WHERE user_id = $1 ORDER BY added_at", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user (id: %d) wallets: %w", userID, err)
	}
	defer rows.Close()

	coins := []string{}
	for rows.Next() {
		var (
			id           int64
			coin, wallet string
			addedAt      time.Time
		)
		if err := rows.Scan(&id, &coin, &wallet, &addedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user (id: %d) wallets columns: %w", userID, err)
		}

		walletItem := UserWalletInfo{
			ID:      id,
			Wallet:  wallet,
			AddedAt: addedAt,
		}
		entry, exists := walletsMap[coin]
		if !exists {
			coins = append(coins, coin)
		}
		entry.Wallets = append(entry.Wallets, walletItem)
		walletsMap[coin] = entry
	}

	if len(coins) > 0 {
		poolsInfoMap, err := w.getPoolsInfoMap(ctx, coins)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) wallets pools info: %w", userID, err)
		}

		for coin, wallets := range walletsMap {
			poolInfo, ok := poolsInfoMap[coin]
			if ok {
				walletsMap[coin] = UserPoolWallets{
					Pool:    &poolInfo,
					Wallets: wallets.Wallets,
				}
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
	newCtx, cancel := w.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	for coin, userWallets := range walletsMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) wallets blockchain connection: %w", userID, err)
		}

		client := pool_payouts_proto.NewPoolPayoutsServiceClient(conn)
		addresses := make([]string, 0, len(userWallets.Wallets))
		for _, wi := range userWallets.Wallets {
			addresses = append(addresses, wi.Wallet)
		}

		go func(c context.Context, cn string, adds []string, cl pool_payouts_proto.PoolPayoutsServiceClient) {
			balances, err := cl.GetMinersBalancesFromList(c, &pool_miners_proto.MinerAddressesRequest{
				Addresses: adds,
			})
			if err != nil {
				errCh <- fmt.Errorf("failed to get user (id: %d) blockchain (coin: %s) wallets balances: %w", userID, cn, err)
				return
			}
			resultCh <- UserPoolBalances{
				Coin:     cn,
				Balances: balances.Balances,
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
					var balanceValue uint64
					if balance, ok := userBalances.Balances[wi.Wallet]; ok {
						balanceValue = balance.Balance
					}

					wallets = append(wallets, UserPoolWallet{
						Pool:    userWallets.Pool,
						Wallet:  wi.Wallet,
						Balance: balanceValue,
						AddedAt: wi.AddedAt,
					})
				}
			}
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
	newCtx, cancel := w.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	for coin, userWallets := range walletsMap {
		conn, err := w.blockchainsService.GetConnection(coin)
		if err != nil {
			return nil, fmt.Errorf("failed to get user (id: %d) workers blockchain connection: %w", userID, err)
		}

		client := pool_miners_proto.NewPoolMinersServiceClient(conn)
		addresses := make([]string, 0, len(userWallets.Wallets))
		for _, wi := range userWallets.Wallets {
			addresses = append(addresses, wi.Wallet)
		}

		go func(c context.Context, cn string, adds []string, cl pool_miners_proto.PoolMinersServiceClient) {
			workers, err := cl.GetMinersWorkersFromList(c, &pool_miners_proto.MinerAddressesRequest{
				Addresses: adds,
			})
			if err != nil {
				errCh <- fmt.Errorf("failed to get user (id: %d) blockchain (coin: %s) wallets workers: %w", userID, cn, err)
				return
			}
			resultCh <- UserPoolWorkers{
				Coin:    cn,
				Workers: workers.Workers,
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
								Solo:        wk.Solo,
								Hashrate:    new(big.Int).SetBytes(wk.Hashrate),
								ConnectedAt: wk.ConnectedAt.AsTime(),
							})
						}
					}
				}
			}
		}
	}

	sort.Slice(workers, func(i, j int) bool {
		return workers[i].ConnectedAt.Before(workers[j].ConnectedAt)
	})

	return workers, nil
}

func (w *UserWalletService) FindBlockchainWallets(ctx context.Context, userID int64, coin string) ([]UserWalletInfo, error) {
	wallets := []UserWalletInfo{}
	rows, err := w.pgConn.Query(ctx, "SELECT id, wallet FROM user_wallets WHERE user_id = $1 AND blockchain_coin = $2", userID, coin)
	if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d) blockchain (coin: %s) wallets: %w", userID, coin, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id     int64
			wallet string
		)
		if err := rows.Scan(&id, &wallet); err != nil {
			return nil, fmt.Errorf("failed to scan user (id: %d) blockchain (coin: %s) wallet: %w", userID, coin, err)
		} else {
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
	if err := w.pgConn.QueryRow(ctx, `SELECT COUNT(*)
		FROM user_wallets
		WHERE user_id = $1 AND blockchain_coin = $2`, userID, coin).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count user wallets (id: %d, coin: %s), error: %w", userID, coin, err)
	}

	return count, nil
}

func (w *UserWalletService) CheckDuplicates(ctx context.Context, userID int64, coin, wallet string) (bool, error) {
	var count int
	if err := w.pgConn.QueryRow(ctx, `SELECT COUNT(*)
		FROM user_wallets
		WHERE user_id = $1 AND blockchain_coin = $2 AND wallet = $3`,
		userID, coin, wallet).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to count duplicate user wallets (id: %d, coin: %s, wallet: %s), error: %w", userID, coin, wallet, err)
	}

	return count > 0, nil
}

func (w *UserWalletService) Add(ctx context.Context, userID int64, coin, wallet string) error {
	if _, err := w.pgConn.Exec(ctx, `INSERT INTO user_wallets (user_id, blockchain_coin, wallet) VALUES ($1, $2, $3)`, userID, coin, wallet); err != nil {
		return fmt.Errorf("failed to add user wallet (id: %d, coin: %s,  wallet: %s), error: %w", userID, coin, wallet, err)
	}

	return nil
}

func (w *UserWalletService) Remove(ctx context.Context, id int64) error {
	if _, err := w.pgConn.Exec(ctx, `DELETE FROM user_wallets WHERE id = $1`, id); err != nil {
		return fmt.Errorf("failed to remove user wallet (id: %d), error: %w", id, err)
	}

	return nil
}

func NewUserWalletService(pgConn *pgxpool.Pool, blockchainsService *blockchains.Service) *UserWalletService {
	return &UserWalletService{
		pgConn:             pgConn,
		blockchainsService: blockchainsService,
	}
}
