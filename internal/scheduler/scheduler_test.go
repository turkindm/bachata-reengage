package scheduler

import (
	"context"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerRunsTaskImmediately(t *testing.T) {
	var runs atomic.Int32

	s := New(log.New(io.Discard, "", 0), Schedule{
		Name:     "test",
		Interval: time.Hour,
		Timeout:  time.Second,
		Run: func(ctx context.Context) error {
			runs.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()

	deadline := time.After(200 * time.Millisecond)
	for runs.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("scheduler did not run immediately")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}
