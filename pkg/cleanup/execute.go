package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/trace"
)

// Deleter performs a live delete of one object. Dry-run never calls it.
type Deleter interface {
	Delete(ctx context.Context, ref trace.ObjectRef) error
}

// ExecuteOptions controls wait-after-delete and definition-layer gates.
type ExecuteOptions struct {
	WaitTimeout  time.Duration
	PollInterval time.Duration
	Getter       ObjectGetter
	// CountRemainingXRs is invoked before the first definition-layer step.
	// When it returns a count > 0, remaining definition steps are blocked.
	CountRemainingXRs func(ctx context.Context) (int, error)
}

// ExecuteResult summarises a plan execution.
type ExecuteResult struct {
	TraceID      string
	DryRun       bool
	Planned      int
	Deleted      int
	Skipped      int
	Errors       []string
	AuditEntries []audit.Entry
}

// Execute runs a DeletePlan. When plan.DryRun is true, no deletes occur.
// After each live delete it polls until the object is gone or waitTimeout elapses.
// Definition steps run only when CountRemainingXRs reports zero XRs (when set).
func Execute(ctx context.Context, plan DeletePlan, policy string, del Deleter, opts ExecuteOptions) ExecuteResult {
	res := ExecuteResult{
		TraceID: plan.TraceID,
		DryRun:  plan.DryRun,
		Planned: len(plan.Steps),
	}

	// Fully blocked plans have no steps. Partial BlockedBy (e.g. XRD gate or
	// zero-XR omitting definitions) still runs the remaining instance steps.
	if len(plan.BlockedBy) > 0 && len(plan.Steps) == 0 {
		for _, b := range plan.BlockedBy {
			res.Errors = append(res.Errors, "blocked: "+b)
		}
		return res
	}
	for _, b := range plan.BlockedBy {
		res.Errors = append(res.Errors, "blocked: "+b)
	}

	definitionGateChecked := false
	definitionsAllowed := true

	for _, step := range plan.Steps {
		entry := audit.Entry{
			Policy:  policy,
			TraceID: plan.TraceID,
			Action:  step.Action,
			DryRun:  plan.DryRun,
			Object: audit.ObjectRef{
				APIVersion: step.Ref.Group + "/" + step.Ref.Version,
				Kind:       step.Ref.Kind,
				Namespace:  step.Ref.Namespace,
				Name:       step.Ref.Name,
				UID:        string(step.Ref.UID),
			},
			Reason: step.Reason,
		}

		if plan.DryRun {
			entry.Action = "dry-run-delete"
			res.AuditEntries = append(res.AuditEntries, entry)
			res.Skipped++
			continue
		}

		if isDefinitionKind(step.Kind) {
			if !definitionGateChecked {
				definitionGateChecked = true
				if opts.CountRemainingXRs != nil {
					n, err := opts.CountRemainingXRs(ctx)
					if err != nil {
						msg := fmt.Sprintf("definition gate: %v", err)
						res.Errors = append(res.Errors, msg)
						definitionsAllowed = false
					} else if n > 0 {
						msg := fmt.Sprintf("definition delete blocked: %d XR(s) still remain", n)
						res.Errors = append(res.Errors, msg)
						definitionsAllowed = false
					}
				}
			}
			if !definitionsAllowed {
				entry.Action = "skip-definition"
				entry.Reason = "zero_xr_gate"
				res.AuditEntries = append(res.AuditEntries, entry)
				res.Skipped++
				continue
			}
		}

		if del == nil {
			res.Errors = append(res.Errors, "deleter is nil")
			res.Skipped++
			continue
		}
		if err := del.Delete(ctx, step.Ref); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", step.Ref.Key(), err))
			res.Skipped++
			continue
		}

		wait := WaitUntilGone(ctx, opts.Getter, step.Ref, opts.WaitTimeout, opts.PollInterval)
		if wait.Err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: wait: %v", step.Ref.Key(), wait.Err))
			res.Skipped++
			// Do not continue the plan after a wait failure — later definition
			// steps must not race a still-terminating instance tree.
			res.AuditEntries = append(res.AuditEntries, entry)
			skipRemaining(&res, plan, policy, step, "wait_error")
			return res
		}
		if wait.TimedOut {
			reason := FormatStuckFinalizers(wait.Finalizers)
			timeoutEntry := entry
			timeoutEntry.Action = "wait-timeout"
			timeoutEntry.Reason = reason
			res.AuditEntries = append(res.AuditEntries, timeoutEntry)
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", step.Ref.Key(), reason))
			res.Skipped++
			skipRemaining(&res, plan, policy, step, "wait_timeout")
			return res
		}

		res.Deleted++
		res.AuditEntries = append(res.AuditEntries, entry)
	}
	return res
}

func isDefinitionKind(k trace.NodeKind) bool {
	switch k {
	case trace.NodeCompositionRevision, trace.NodeComposition, trace.NodeXRD:
		return true
	case trace.NodeXR, trace.NodeComposed, trace.NodeMR:
		return false
	default:
		return false
	}
}

func skipRemaining(res *ExecuteResult, plan DeletePlan, policy string, after DeleteStep, reason string) {
	seen := false
	for _, step := range plan.Steps {
		if !seen {
			if step.Ref.Key() == after.Ref.Key() && step.Kind == after.Kind {
				seen = true
			}
			continue
		}
		res.Skipped++
		res.AuditEntries = append(res.AuditEntries, audit.Entry{
			Policy:  policy,
			TraceID: plan.TraceID,
			Action:  "skip",
			DryRun:  false,
			Object: audit.ObjectRef{
				APIVersion: step.Ref.Group + "/" + step.Ref.Version,
				Kind:       step.Ref.Kind,
				Namespace:  step.Ref.Namespace,
				Name:       step.Ref.Name,
				UID:        string(step.Ref.UID),
			},
			Reason: reason,
		})
	}
}
