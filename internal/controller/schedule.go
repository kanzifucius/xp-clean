package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kanzifucius/xp-clean/internal/conf"
)

// RunnerFunc is invoked on each schedule tick.
type RunnerFunc func(ctx context.Context, policy conf.Resolved) error

// Scheduler is a minimal wall-clock ticker for MVP (not a full cron parser).
// Production should use robfig/cron or controller-runtime requeue; this stub
// runs at a fixed interval derived from a simple parse of common schedules,
// or falls back to every 6 hours.
type Scheduler struct {
	Interval time.Duration
	Policy   conf.Resolved
	Run      RunnerFunc
	Log      *slog.Logger
}

// NewScheduler builds a scheduler from policy.Schedule (best-effort).
func NewScheduler(policy conf.Resolved, run RunnerFunc, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		Interval: intervalFromCron(policy.Schedule),
		Policy:   policy,
		Run:      run,
		Log:      log,
	}
}

func intervalFromCron(expr string) time.Duration {
	// MVP stub: recognise a few common forms; otherwise 6h.
	switch expr {
	case "@hourly", "0 * * * *":
		return time.Hour
	case "0 */6 * * *":
		return 6 * time.Hour
	case "@daily", "0 0 * * *":
		return 24 * time.Hour
	default:
		return 6 * time.Hour
	}
}

// Start blocks until ctx is cancelled, invoking Run on each tick (including immediately).
func (s *Scheduler) Start(ctx context.Context) error {
	if s.Run == nil {
		return fmt.Errorf("scheduler: Run is nil")
	}
	s.Log.Info("scheduler starting", "interval", s.Interval.String(), "policy", s.Policy.Name)
	if err := s.Run(ctx, s.Policy); err != nil {
		s.Log.Error("scheduled run failed", "err", err)
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Log.Info("scheduler stopped")
			return ctx.Err()
		case <-t.C:
			if err := s.Run(ctx, s.Policy); err != nil {
				s.Log.Error("scheduled run failed", "err", err)
			}
		}
	}
}
