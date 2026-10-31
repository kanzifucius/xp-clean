package crossplane_test

import (
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResourceRefsV2Path(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"crossplane": map[string]interface{}{
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       "cm",
						"namespace":  "ns",
					},
				},
			},
		},
	}}
	refs, err := crossplane.ResourceRefs(u)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Name != "cm" {
		t.Fatalf("got %#v", refs)
	}
}

func TestXRServeFromXRD(t *testing.T) {
	xrd := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "xdbs.dev.example.org"},
		"spec": map[string]interface{}{
			"group": "db.dev.example.org",
			"names": map[string]interface{}{
				"kind":   "XDB",
				"plural": "xdbs",
			},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	}}
	gvk, err := crossplane.XRServeFromXRD(xrd)
	if err != nil {
		t.Fatal(err)
	}
	if gvk.Group != "db.dev.example.org" || gvk.Kind != "XDB" || gvk.Version != "v1alpha1" {
		t.Fatalf("got %#v", gvk)
	}
	if crossplane.IsLegacyClusterScope(xrd) {
		t.Fatal("should not be LegacyCluster")
	}
}
