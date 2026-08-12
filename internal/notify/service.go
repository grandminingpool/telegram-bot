package bot_notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-telegram/bot"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/grandminingpool/telegram-bot/internal/utils/cron"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlannedJob struct {
	definition gocron.JobDefinition
	task       gocron.Task
}

type Service struct {
	scd       gocron.Scheduler
	ctxCancel context.CancelFunc
	workers   *Workers
	payouts   *Payouts
	config    *bot_config.NotifyConfig
	jobs      []gocron.Job
}

func (s *Service) Start(ctx context.Context) error {
	if s.scd != nil {
		return errors.New("notify has been already started")
	}

	scd, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to create notify scheduler: %w", err)
	}

	s.scd = scd

	serviceCtx, cancel := context.WithCancel(ctx)
	s.ctxCancel = cancel

	workersCron, err := cron.FromMinutes(s.config.CheckIntervals.Workers)
	if err != nil {
		cancel()

		return fmt.Errorf("invalid workers check interval: %w", err)
	}

	payoutsCron, err := cron.FromMinutes(s.config.CheckIntervals.Payouts)
	if err != nil {
		cancel()

		return fmt.Errorf("invalid payouts check interval: %w", err)
	}

	plannedJobs := []PlannedJob{
		{
			definition: gocron.CronJob(workersCron, false),
			task:       gocron.NewTask(s.workers.Check, serviceCtx),
		},
		{
			definition: gocron.CronJob(payoutsCron, false),
			task:       gocron.NewTask(s.payouts.Check, serviceCtx),
		},
	}

	for _, pj := range plannedJobs {
		job, err := scd.NewJob(
			pj.definition,
			pj.task,
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if err != nil {
			scd.Shutdown()

			return fmt.Errorf("failed to create notify job: %w", err)
		}

		s.jobs = append(s.jobs, job)
	}

	scd.Start()

	return nil
}

func (s *Service) Stop() error {
	if s.scd == nil && s.ctxCancel == nil {
		return errors.New("service is not started")
	}

	if err := s.scd.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown notify scheduler: %w", err)
	}

	s.scd = nil
	s.ctxCancel = nil
	s.jobs = nil

	return nil
}

func NewService(
	pgConn *pgxpool.Pool,
	blockchainsService *blockchains.Service,
	b *bot.Bot,
	languages *languages.Languages,
	config *bot_config.NotifyConfig,
) *Service {
	workers := &Workers{
		pgConn:             pgConn,
		blockchainsService: blockchainsService,
		b:                  b,
		languages:          languages,
		config:             config,
	}
	payouts := &Payouts{
		pgConn:             pgConn,
		blockchainsService: blockchainsService,
		b:                  b,
		languages:          languages,
		config:             config,
	}

	return &Service{
		workers: workers,
		payouts: payouts,
		config:  config,
		jobs:    []gocron.Job{},
	}
}
