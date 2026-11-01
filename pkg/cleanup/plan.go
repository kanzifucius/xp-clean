package cleanup

import (
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/trace"
)

// DeleteStep is one ordered action in a DeletePlan.
type DeleteStep struct {
	Ref    trace.ObjectRef `json:"ref"`
	Kind   trace.NodeKind  `json:"kind"`
	Action string          `json:"action"` // delete
	Reason string          `json:"reason"`
}

// DeletePlan is the ordered set of deletes for one trace.
type DeletePlan struct {
	TraceID   string       `json:"traceId"`
	Steps     []DeleteStep `json:"steps"`
	BlockedBy []string     `json:"blockedBy,omitempty"`
	DryRun    bool         `json:"dryRun"`
}

// BuildPlan creates an ordered delete plan from a trace and deletion scope.
//
// Instance layer: delete the XR root only (children follow via ownerReferences /
// Crossplane finalizers). Definition layer (scope=all): after instances, delete
// CompositionRevisions → Composition → XRD (XRD only if allowXRDDeletion).
//
// remainingXRsAfterRun is the count of XRs for this XRD expected to remain after
// this cleanup pass's instance deletes. When > 0, definition steps are omitted
// (zero-XR gate). Pass 0 when unknown / single-XR dry-run fixtures.
func BuildPlan(t trace.Trace, cfg DeletionConfig, dryRun bool) DeletePlan {
	return BuildPlanWithRemaining(t, cfg, dryRun, 0)
}

// BuildPlanWithRemaining is BuildPlan with an explicit zero-XR remaining count.
func BuildPlanWithRemaining(t trace.Trace, cfg DeletionConfig, dryRun bool, remainingXRsAfterRun int) DeletePlan {
	plan := DeletePlan{
		TraceID: t.ID,
		DryRun:  dryRun,
	}

	gates := CheckGates(t, cfg, dryRun)
	if !gates.OK {
		plan.BlockedBy = gates.BlockedBy
		return plan
	}

	// Instance layer: XR root delete is the primary action.
	rootNode, ok := findNode(t, t.Root)
	if !ok || rootNode.Kind != trace.NodeXR {
		plan.BlockedBy = append(plan.BlockedBy, "root is not an XR instance")
		return plan
	}

	plan.Steps = append(plan.Steps, DeleteStep{
		Ref:    rootNode.Ref,
		Kind:   rootNode.Kind,
		Action: "delete",
		Reason: "instance_root",
	})

	if cfg.Scope != ScopeAll {
		return plan
	}

	if remainingXRsAfterRun > 0 {
		// Omit definition steps so instance deletes still run; do not mark the
		// whole plan blocked (operators with --fail-on-blocked still clean XRs).
		return plan
	}

	// Definition layer order: revisions → composition → XRD.
	for _, n := range orderedDefinitions(t) {
		if err := ValidateDefinitionStep(n.Kind, cfg); err != nil {
			plan.BlockedBy = append(plan.BlockedBy, err.Error())
			continue
		}
		plan.Steps = append(plan.Steps, DeleteStep{
			Ref:    n.Ref,
			Kind:   n.Kind,
			Action: "delete",
			Reason: fmt.Sprintf("definition_scope_%s", cfg.Scope),
		})
	}

	return plan
}

func findNode(t trace.Trace, ref trace.ObjectRef) (trace.Node, bool) {
	for _, n := range t.Nodes {
		if n.Ref.Key() == ref.Key() {
			return n, true
		}
	}
	return trace.Node{}, false
}

// orderedDefinitions returns definition nodes in safe delete order.
func orderedDefinitions(t trace.Trace) []trace.Node {
	var revs, comps, xrds []trace.Node
	for _, n := range t.Nodes {
		switch n.Kind {
		case trace.NodeCompositionRevision:
			revs = append(revs, n)
		case trace.NodeComposition:
			comps = append(comps, n)
		case trace.NodeXRD:
			xrds = append(xrds, n)
		}
	}
	out := make([]trace.Node, 0, len(revs)+len(comps)+len(xrds))
	out = append(out, revs...)
	out = append(out, comps...)
	out = append(out, xrds...)
	return out
}

// ApplyBudgets truncates plans to honour maxTracesPerRun / maxDeletesPerRun.
func ApplyBudgets(plans []DeletePlan, cfg DeletionConfig) []DeletePlan {
	maxTraces := cfg.MaxTracesPerRun
	if maxTraces <= 0 {
		maxTraces = len(plans)
	}
	maxDeletes := cfg.MaxDeletesPerRun
	if maxDeletes <= 0 {
		maxDeletes = int(^uint(0) >> 1)
	}

	out := make([]DeletePlan, 0, len(plans))
	deletes := 0
	for i, p := range plans {
		if i >= maxTraces {
			break
		}
		if len(p.BlockedBy) > 0 || len(p.Steps) == 0 {
			out = append(out, p)
			continue
		}
		remaining := maxDeletes - deletes
		if remaining <= 0 {
			p.BlockedBy = append(p.BlockedBy, "maxDeletesPerRun budget exhausted")
			p.Steps = nil
			out = append(out, p)
			continue
		}
		if len(p.Steps) > remaining {
			p.Steps = p.Steps[:remaining]
			p.BlockedBy = append(p.BlockedBy, "truncated by maxDeletesPerRun")
		}
		deletes += len(p.Steps)
		out = append(out, p)
	}
	return out
}
