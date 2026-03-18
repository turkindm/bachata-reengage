package tasks

import (
	"context"
)

type Runner interface {
	Run(context.Context) error
}

type SyncTask struct {
	service Runner
}

func NewSyncTask(service Runner) *SyncTask {
	return &SyncTask{service: service}
}

func (t *SyncTask) Name() string {
	return "reminder-sync"
}

func (t *SyncTask) Run(ctx context.Context) error {
	return t.service.Run(ctx)
}
