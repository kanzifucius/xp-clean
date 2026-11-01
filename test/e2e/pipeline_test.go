package e2e_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Pipeline scenarios exercise discover → plan → execute with XR→ConfigMap
// fixtures (no cloud providers). They run under ordinary `go test ./...`.
// Cluster-backed kind runs live under `make e2e` (see test/e2e/README.md).

func seedTree(t *testing.T, labels map[string]string, ann map[string]string) (*discover.MemoryBackend, discover.XRDInfo, *unstructured.Unstructured) {
	t.Helper()
	xrd := discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", "xdbs.db.dev.example.org",
		map[string]string{match.LabelEligible: "true"},
		map[string]interface{}{
			"group": "db.dev.example.org",
			"scope": "Namespaced",
			"names": map[string]interface{}{"kind": "XDB", "plural": "xdbs"},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	)
	xr := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "test-db", labels,
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"compositionRef": map[string]interface{}{"name": "xdb-comp"},
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "test-db-cm",
						"namespace":  "xp-test-1",
					},
					map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "test-db-deploy",
						"namespace":  "xp-test-1",
					},
				},
			},
		},
	)
	if ann != nil {
		xr.SetAnnotations(ann)
	}
	xr.SetCreationTimestamp(metav1.NewTime(time.Now().UTC().Add(-72 * time.Hour)))
	cm := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "test-db-cm", nil, map[string]interface{}{})
	deploy := discover.MustUnstructured("apps/v1", "Deployment", "xp-test-1", "test-db-deploy", nil, map[string]interface{}{})

	info, err := discover.InspectXRD(xrd, match.Selectors{RequireLabels: map[string]string{match.LabelEligible: "true"}})
	if err != nil {
		t.Fatal(err)
	}
	mem := &discover.MemoryBackend{Roots: []discover.RootCandidate{{XR: xr, XRD: info}}}
	mem.Seed(xrd, xr, cm, deploy)
	return mem, info, xr
}

func policy(dryRun bool) cleanup.PolicyView {
	return cleanup.PolicyView{
		Name:   "e2e-lab",
		DryRun: dryRun,
		Selectors: match.Selectors{
			RequireLabels: map[string]string{match.LabelEligible: "true"},
			Globs:         match.GlobPatterns{XRNames: []string{"test-*"}},
		},
		TTL:      match.TTLConfig{RespectResourceTTL: false},
		Deletion: cleanup.DefaultDeletionConfig(),
	}
}

func TestE2EDryRunPlanDoesNotDelete(t *testing.T) {
	mem, _, _ := seedTree(t, map[string]string{match.LabelEligible: "true"}, nil)
	runner := &cleanup.Runner{Backend: mem, Out: &bytes.Buffer{}, Now: func() time.Time { return time.Now().UTC() }}
	res, err := runner.RunOnce(context.Background(), policy(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Plans) != 1 || !res.Plans[0].DryRun || len(res.Plans[0].Steps) != 1 {
		t.Fatalf("unexpected plan %#v", res.Plans)
	}
	if res.Plans[0].Steps[0].Kind != trace.NodeXR {
		t.Fatalf("instances-only should plan XR only, got %s", res.Plans[0].Steps[0].Kind)
	}
	ref := res.Plans[0].Steps[0].Ref
	if _, err := mem.Get(context.Background(), ref); err != nil {
		t.Fatalf("dry-run must leave XR: %v", err)
	}
	if _, err := mem.Get(context.Background(), trace.ObjectRef{Version: "v1", Kind: "ConfigMap", Namespace: "xp-test-1", Name: "test-db-cm"}); err != nil {
		t.Fatalf("composed ConfigMap must remain: %v", err)
	}
}

func TestE2ELiveInstanceDelete(t *testing.T) {
	mem, _, _ := seedTree(t, map[string]string{match.LabelEligible: "true"}, nil)
	runner := &cleanup.Runner{Backend: mem, Out: &bytes.Buffer{}, Now: func() time.Time { return time.Now().UTC() }}
	res, err := runner.RunOnce(context.Background(), policy(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 || res.Results[0].Deleted != 1 {
		t.Fatalf("want 1 delete, got %#v", res.Results)
	}
	xrRef := trace.ObjectRef{Group: "db.dev.example.org", Version: "v1alpha1", Kind: "XDB", Namespace: "xp-test-1", Name: "test-db"}
	if _, err := mem.Get(context.Background(), xrRef); err == nil {
		t.Fatal("XR should be deleted after live run")
	}
	// instances-only: composed children are left for Crossplane GC; MemoryBackend does not GC.
	if _, err := mem.Get(context.Background(), trace.ObjectRef{Version: "v1", Kind: "ConfigMap", Namespace: "xp-test-1", Name: "test-db-cm"}); err != nil {
		t.Fatalf("ConfigMap should remain under instances-only: %v", err)
	}
}

func TestE2EKeepBlocksPlan(t *testing.T) {
	mem, _, _ := seedTree(t, map[string]string{
		match.LabelEligible: "true",
		match.LabelKeep:     "true",
	}, nil)
	// Keep roots are not returned as Eligible by MemoryBackend.DiscoverRoots.
	// Seed an eligible-looking root that EvaluateXR marks keep.
	res, err := (&cleanup.Runner{Backend: mem, Out: &bytes.Buffer{}}).RunOnce(context.Background(), policy(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Traces) != 0 || len(res.Plans) != 0 {
		t.Fatalf("keep XR must not produce plans, got traces=%d plans=%d", len(res.Traces), len(res.Plans))
	}
}

func TestE2EPausedBlocksPlan(t *testing.T) {
	mem, _, xr := seedTree(t, map[string]string{match.LabelEligible: "true"}, map[string]string{match.AnnotPaused: "true"})
	// Force root candidate paused flags the way DiscoverRoots/EvaluateXR would.
	mem.Roots[0].XR = xr
	runner := &cleanup.Runner{Backend: mem, Out: &bytes.Buffer{}, Now: func() time.Time { return time.Now().UTC() }}
	res, err := runner.RunOnce(context.Background(), policy(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Plans) != 1 {
		t.Fatalf("want 1 blocked plan, got %#v", res.Plans)
	}
	if len(res.Plans[0].BlockedBy) == 0 || len(res.Plans[0].Steps) != 0 {
		t.Fatalf("paused should block with no steps: %#v", res.Plans[0])
	}
	if !cleanup.HasBlockedPlans(res.Plans) {
		t.Fatal("expected HasBlockedPlans")
	}
}

func TestE2EFailOnBlockedExitCode(t *testing.T) {
	plans := []cleanup.DeletePlan{{
		TraceID:   "xr/paused",
		BlockedBy: []string{"crossplane.io/paused (deletePaused=false)"},
	}}
	if !cleanup.HasBlockedPlans(plans) {
		t.Fatal("expected blocked plans")
	}
	err := exit.BlockedErr(errString("completed with blocked plans (--fail-on-blocked)"))
	if got := exit.Status(err); got != exit.Blocked {
		t.Fatalf("want exit %d (--fail-on-blocked), got %d", exit.Blocked, got)
	}
	if exit.Blocked != 3 {
		t.Fatalf("exit.Blocked must remain 3, got %d", exit.Blocked)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
