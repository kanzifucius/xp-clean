package cleanup

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/metrics"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PolicyView is the subset of config the runner needs (avoids importing internal/conf).
type PolicyView struct {
	Name      string
	DryRun    bool
	Selectors match.Selectors
	TTL       match.TTLConfig
	Deletion  DeletionConfig
}

// RunResult summarises one cleanup pass.
type RunResult struct {
	Traces  []trace.Trace
	Plans   []DeletePlan
	Results []ExecuteResult
}

// ClusterBackend abstracts discovery + delete for live or fake clusters.
type ClusterBackend interface {
	DiscoverRoots(ctx context.Context, sel match.Selectors, ttl match.TTLConfig, now time.Time) ([]discover.RootCandidate, error)
	BuildTrace(ctx context.Context, root *unstructured.Unstructured) (trace.Trace, error)
	Delete(ctx context.Context, ref trace.ObjectRef) error
}

// Runner orchestrates discover → match → trace → plan → execute.
type Runner struct {
	Backend ClusterBackend
	Audit   *audit.Logger
	Out     io.Writer
	Now     func() time.Time
}

// RunOnce performs a single cleanup pass.
func (r *Runner) RunOnce(ctx context.Context, policy PolicyView) (RunResult, error) {
	if r.Backend == nil {
		return RunResult{}, fmt.Errorf("runner: backend is nil")
	}
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now()
	}
	if r.Audit == nil {
		r.Audit = audit.NewLogger(r.Out)
	}

	roots, err := r.Backend.DiscoverRoots(ctx, policy.Selectors, policy.TTL, now)
	if err != nil {
		return RunResult{}, err
	}

	var result RunResult
	for _, root := range roots {
		metrics.Global.TracesEligible.Add(1)
		tr, err := r.Backend.BuildTrace(ctx, root.XR)
		if err != nil {
			return result, fmt.Errorf("build trace: %w", err)
		}
		// Mark root eligibility / keep / paused from candidate.
		for i := range tr.Nodes {
			if tr.Nodes[i].Ref.Key() == tr.Root.Key() {
				tr.Nodes[i].Eligible = root.Eligible
				tr.Nodes[i].Keep = root.Keep
				tr.Nodes[i].Paused = root.Paused
				tr.Nodes[i].ExpiresAt = root.ExpiresAt
			}
		}
		metrics.Global.TracesBuilt.Add(1)
		result.Traces = append(result.Traces, tr)
	}

	plans := make([]DeletePlan, 0, len(result.Traces))
	for _, tr := range result.Traces {
		p := BuildPlan(tr, policy.Deletion, policy.DryRun)
		plans = append(plans, p)
		metrics.Global.DeletePlans.Add(1)
	}
	plans = ApplyBudgets(plans, policy.Deletion)
	result.Plans = plans

	for _, p := range plans {
		er := Execute(ctx, p, policy.Name, r.Backend)
		result.Results = append(result.Results, er)
		_ = r.Audit.LogAll(er.AuditEntries)
		if p.DryRun {
			metrics.Global.DryRuns.Add(int64(len(p.Steps)))
		} else {
			metrics.Global.Deletes.Add(int64(er.Deleted))
		}
	}
	return result, nil
}
