package discover_test

import (
	"context"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestParseNamespacedName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		wantNS  string
		wantName string
		wantErr bool
	}{
		{in: "solo", wantName: "solo"},
		{in: "xp-test-1/test-db", wantNS: "xp-test-1", wantName: "test-db"},
		{in: "", wantErr: true},
		{in: "a/b/c", wantErr: true},
		{in: "  ns/name  ", wantNS: "ns", wantName: "name"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			ns, name, err := discover.ParseNamespacedName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ns != tt.wantNS || name != tt.wantName {
				t.Fatalf("got %q/%q want %q/%q", ns, name, tt.wantNS, tt.wantName)
			}
		})
	}
}

func TestAttachDefinitions(t *testing.T) {
	t.Parallel()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	comp := discover.MustUnstructured("apiextensions.crossplane.io/v1", "Composition", "", "xdb-comp",
		map[string]string{"crossplane.io/composition-name": "xdb-comp"},
		map[string]interface{}{
			"compositeTypeRef": map[string]interface{}{
				"apiVersion": "db.dev.example.org/v1alpha1",
				"kind":       "XDB",
			},
		},
	)
	rev := discover.MustUnstructured("apiextensions.crossplane.io/v1", "CompositionRevision", "", "xdb-comp-abc",
		map[string]string{"crossplane.io/composition-name": "xdb-comp"},
		nil,
	)
	xr := sampleXR("xp-test-1", "test-db", map[string]string{match.LabelEligible: "true"}, nil, time.Now().UTC().Add(-72*time.Hour))

	dyn := discoverClient(xrd, comp, rev, xr)
	tr := trace.Trace{
		ID:   "xr/test",
		Root: trace.RefFromUnstructured(xr),
		Nodes: []trace.Node{
			{Ref: trace.RefFromUnstructured(xr), Kind: trace.NodeXR, Eligible: true},
		},
	}
	discover.AttachDefinitions(context.Background(), dyn, &tr, xrd, xr)

	kinds := map[trace.NodeKind]int{}
	for _, n := range tr.Nodes {
		kinds[n.Kind]++
	}
	if kinds[trace.NodeXRD] != 1 || kinds[trace.NodeComposition] != 1 || kinds[trace.NodeCompositionRevision] != 1 {
		t.Fatalf("unexpected nodes: %#v warnings=%v", tr.Nodes, tr.Warnings)
	}
	if len(tr.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", tr.Warnings)
	}
}

func TestAttachDefinitionsMissingCompositionWarns(t *testing.T) {
	t.Parallel()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	xr := sampleXR("xp-test-1", "test-db", map[string]string{match.LabelEligible: "true"}, nil, time.Now().UTC())
	dyn := discoverClient(xrd, xr)
	tr := trace.Trace{
		Root:  trace.ObjectRef{Group: "db.dev.example.org", Version: "v1alpha1", Kind: "XDB", Namespace: "xp-test-1", Name: "test-db", UID: types.UID("u")},
		Nodes: []trace.Node{{Kind: trace.NodeXR}},
	}
	discover.AttachDefinitions(context.Background(), dyn, &tr, xrd, xr)
	if len(tr.Warnings) == 0 {
		t.Fatal("expected composition warning")
	}
}

func TestAttachDefinitionsNilDynStillAddsXRD(t *testing.T) {
	t.Parallel()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	xr := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "db.dev.example.org/v1alpha1",
		"kind":       "XDB",
		"metadata":   map[string]interface{}{"name": "x", "namespace": "ns"},
	}}
	tr := trace.Trace{Root: trace.RefFromUnstructured(xr)}
	discover.AttachDefinitions(context.Background(), nil, &tr, xrd, xr)
	if len(tr.Nodes) != 1 || tr.Nodes[0].Kind != trace.NodeXRD {
		t.Fatalf("want XRD node only, got %#v", tr.Nodes)
	}
}
