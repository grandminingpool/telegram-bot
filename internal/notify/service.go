package botNotify

import (
	"context"

	"github.com/grandminingpool/telegram-bot/internal/blockchains"
)

type Service struct {
	blockchainsService *blockchains.Service
}

func (s *Service) Start(ctx context.Context) error {

}

func (s *Service) Stop() error {
	
}