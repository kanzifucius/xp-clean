package trace_test

import (
	"context"
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildFromXRWalksResourceRefs(t *testing.T) {
	root := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "test-db",
		map[string]string{"xp-clean.io/eligible": "true"},
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "test-db-cm",
						"namespace":  "xp-test-1",
					},
					map[string]interface{}{
						"apiVersion": "db.dev.example.org/v1alpha1",
						"kind":       "XDB",
						"name":       "nested",
						"namespace":  "xp-test-1",
					},
				},
			},
		},
	)
	child := discover.MustUnstructured("v1", "ConfigMap", "xp-test-1", "test-db-cm", nil, map[string]interface{}{"data": map[string]interface{}{"k": "v"}})
	nested := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", "xp-test-1", "nested", nil,
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "nested-dep",
						"namespace":  "xp-test-1",
					},
				},
			},
		},
	)
	dep := discover.MustUnstructured("apps/v1", "Deployment", "xp-test-1", "nested-dep", nil, map[string]interface{}{})

	mem := &discover.MemoryBackend{}
	mem.Seed(root, child, nested, dep)

	tr, err := trace.BuildFromXR(context.Background(), root, mem, trace.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Nodes) != 4 {
		t.Fatalf("want 4 nodes, got %d: %#v", len(tr.Nodes), tr.Nodes)
	}
	if len(tr.Edges) != 3 {
		t.Fatalf("want 3 edges, got %d", len(tr.Edges))
	}
	text := trace.RenderText(tr)
	if text == "" {
		t.Fatal("empty render")
	}
	t.Log(text)
}

func TestMissingResourceRefWarned(t *testing.T) {
	root := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "db.dev.example.org/v1alpha1",
		"kind":       "XDB",
		"metadata":   map[string]interface{}{"name": "solo", "namespace": "ns"},
		"spec": map[string]interface{}{
			"crossplane": map[string]interface{}{
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "missing",
						"namespace":  "ns",
					},
				},
			},
		},
	}}
	mem := &discover.MemoryBackend{Objects: map[string]*unstructured.Unstructured{}}
	tr, err := trace.BuildFromXR(context.Background(), root, mem, trace.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Warnings) == 0 {
		t.Fatal("expected missing ref warning")
	}
}
