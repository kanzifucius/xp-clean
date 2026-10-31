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
		TTL: match.TTLConfig{RespectResourceTTL: false},
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
