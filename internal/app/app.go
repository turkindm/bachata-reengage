package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/turkindm/bachata-reengage/internal/api"
	"github.com/turkindm/bachata-reengage/internal/config"
	"github.com/turkindm/bachata-reengage/internal/scheduler"
	"github.com/turkindm/bachata-reengage/internal/tasks"
)

type App struct {
	scheduler *scheduler.Scheduler
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
	}

	apiClient := api.NewClient(cfg.APIBaseURL, cfg.APIKey, httpClient)

	logger := log.New(log.Writer(), "reengage ", log.LstdFlags|log.Lmsgprefix)
	task := tasks.NewSyncTask(apiClient, logger)

	schedule := scheduler.Schedule{
		Name:     task.Name(),
		Interval: cfg.PollInterval,
		Timeout:  maxDuration(cfg.TaskTimeout, cfg.PollInterval),
		Run:      task.Run,
	}

	return &App{
		scheduler: scheduler.New(logger, schedule),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	return a.scheduler.Run(ctx)
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
