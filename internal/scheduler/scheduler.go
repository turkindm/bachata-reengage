package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Schedule struct {
	Name     string
	Interval time.Duration
	Timeout  time.Duration
	Run      func(context.Context) error
}

type Scheduler struct {
	logger   *log.Logger
	schedule Schedule
}

func New(logger *log.Logger, schedule Schedule) *Scheduler {
	return &Scheduler{
		logger:   logger,
		schedule: schedule,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.runOnce(ctx); err != nil {
		s.logger.Printf("initial run failed: %v", err)
	}

	ticker := time.NewTicker(s.schedule.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("scheduler stopped")
			return nil
		case <-ticker.C:
			if err := s.runOnce(ctx); err != nil {
				s.logger.Printf("scheduled run failed: %v", err)
			}
		}
	}
}

func (s *Scheduler) runOnce(parent context.Context) error {
	runCtx, cancel := context.WithTimeout(parent, s.schedule.Timeout)
	defer cancel()

	start := time.Now()
	if err := s.schedule.Run(runCtx); err != nil {
		return fmt.Errorf("%s: %w", s.schedule.Name, err)
	}

	s.logger.Printf("task %q finished in %s", s.schedule.Name, time.Since(start).Round(time.Millisecond))
	return nil
}
