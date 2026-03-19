package tasks

import "context"

// Runner is anything that can execute a sync run.
type Runner interface {
	Run(context.Context) error
}

// SyncTask wraps a Runner and gives it a stable Name() for the scheduler.
type SyncTask struct {
	runner Runner
}

func NewSyncTask(runner Runner) *SyncTask {
	return &SyncTask{runner: runner}
}

func (t *SyncTask) Name() string {
	return "reminder-sync"
}

func (t *SyncTask) Run(ctx context.Context) error {
	return t.runner.Run(ctx)
}
