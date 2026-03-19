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

// App wires all components and runs the scheduler + metrics server.
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

	initCtx, cancel := context.WithTimeout(context.Background(), cfg.TaskTimeout)
	defer cancel()

	pgStore, err := store.Open(initCtx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	apiClient := api.NewClient(cfg.APIBaseURL, cfg.APIToken, cfg.OperatorLogin, httpClient, 80)
	svcMetrics := metrics.New()

	source := &chatSource{client: apiClient}
	service := reminders.NewService(source, pgStore, logger, svcMetrics, time.Now, cfg.LookbackWindow, cfg.DryRun)
	task := tasks.NewSyncTask(service)

	stdLogger := zap.NewStdLog(logger)
	schedule := scheduler.Schedule{
		Name:     task.Name(),
		Interval: cfg.PollInterval,
		Timeout:  maxDuration(cfg.TaskTimeout, cfg.PollInterval),
		Run:      task.Run,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", svcMetrics.Handler())

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

// chatSource adapts api.Client to the reminders.Source interface.
type chatSource struct {
	client *api.Client
}

func (s *chatSource) ListDialogs(ctx context.Context, start, stop time.Time) ([]reminders.Dialog, error) {
	apiDialogs, err := s.client.ListDialogs(ctx, start, stop)
	if err != nil {
		return nil, err
	}

	dialogs := make([]reminders.Dialog, 0, len(apiDialogs))
	for _, d := range apiDialogs {
		msgs := make([]reminders.Message, 0, len(d.Messages))
		for _, m := range d.Messages {
			msgs = append(msgs, reminders.Message{
				DialogID: m.DialogID,
				WhoSend:  m.WhoSend,
				SentAt:   m.DateTimeUTC,
			})
		}
		dialogs = append(dialogs, reminders.Dialog{
			ID:       d.ID,
			ClientID: d.ClientID,
			Phone:    d.Phone,
			Messages: msgs,
		})
	}

	return dialogs, nil
}

func (s *chatSource) SendMessage(ctx context.Context, clientID, text string) error {
	return s.client.SendMessage(ctx, clientID, text)
}

func maxDuration(values ...time.Duration) time.Duration {
	var max time.Duration
	for _, v := range values {
		if v > max {
			max = v
		}
	}

	return max
}
