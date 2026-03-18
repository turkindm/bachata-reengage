package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/turkindm/bachata-reengage/internal/api"
	"github.com/turkindm/bachata-reengage/internal/config"
	"github.com/turkindm/bachata-reengage/internal/metrics"
	"github.com/turkindm/bachata-reengage/internal/reminders"
	"github.com/turkindm/bachata-reengage/internal/scheduler"
	"github.com/turkindm/bachata-reengage/internal/store"
	"github.com/turkindm/bachata-reengage/internal/tasks"
)

type App struct {
	scheduler   *scheduler.Scheduler
	logger      *zap.Logger
	metricsHTTP *http.Server
	closeStore  func() error
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.TaskTimeout)
	defer cancel()

	pgStore, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	apiClient := api.NewClient(cfg.APIBaseURL, cfg.APIToken, httpClient)
	serviceMetrics := metrics.New()

	source := &chatSource{
		client:    apiClient,
		pageLimit: cfg.PageLimit,
	}
	service := reminders.NewService(source, pgStore, logger, serviceMetrics, time.Now, cfg.LookbackWindow)
	task := tasks.NewSyncTask(service)

	stdLogger := zap.NewStdLog(logger)
	schedule := scheduler.Schedule{
		Name:     task.Name(),
		Interval: cfg.PollInterval,
		Timeout:  maxDuration(cfg.TaskTimeout, cfg.PollInterval),
		Run:      task.Run,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", serviceMetrics.Handler())

	return &App{
		scheduler: scheduler.New(stdLogger, schedule),
		logger:    logger,
		metricsHTTP: &http.Server{
			Addr:    cfg.MetricsAddr,
			Handler: mux,
			BaseContext: func(net.Listener) context.Context {
				return context.Background()
			},
		},
		closeStore: pgStore.Close,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := a.metricsHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("run metrics server: %w", err)
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.metricsHTTP.Shutdown(shutdownCtx)
		_ = a.closeStore()
		_ = a.logger.Sync()
	}()

	runDone := make(chan error, 1)
	go func() {
		runDone <- a.scheduler.Run(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case err := <-runDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type chatSource struct {
	client    *api.Client
	pageLimit int
}

func (s *chatSource) ListRecentClientMessages(ctx context.Context, start, stop time.Time) ([]reminders.Message, error) {
	var items []reminders.Message
	for page := 0; ; page++ {
		resp, err := s.client.ListRecentClientMessages(ctx, start, stop, page, s.pageLimit)
		if err != nil {
			return nil, err
		}

		for _, msg := range resp.Items {
			items = append(items, reminders.Message{
				DialogID: msg.DialogID,
				WhoSend:  msg.WhoSend,
				SentAt:   msg.DateTimeUTC,
			})
		}

		if len(resp.Items) < s.pageLimit {
			break
		}
	}

	return items, nil
}

func (s *chatSource) GetDialog(ctx context.Context, dialogID int64) (reminders.Dialog, error) {
	dialog, err := s.client.GetDialog(ctx, dialogID)
	if err != nil {
		return reminders.Dialog{}, err
	}

	messages := make([]reminders.Message, 0, len(dialog.Messages))
	for _, msg := range dialog.Messages {
		messages = append(messages, reminders.Message{
			DialogID: msg.DialogID,
			WhoSend:  msg.WhoSend,
			SentAt:   msg.DateTimeUTC,
		})
	}

	return reminders.Dialog{
		ID:       dialog.ID,
		ClientID: dialog.ClientID,
		Phone:    dialog.Client.Phone,
		Messages: messages,
	}, nil
}

func maxDuration(values ...time.Duration) time.Duration {
	var max time.Duration
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
