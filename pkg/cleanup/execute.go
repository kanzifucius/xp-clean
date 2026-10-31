package cleanup

import (
	"context"
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/trace"
)

// Deleter performs a live delete of one object. Dry-run never calls it.
type Deleter interface {
	Delete(ctx context.Context, ref trace.ObjectRef) error
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
func Execute(ctx context.Context, plan DeletePlan, policy string, del Deleter) ExecuteResult {
	res := ExecuteResult{
		TraceID: plan.TraceID,
		DryRun:  plan.DryRun,
		Planned: len(plan.Steps),
	}

	if len(plan.BlockedBy) > 0 {
		res.Skipped = len(plan.Steps)
		for _, b := range plan.BlockedBy {
			res.Errors = append(res.Errors, "blocked: "+b)
		}
		return res
	}

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
		res.Deleted++
		res.AuditEntries = append(res.AuditEntries, entry)
	}
	return res
}
