package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
)

// RunMeta is policy metadata for a single AroundRun invocation.
type RunMeta struct {
	Policy string
	DryRun bool
	Scope  string
	RunID  string // optional; generated when empty
}

// AroundRun notifies started / finished / failed around fn without altering fn's result.
// onNotify receives each delivery attempt: err==nil means ok; other errors are
// delivery failures. Intentional skips (ErrSkipped) do not invoke onNotify.
// The cleanup error from fn is always returned as the second return value.
func AroundRun(
	ctx context.Context,
	n Notifier,
	meta RunMeta,
	fn func(context.Context) (cleanup.RunResult, error),
	onNotify func(kind EventKind, err error),
) (cleanup.RunResult, error) {
	if n == nil {
		n = Noop{}
	}
	if onNotify == nil {
		onNotify = func(EventKind, error) {}
	}
	runID := meta.RunID
	if runID == "" {
		runID = uuid.NewString()
	}
	startedAt := time.Now().UTC()

	startEv := Event{
		Kind:      KindStarted,
		RunID:     runID,
		Policy:    meta.Policy,
		DryRun:    meta.DryRun,
		Scope:     meta.Scope,
		StartedAt: startedAt,
	}
	reportNotify(onNotify, KindStarted, safeNotify(ctx, n, startEv))

	result, runErr := fn(ctx)
	finishedAt := time.Now().UTC()

	if runErr != nil {
		failEv := EventFromResult(KindFailed, meta, runID, startedAt, finishedAt, result, runErr)
		reportNotify(onNotify, KindFailed, safeNotify(ctx, n, failEv))
		return result, runErr
	}

	finEv := EventFromResult(KindFinished, meta, runID, startedAt, finishedAt, result, nil)
	reportNotify(onNotify, KindFinished, safeNotify(ctx, n, finEv))
	return result, nil
}

func reportNotify(onNotify func(EventKind, error), kind EventKind, err error) {
	if errors.Is(err, ErrSkipped) {
		return
	}
	if err != nil {
		onNotify(kind, fmt.Errorf("notify %s: %w", kind, err))
		return
	}
	onNotify(kind, nil)
}

// EventFromResult builds an Event from a RunResult (and optional hard error).
func EventFromResult(
	kind EventKind,
	meta RunMeta,
	runID string,
	startedAt, finishedAt time.Time,
	result cleanup.RunResult,
	runErr error,
) Event {
	ev := Event{
		Kind:       kind,
		RunID:      runID,
		Policy:     meta.Policy,
		DryRun:     meta.DryRun,
		Scope:      meta.Scope,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Traces:     len(result.Traces),
		Err:        runErr,
	}
	var errs []string
	for _, p := range result.Plans {
		if len(p.BlockedBy) > 0 {
			ev.Blocked += len(p.BlockedBy)
		}
	}
	for _, er := range result.Results {
		ev.Planned += er.Planned
		ev.Deleted += er.Deleted
		ev.Skipped += er.Skipped
		errs = append(errs, er.Errors...)
	}
	if runErr != nil {
		errs = append(errs, runErr.Error())
	}
	ev.Errors = TruncateErrors(errs, 5, 500)
	ev.ErrorCount = len(errs)
	if runErr != nil && ev.ErrorCount == 0 {
		ev.ErrorCount = 1
	}
	objs := objectsFromResult(kind, meta.DryRun, result)
	ev.ObjectsTotal = len(objs)
	ev.Objects = TruncateObjects(objs, MaxObjectsInEvent)
	return ev
}

// objectsFromResult collects delete targets with planner reasons.
// Live finished → successfully deleted audit entries; dry-run / failed → planned
// steps (or dry-run-delete / attempted delete audit when plans are empty).
func objectsFromResult(kind EventKind, dryRun bool, result cleanup.RunResult) []ObjectSummary {
	if kind == KindStarted {
		return nil
	}

	liveFinished := kind == KindFinished && !dryRun
	if liveFinished {
		return objectsFromAudit(result, "delete")
	}

	if objs := objectsFromPlans(result); len(objs) > 0 {
		return objs
	}
	// Fallback when plans were not retained but execute wrote audit lines.
	if dryRun || kind == KindFailed {
		objs := objectsFromAudit(result, "dry-run-delete")
		if kind == KindFailed {
			objs = append(objs, objectsFromAudit(result, "delete")...)
		}
		return objs
	}
	return nil
}

func objectsFromPlans(result cleanup.RunResult) []ObjectSummary {
	var out []ObjectSummary
	for _, p := range result.Plans {
		for _, step := range p.Steps {
			out = append(out, ObjectSummary{
				APIVersion: step.Ref.Group + "/" + step.Ref.Version,
				Kind:       step.Ref.Kind,
				Namespace:  step.Ref.Namespace,
				Name:       step.Ref.Name,
				Reason:     step.Reason,
			})
		}
	}
	return out
}

func objectsFromAudit(result cleanup.RunResult, action string) []ObjectSummary {
	var out []ObjectSummary
	for _, er := range result.Results {
		for _, e := range er.AuditEntries {
			if e.Action != action {
				continue
			}
			out = append(out, ObjectSummary{
				APIVersion: e.Object.APIVersion,
				Kind:       e.Object.Kind,
				Namespace:  e.Object.Namespace,
				Name:       e.Object.Name,
				Reason:     e.Reason,
			})
		}
	}
	return out
}
