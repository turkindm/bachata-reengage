package tasks

import (
	"context"
	"log"
)

type Pinger interface {
	Ping(context.Context) error
}

type SyncTask struct {
	client Pinger
	logger *log.Logger
}

func NewSyncTask(client Pinger, logger *log.Logger) *SyncTask {
	return &SyncTask{
		client: client,
		logger: logger,
	}
}

func (t *SyncTask) Name() string {
	return "api-health-sync"
}

func (t *SyncTask) Run(ctx context.Context) error {
	t.logger.Printf("task %q started", t.Name())
	return t.client.Ping(ctx)
}
