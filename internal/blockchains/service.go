package blockchains

import (
	"context"
	"database/sql"
	"fmt"

	poolAPIClient "github.com/grandminingpool/telegram-bot/internal/clients/pool_api"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc"
)

type PoolAPIDB struct {
	URL        string `db:"pool_api_url"`
	TLSCA      string `db:"pool_api_tls_ca"`
	ServerName string `db:"pool_api_server_name"`
}

type BlockchainDB struct {
	Coin       string `db:"coin"`
	Name       string `db:"name"`
	Ticker     string `db:"ticker"`
	AtomicUnit uint16 `db:"atomic_unit"`
	PoolAPIDB
}

type BlockchainInfo struct {
	ID         int16
	Coin       string
	Name       string
	Ticker     string
	AtomicUnit uint16
}

type Blockchain struct {
	info BlockchainInfo
	conn *grpc.ClientConn
}

type Service struct {
	pgConn      *sqlx.DB
	blockchains map[string]Blockchain
}

func (s *Service) getBlockchainsFromDB(ctx context.Context) ([]BlockchainDB, error) {
	blockchains := []BlockchainDB{}
	if err := s.pgConn.SelectContext(ctx, &blockchains, "SELECT * FROM blockchains"); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query blockchains: %w", err)
	}

	return blockchains, nil
}

func (s *Service) GetBlockchains() []BlockchainInfo {
	blockchains := make([]BlockchainInfo, 0, len(s.blockchains))

	for _, b := range s.blockchains {
		blockchains = append(blockchains, b.info)
	}

	return blockchains
}

func (s *Service) Start(ctx context.Context, certsPath string) error {
	blockchains, err := s.getBlockchainsFromDB(ctx)
	if err != nil {
		return err
	}

	for _, b := range blockchains {
		conn, err := poolAPIClient.NewClient(b.PoolAPIDB.URL, certsPath, b.PoolAPIDB.TLSCA, b.PoolAPIDB.ServerName)
		if err != nil {
			s.Close()

			return fmt.Errorf("failed to create blockchain pool api client (coin: %s), error: %w", b.Coin, err)
		}

		s.blockchains[b.Coin] = Blockchain{
			info: BlockchainInfo{
				Coin:       b.Coin,
				Name:       b.Name,
				Ticker:     b.Ticker,
				AtomicUnit: b.AtomicUnit,
			},
			conn: conn,
		}
	}

	return nil
}

func (s *Service) Close() {
	for _, b := range s.blockchains {
		b.conn.Close()
	}

	clear(s.blockchains)
}

func NewService(pgConn *sqlx.DB) *Service {
	return &Service{
		pgConn:      pgConn,
		blockchains: make(map[string]Blockchain),
	}
}
