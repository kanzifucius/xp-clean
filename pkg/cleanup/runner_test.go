package cleanup_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRunnerDryRunMemoryBackend(t *testing.T) {
	xrdU := discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", "xdbs.dev.example.org",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"group": "db.dev.example.org",
			"scope": "Namespaced",
			"names": map[string]interface{}{"kind": "XDB", "plural": "xdbs"},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	)
	xr := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "test-db",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "cm",
						"namespace":  "xp-test-1",
					},
				},
			},
		},
	)
	xr.SetCreationTimestamp(xr.GetCreationTimestamp())
	_ = unstructured.SetNestedField(xr.Object, time.Now().UTC().Add(-72*time.Hour).Format(time.RFC3339), "metadata", "creationTimestamp")
	cm := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "cm", nil, map[string]interface{}{})

	mem := &discover.MemoryBackend{}
	mem.Seed(xrdU, xr, cm)
	info, err := discover.InspectXRD(xrdU, match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = crossplane.LabelComposite
	mem.Roots = []discover.RootCandidate{{XR: xr, XRD: info}}

	runner := &cleanup.Runner{Backend: mem, Out: io.Discard, Now: func() time.Time { return time.Now().UTC() }}
	res, err := runner.RunOnce(context.Background(), cleanup.PolicyView{
		Name:   "test",
		DryRun: true,
		Selectors: match.Selectors{
			RequireLabels: map[string]string{match.LabelEligible: "true"},
			Globs:         match.GlobPatterns{XRNames: []string{"test-*"}},
		},
		TTL:      match.TTLConfig{RespectResourceTTL: false},
		Deletion: cleanup.DefaultDeletionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Traces) != 1 || len(res.Plans) != 1 {
		t.Fatalf("got traces=%d plans=%d", len(res.Traces), len(res.Plans))
	}
	if !res.Plans[0].DryRun || len(res.Plans[0].Steps) != 1 {
		t.Fatalf("plan %#v", res.Plans[0])
	}
	// Object still present after dry-run.
	if _, err := mem.Get(context.Background(), res.Plans[0].Steps[0].Ref); err != nil {
		t.Fatalf("dry-run should not delete: %v", err)
	}
}

func TestRunnerScopeAllAttachesDefinitionsAndDeletes(t *testing.T) {
	xrdU := discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", "xdbs.dev.example.org",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"group": "db.dev.example.org",
			"scope": "Namespaced",
			"names": map[string]interface{}{"kind": "XDB", "plural": "xdbs"},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	)
	comp := discover.MustUnstructured("apiextensions.crossplane.io/v1", "Composition", "", "xdb-comp",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"compositeTypeRef": map[string]interface{}{
				"apiVersion": "db.dev.example.org/v1alpha1",
				"kind":       "XDB",
			},
		},
	)
	rev := discover.MustUnstructured("apiextensions.crossplane.io/v1", "CompositionRevision", "", "xdb-comp-abc",
		map[string]string{crossplane.LabelCompositionName: "xdb-comp"},
		nil,
	)
	xr := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "test-db",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"compositionRef": map[string]interface{}{"name": "xdb-comp"},
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "cm",
						"namespace":  "xp-test-1",
					},
				},
			},
		},
	)
	_ = unstructured.SetNestedField(xr.Object, time.Now().UTC().Add(-72*time.Hour).Format(time.RFC3339), "metadata", "creationTimestamp")
	cm := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "cm", nil, map[string]interface{}{})

	mem := &discover.MemoryBackend{}
	mem.Seed(xrdU, comp, rev, xr, cm)
	info, err := discover.InspectXRD(xrdU, match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mem.Roots = []discover.RootCandidate{{XR: xr, XRD: info}}

	delCfg := cleanup.DefaultDeletionConfig()
	delCfg.Scope = cleanup.ScopeAll
	delCfg.AllowXRDDeletion = true
	delCfg.WaitTimeout = "1s"

	runner := &cleanup.Runner{
		Backend:      mem,
		Out:          io.Discard,
		Now:          func() time.Time { return time.Now().UTC() },
		PollInterval: time.Millisecond,
	}
	res, err := runner.RunOnce(context.Background(), cleanup.PolicyView{
		Name:   "test",
		DryRun: false,
		Selectors: match.Selectors{
			RequireLabels: map[string]string{match.LabelEligible: "true"},
			Globs:         match.GlobPatterns{XRNames: []string{"test-*"}},
		},
		TTL:      match.TTLConfig{RespectResourceTTL: false},
		Deletion: delCfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Traces) != 1 {
		t.Fatalf("traces=%d", len(res.Traces))
	}
	kinds := map[string]bool{}
	for _, n := range res.Traces[0].Nodes {
		kinds[string(n.Kind)] = true
	}
	for _, want := range []string{"XR", "XRD", "Composition", "CompositionRevision"} {
		if !kinds[want] {
			t.Fatalf("trace missing %s nodes: %#v", want, kinds)
		}
	}
	if len(res.Plans) != 1 || len(res.Plans[0].BlockedBy) > 0 {
		t.Fatalf("plan %#v", res.Plans[0])
	}
	wantSteps := []string{"XR", "CompositionRevision", "Composition", "XRD"}
	if len(res.Plans[0].Steps) != len(wantSteps) {
		t.Fatalf("steps %#v", res.Plans[0].Steps)
	}
	for i, k := range wantSteps {
		if string(res.Plans[0].Steps[i].Kind) != k {
			t.Fatalf("step %d want %s got %s", i, k, res.Plans[0].Steps[i].Kind)
		}
	}
	if len(res.Results) != 1 || res.Results[0].Deleted != 4 || len(res.Results[0].Errors) != 0 {
		t.Fatalf("result %#v", res.Results[0])
	}
	// XR and definitions removed from memory backend.
	if _, err := mem.Get(context.Background(), res.Plans[0].Steps[0].Ref); err == nil {
		t.Fatal("XR should be deleted")
	}
	if n, err := mem.CountXRs(context.Background(), info); err != nil || n != 0 {
		t.Fatalf("remaining XRs=%d err=%v", n, err)
	}
}

func TestRunnerScopeAllBlocksDefinitionsWhenOtherXRRemains(t *testing.T) {
	xrdU := discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", "xdbs.dev.example.org",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"group": "db.dev.example.org",
			"scope": "Namespaced",
			"names": map[string]interface{}{"kind": "XDB", "plural": "xdbs"},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	)
	comp := discover.MustUnstructured("apiextensions.crossplane.io/v1", "Composition", "", "xdb-comp", nil, nil)
	xr := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "test-db",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"compositionRef": map[string]interface{}{"name": "xdb-comp"},
			},
		},
	)
	other := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "prod", "keep-me",
		map[string]string{"env": "prod"},
		nil,
	)
	_ = unstructured.SetNestedField(xr.Object, time.Now().UTC().Add(-72*time.Hour).Format(time.RFC3339), "metadata", "creationTimestamp")

	mem := &discover.MemoryBackend{}
	mem.Seed(xrdU, comp, xr, other)
	info, err := discover.InspectXRD(xrdU, match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mem.Roots = []discover.RootCandidate{{XR: xr, XRD: info}}

	delCfg := cleanup.DefaultDeletionConfig()
	delCfg.Scope = cleanup.ScopeAll
	delCfg.AllowXRDDeletion = true
	delCfg.WaitTimeout = "1s"

	runner := &cleanup.Runner{Backend: mem, Out: io.Discard, PollInterval: time.Millisecond}
	res, err := runner.RunOnce(context.Background(), cleanup.PolicyView{
		Name:      "test",
		DryRun:    false,
		Selectors: match.Selectors{RequireLabels: map[string]string{match.LabelEligible: "true"}},
		TTL:       match.TTLConfig{RespectResourceTTL: false},
		Deletion:  delCfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Plans) != 1 {
		t.Fatalf("plans=%d", len(res.Plans))
	}
	if len(res.Plans[0].Steps) != 1 || res.Plans[0].Steps[0].Kind != "XR" {
		t.Fatalf("expected XR-only plan when other XR remains, got %#v", res.Plans[0])
	}
	if len(res.Plans[0].BlockedBy) != 0 {
		t.Fatalf("instance plan should not be blocked: %#v", res.Plans[0].BlockedBy)
	}
	if res.Results[0].Deleted != 1 {
		t.Fatalf("want XR deleted, got %#v", res.Results[0])
	}
	// Composition must remain (prod XR still present).
	if _, err := mem.Get(context.Background(), trace.RefFromUnstructured(comp)); err != nil {
		t.Fatalf("composition should remain: %v", err)
	}
	if n, _ := mem.CountXRs(context.Background(), info); n != 1 {
		t.Fatalf("want 1 remaining XR (prod), got %d", n)
	}
}

func TestHasBlockedPlans(t *testing.T) {
	if cleanup.HasBlockedPlans(nil) {
		t.Fatal("nil plans should not be blocked")
	}
	if cleanup.HasBlockedPlans([]cleanup.DeletePlan{{TraceID: "ok", Steps: []cleanup.DeleteStep{{}}}}) {
		t.Fatal("unblocked plan")
	}
	if !cleanup.HasBlockedPlans([]cleanup.DeletePlan{{TraceID: "b", BlockedBy: []string{"keep"}}}) {
		t.Fatal("expected blocked")
	}
	merged := cleanup.MergeRunResults(
		cleanup.RunResult{Plans: []cleanup.DeletePlan{{TraceID: "a"}}},
		cleanup.RunResult{Plans: []cleanup.DeletePlan{{TraceID: "b", BlockedBy: []string{"x"}}}},
	)
	if !cleanup.HasBlockedPlans(merged.Plans) {
		t.Fatal("merged should report blocked")
	}
}
