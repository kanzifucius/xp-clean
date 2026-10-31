package cleanup_test

import (
	"context"
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/apimachinery/pkg/types"
)

func sampleTrace() trace.Trace {
	root := trace.ObjectRef{
		Group: "db.dev.example.org", Version: "v1alpha1", Kind: "XDB",
		Namespace: "xp-test-1", Name: "test-db", UID: types.UID("root-uid"),
	}
	child := trace.ObjectRef{
		Group: "s3.aws.m.upbound.io", Version: "v1beta1", Kind: "Bucket",
		Namespace: "xp-test-1", Name: "test-db-bucket",
	}
	xrd := trace.ObjectRef{Group: "apiextensions.crossplane.io", Version: "v2", Kind: "CompositeResourceDefinition", Name: "xdbs.dev.example.org"}
	comp := trace.ObjectRef{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "Composition", Name: "xdb-comp"}
	rev := trace.ObjectRef{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "CompositionRevision", Name: "xdb-comp-abc"}

	return trace.Trace{
		ID:   "xr/" + root.Key(),
		Root: root,
		Nodes: []trace.Node{
			{Ref: root, Kind: trace.NodeXR, Eligible: true},
			{Ref: child, Kind: trace.NodeMR, Eligible: true},
			{Ref: xrd, Kind: trace.NodeXRD, Eligible: true},
			{Ref: comp, Kind: trace.NodeComposition, Eligible: true},
			{Ref: rev, Kind: trace.NodeCompositionRevision, Eligible: true},
		},
		Edges: []trace.Edge{
			{From: root, To: child, Reason: "resourceRef"},
			{From: xrd, To: root, Reason: "xrdMatch"},
			{From: xrd, To: comp, Reason: "compositeTypeRef"},
			{From: comp, To: rev, Reason: "compositionRevision"},
		},
	}
}

func TestBuildPlanInstancesOnly(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	plan := cleanup.BuildPlan(tr, cfg, true)
	if len(plan.BlockedBy) > 0 {
		t.Fatalf("unexpected block: %v", plan.BlockedBy)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("want 1 step (XR only), got %#v", plan.Steps)
	}
	if plan.Steps[0].Kind != trace.NodeXR {
		t.Fatalf("want XR delete, got %s", plan.Steps[0].Kind)
	}
	if !plan.DryRun {
		t.Fatal("expected dry-run plan")
	}
}

func TestBuildPlanScopeAllOrder(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	cfg.Scope = cleanup.ScopeAll
	cfg.AllowXRDDeletion = true
	plan := cleanup.BuildPlan(tr, cfg, true)
	if len(plan.BlockedBy) > 0 {
		t.Fatalf("unexpected block: %v", plan.BlockedBy)
	}
	// XR → CompositionRevision → Composition → XRD
	want := []trace.NodeKind{trace.NodeXR, trace.NodeCompositionRevision, trace.NodeComposition, trace.NodeXRD}
	if len(plan.Steps) != len(want) {
		t.Fatalf("want %d steps, got %#v", len(want), plan.Steps)
	}
	for i, k := range want {
		if plan.Steps[i].Kind != k {
			t.Fatalf("step %d: want %s got %s", i, k, plan.Steps[i].Kind)
		}
	}
}

func TestBuildPlanScopeAllBlocksXRDWithoutGate(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	cfg.Scope = cleanup.ScopeAll
	cfg.AllowXRDDeletion = false
	plan := cleanup.BuildPlan(tr, cfg, true)
	// XR + rev + composition should still be present; XRD blocked.
	var kinds []trace.NodeKind
	for _, s := range plan.Steps {
		kinds = append(kinds, s.Kind)
	}
	for _, k := range kinds {
		if k == trace.NodeXRD {
			t.Fatalf("XRD should not be planned when allowXRDDeletion=false: %#v", plan)
		}
	}
	if len(plan.BlockedBy) == 0 {
		t.Fatal("expected XRD gate block message")
	}
}

func TestKeepBlocksPlan(t *testing.T) {
	tr := sampleTrace()
	tr.Nodes[0].Keep = true
	plan := cleanup.BuildPlan(tr, cleanup.DefaultDeletionConfig(), true)
	if len(plan.Steps) != 0 {
		t.Fatalf("expected no steps, got %#v", plan.Steps)
	}
	if len(plan.BlockedBy) == 0 {
		t.Fatal("expected blockedBy")
	}
}

func TestDryRunExecuteDoesNotDelete(t *testing.T) {
	tr := sampleTrace()
	plan := cleanup.BuildPlan(tr, cleanup.DefaultDeletionConfig(), true)
	del := &countingDeleter{}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del)
	if del.n != 0 {
		t.Fatalf("dry-run deleted %d objects", del.n)
	}
	if res.Deleted != 0 || res.Skipped != 1 {
		t.Fatalf("unexpected result %#v", res)
	}
	if len(res.AuditEntries) != 1 || !res.AuditEntries[0].DryRun {
		t.Fatalf("expected dry-run audit entry, got %#v", res.AuditEntries)
	}
}

func TestExecuteDeletesWhenNotDryRun(t *testing.T) {
	tr := sampleTrace()
	plan := cleanup.BuildPlan(tr, cleanup.DefaultDeletionConfig(), false)
	del := &countingDeleter{}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del)
	if del.n != 1 {
		t.Fatalf("want 1 delete, got %d", del.n)
	}
	if res.Deleted != 1 {
		t.Fatalf("unexpected %#v", res)
	}
}

type countingDeleter struct{ n int }

func (c *countingDeleter) Delete(context.Context, trace.ObjectRef) error {
	c.n++
	return nil
}

func TestParseScope(t *testing.T) {
	s, err := cleanup.ParseScope("")
	if err != nil || s != cleanup.ScopeInstancesOnly {
		t.Fatalf("got %q %v", s, err)
	}
	_, err = cleanup.ParseScope("claims")
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}
