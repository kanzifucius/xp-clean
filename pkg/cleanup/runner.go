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

// HasBlockedPlans reports whether any plan was blocked by safety gates or budgets.
func HasBlockedPlans(plans []DeletePlan) bool {
	for _, p := range plans {
		if len(p.BlockedBy) > 0 {
			return true
		}
	}
	return false
}

// MergeRunResults concatenates results from multiple policy passes.
func MergeRunResults(parts ...RunResult) RunResult {
	var out RunResult
	for _, p := range parts {
		out.Traces = append(out.Traces, p.Traces...)
		out.Plans = append(out.Plans, p.Plans...)
		out.Results = append(out.Results, p.Results...)
	}
	return out
}

// ClusterBackend abstracts discovery + delete for live or fake clusters.
type ClusterBackend interface {
	DiscoverRoots(ctx context.Context, sel match.Selectors, ttl match.TTLConfig, now time.Time) ([]discover.RootCandidate, error)
	BuildTrace(ctx context.Context, root *unstructured.Unstructured) (trace.Trace, error)
	Delete(ctx context.Context, ref trace.ObjectRef) error
}

// DefinitionAttacher enriches a trace with XRD / Composition / Revision nodes.
type DefinitionAttacher interface {
	AttachDefinitions(ctx context.Context, tr *trace.Trace, xrd, xr *unstructured.Unstructured)
}

// XRCounter counts live XR instances for an XRD (zero-XR definition gate).
type XRCounter interface {
	CountXRs(ctx context.Context, info discover.XRDInfo) (int, error)
}

// Runner orchestrates discover → match → trace → plan → execute.
type Runner struct {
	Backend ClusterBackend
	Audit   *audit.Logger
	Out     io.Writer
	Now     func() time.Time
	// PollInterval overrides wait polling cadence (tests use a short interval).
	PollInterval time.Duration
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

	waitTimeout, err := ParseWaitTimeout(policy.Deletion.WaitTimeout)
	if err != nil {
		return RunResult{}, err
	}

	// Per-XRD counts of eligible roots in this run (for plan-time zero-XR gate).
	sameRunByXRD := map[string]int{}
	for _, root := range roots {
		if root.XRD.XRD != nil {
			sameRunByXRD[root.XRD.XRD.GetName()]++
		}
	}

	var result RunResult
	type rootedTrace struct {
		tr   trace.Trace
		root discover.RootCandidate
	}
	built := make([]rootedTrace, 0, len(roots))

	attacher, _ := r.Backend.(DefinitionAttacher)
	counter, _ := r.Backend.(XRCounter)
	getter, _ := r.Backend.(ObjectGetter)

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
		if policy.Deletion.Scope == ScopeAll && attacher != nil {
			attacher.AttachDefinitions(ctx, &tr, root.XRD.XRD, root.XR)
		}
		metrics.Global.TracesBuilt.Add(1)
		result.Traces = append(result.Traces, tr)
		built = append(built, rootedTrace{tr: tr, root: root})
	}

	plans := make([]DeletePlan, 0, len(built))
	for _, rt := range built {
		remainingAfterRun := 0
		if policy.Deletion.Scope == ScopeAll && counter != nil && rt.root.XRD.XRD != nil {
			total, err := counter.CountXRs(ctx, rt.root.XRD)
			if err != nil {
				return result, fmt.Errorf("count XRs for %s: %w", rt.root.XRD.XRD.GetName(), err)
			}
			same := sameRunByXRD[rt.root.XRD.XRD.GetName()]
			remainingAfterRun = total - same
			if remainingAfterRun < 0 {
				remainingAfterRun = 0
			}
		}
		p := BuildPlanWithRemaining(rt.tr, policy.Deletion, policy.DryRun, remainingAfterRun)
		plans = append(plans, p)
		metrics.Global.DeletePlans.Add(1)
	}
	plans = ApplyBudgets(plans, policy.Deletion)
	result.Plans = plans

	for i, p := range plans {
		opts := ExecuteOptions{
			WaitTimeout:  waitTimeout,
			PollInterval: r.PollInterval,
			Getter:       getter,
		}
		if policy.Deletion.Scope == ScopeAll && counter != nil && i < len(built) && built[i].root.XRD.XRD != nil {
			info := built[i].root.XRD
			opts.CountRemainingXRs = func(ctx context.Context) (int, error) {
				return counter.CountXRs(ctx, info)
			}
		}
		er := Execute(ctx, p, policy.Name, r.Backend, opts)
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
