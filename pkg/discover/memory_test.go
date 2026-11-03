package discover_test

import (
	"context"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
)

func TestMemoryBackendSeedGetDelete(t *testing.T) {
	t.Parallel()
	cm := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "cm", nil, map[string]interface{}{})
	mem := &discover.MemoryBackend{}
	mem.Seed(cm)

	ref := trace.ObjectRef{Group: "", Version: "v1", Kind: "ConfigMap", Namespace: "xp-test-1", Name: "cm"}
	got, err := mem.Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetName() != "cm" {
		t.Fatalf("got %s", got.GetName())
	}
	if err := mem.Delete(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := mem.Get(context.Background(), ref); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestMemoryBackendDiscoverRootsFilters(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	ok := sampleXR("xp-test-1", "ok", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	kept := sampleXR("xp-test-1", "kept", map[string]string{match.LabelEligible: "true", match.LabelKeep: "true"}, nil, now.Add(-72*time.Hour))

	mem := &discover.MemoryBackend{
		Roots: []discover.RootCandidate{
			{XR: ok, XRD: info},
			{XR: kept, XRD: info},
		},
	}
	roots, err := mem.DiscoverRoots(context.Background(), eligibleSelectors(), match.TTLConfig{RespectResourceTTL: false}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].XR.GetName() != "ok" {
		t.Fatalf("want only ok root, got %#v", roots)
	}
}

func TestMemoryBackendBuildTrace(t *testing.T) {
	t.Parallel()
	xr := sampleXR("xp-test-1", "test-db", map[string]string{match.LabelEligible: "true"}, nil, time.Now().UTC().Add(-72*time.Hour))
	cm := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "test-db-cm", nil, map[string]interface{}{})
	mem := &discover.MemoryBackend{}
	mem.Seed(xr, cm)

	tr, err := mem.BuildTrace(context.Background(), xr)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Root.Name != "test-db" {
		t.Fatalf("root=%s", tr.Root.Name)
	}
	if len(tr.Nodes) < 2 {
		t.Fatalf("expected XR + ConfigMap nodes, got %#v", tr.Nodes)
	}
}

func TestObjectKeyAndMustUnstructured(t *testing.T) {
	t.Parallel()
	if k := discover.ObjectKey("g", "v", "K", "", "n"); k != "g/v/K/n" {
		t.Fatalf("cluster key: %s", k)
	}
	if k := discover.ObjectKey("g", "v", "K", "ns", "n"); k != "g/v/K/ns/n" {
		t.Fatalf("namespaced key: %s", k)
	}
	u := discover.MustUnstructured("apps/v1", "Deployment", "ns", "web", map[string]string{"a": "b"}, nil)
	if discover.KeyFor(u) != "apps/v1/Deployment/ns/web" {
		t.Fatalf("KeyFor=%s", discover.KeyFor(u))
	}
}
