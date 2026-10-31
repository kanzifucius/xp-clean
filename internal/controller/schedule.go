package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/internal/conf"
)

// RunnerFunc is invoked on each schedule tick for one policy.
type RunnerFunc func(ctx context.Context, policy conf.Resolved) error

// Scheduler is a minimal wall-clock ticker for MVP (not a full cron parser).
// Production should use robfig/cron or controller-runtime requeue; this stub
// runs at a fixed interval derived from a simple parse of common schedules,
// or falls back to every 6 hours. Multiple policies (ConfigMap multi-tenancy)
// are executed sequentially on each tick.
type Scheduler struct {
	Interval time.Duration
	Policies []conf.Resolved
	Run      RunnerFunc
	Log      logr.Logger
}

// NewScheduler builds a scheduler from the first policy's schedule (best-effort)
// and runs every loaded policy on each tick.
func NewScheduler(policies []conf.Resolved, run RunnerFunc, log logr.Logger) *Scheduler {
	if log.GetSink() == nil {
		log = logr.Discard()
	}
	schedule := "0 */6 * * *"
	if len(policies) > 0 && policies[0].Schedule != "" {
		schedule = policies[0].Schedule
	}
	return &Scheduler{
		Interval: intervalFromCron(schedule),
		Policies: policies,
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

func (s *Scheduler) runAll(ctx context.Context) {
	for _, p := range s.Policies {
		if err := s.Run(ctx, p); err != nil {
			s.Log.Error(err, "scheduled run failed", "policy", p.Name)
		}
	}
}

// Start blocks until ctx is cancelled, invoking Run on each tick (including immediately).
func (s *Scheduler) Start(ctx context.Context) error {
	if s.Run == nil {
		return fmt.Errorf("scheduler: Run is nil")
	}
	s.Log.Info("scheduler starting", "interval", s.Interval.String(), "policies", len(s.Policies))
	s.runAll(ctx)
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Log.Info("scheduler stopped")
			return ctx.Err()
		case <-t.C:
			s.runAll(ctx)
		}
	}
}
