package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MemoryBackend is an in-memory ClusterBackend for unit tests and offline dry-runs.
type MemoryBackend struct {
	Objects map[string]*unstructured.Unstructured // key: group/version/kind/ns/name
	Roots   []RootCandidate
}

// KeyFor builds a map key for an unstructured object.
func KeyFor(u *unstructured.Unstructured) string {
	gv := u.GroupVersionKind()
	return ObjectKey(gv.Group, gv.Version, gv.Kind, u.GetNamespace(), u.GetName())
}

// ObjectKey formats a stable object key.
func ObjectKey(group, version, kind, ns, name string) string {
	if ns == "" {
		return group + "/" + version + "/" + kind + "/" + name
	}
	return group + "/" + version + "/" + kind + "/" + ns + "/" + name
}

// Get implements trace.ObjectFetcher and cleanup.ObjectGetter.
func (m *MemoryBackend) Get(_ context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	k := ObjectKey(ref.Group, ref.Version, ref.Kind, ref.Namespace, ref.Name)
	u, ok := m.Objects[k]
	if !ok {
		gr := schema.GroupResource{Group: ref.Group, Resource: ref.Kind}
		return nil, apierrors.NewNotFound(gr, ref.Name)
	}
	return u.DeepCopy(), nil
}

// DiscoverRoots returns pre-seeded roots (optionally filtered).
func (m *MemoryBackend) DiscoverRoots(_ context.Context, sel match.Selectors, ttl match.TTLConfig, now time.Time) ([]RootCandidate, error) {
	if m.Roots != nil {
		out := make([]RootCandidate, 0, len(m.Roots))
		for _, r := range m.Roots {
			c, err := EvaluateXR(r.XR, r.XRD, sel, ttl, now)
			if err != nil {
				return nil, err
			}
			if c.Eligible {
				out = append(out, c)
			}
		}
		return HighestParents(out), nil
	}
	return nil, nil
}

// BuildTrace walks resourceRefs using the in-memory object map.
func (m *MemoryBackend) BuildTrace(ctx context.Context, root *unstructured.Unstructured) (trace.Trace, error) {
	return trace.BuildFromXR(ctx, root, m, trace.BuildOptions{})
}

// Delete removes an object from the map.
func (m *MemoryBackend) Delete(_ context.Context, ref trace.ObjectRef) error {
	k := ObjectKey(ref.Group, ref.Version, ref.Kind, ref.Namespace, ref.Name)
	if _, ok := m.Objects[k]; !ok {
		gr := schema.GroupResource{Group: ref.Group, Resource: ref.Kind}
		return apierrors.NewNotFound(gr, ref.Name)
	}
	delete(m.Objects, k)
	return nil
}

// CountXRs implements cleanup.XRCounter.
func (m *MemoryBackend) CountXRs(_ context.Context, info XRDInfo) (int, error) {
	if info.XRGVK.Empty() {
		return 0, fmt.Errorf("XRD info missing XR GVK")
	}
	n := 0
	for _, u := range m.Objects {
		gvk := u.GroupVersionKind()
		if gvk.Group == info.XRGVK.Group && gvk.Version == info.XRGVK.Version && gvk.Kind == info.XRGVK.Kind {
			n++
		}
	}
	return n, nil
}

// AttachDefinitions implements cleanup.DefinitionAttacher using the in-memory map.
func (m *MemoryBackend) AttachDefinitions(ctx context.Context, tr *trace.Trace, xrd, xr *unstructured.Unstructured) {
	if tr == nil {
		return
	}
	if xrd != nil {
		ref := trace.RefFromUnstructured(xrd)
		if !hasNode(*tr, ref) {
			tr.Nodes = append(tr.Nodes, trace.Node{Ref: ref, Kind: trace.NodeXRD, Labels: xrd.GetLabels(), Eligible: true})
			tr.Edges = append(tr.Edges, trace.Edge{From: ref, To: tr.Root, Reason: "xrdMatch"})
		}
	}
	if xr == nil {
		return
	}
	compName := crossplane.CompositionRefName(xr)
	if compName == "" {
		return
	}
	compRef := trace.ObjectRef{
		Group:   crossplane.XRDGroup,
		Version: "v1",
		Kind:    "Composition",
		Name:    compName,
	}
	comp, err := m.Get(ctx, compRef)
	if err != nil {
		tr.Warnings = append(tr.Warnings, fmt.Sprintf("composition %s: %v", compName, err))
		return
	}
	compRef = trace.RefFromUnstructured(comp)
	if !hasNode(*tr, compRef) {
		tr.Nodes = append(tr.Nodes, trace.Node{Ref: compRef, Kind: trace.NodeComposition, Labels: comp.GetLabels(), Eligible: true})
		if xrd != nil {
			tr.Edges = append(tr.Edges, trace.Edge{From: trace.RefFromUnstructured(xrd), To: compRef, Reason: "compositeTypeRef"})
		}
		tr.Edges = append(tr.Edges, trace.Edge{From: compRef, To: tr.Root, Reason: "compositionRef"})
	}

	revGK := schema.GroupKind{Group: crossplane.XRDGroup, Kind: "CompositionRevision"}
	for _, u := range m.Objects {
		if u.GroupVersionKind().GroupKind() != revGK {
			continue
		}
		labels := u.GetLabels()
		if labels[crossplane.LabelCompositionName] != compName {
			continue
		}
		revRef := trace.RefFromUnstructured(u)
		if hasNode(*tr, revRef) {
			continue
		}
		tr.Nodes = append(tr.Nodes, trace.Node{Ref: revRef, Kind: trace.NodeCompositionRevision, Labels: labels, Eligible: true})
		tr.Edges = append(tr.Edges, trace.Edge{From: compRef, To: revRef, Reason: "compositionRevision"})
	}
}

func hasNode(tr trace.Trace, ref trace.ObjectRef) bool {
	for _, n := range tr.Nodes {
		if n.Ref.Key() == ref.Key() {
			return true
		}
	}
	return false
}

// Seed stores objects for later Get/Delete.
func (m *MemoryBackend) Seed(objs ...*unstructured.Unstructured) {
	if m.Objects == nil {
		m.Objects = map[string]*unstructured.Unstructured{}
	}
	for _, u := range objs {
		m.Objects[KeyFor(u)] = u.DeepCopy()
	}
}

// MustUnstructured builds an unstructured object for tests.
func MustUnstructured(apiVersion, kind, ns, name string, labels map[string]string, spec map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
	}}
	if ns != "" {
		u.SetNamespace(ns)
	}
	if labels != nil {
		u.SetLabels(labels)
	}
	if spec != nil {
		_ = unstructured.SetNestedMap(u.Object, spec, "spec")
	}
	gv, _ := schema.ParseGroupVersion(apiVersion)
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: kind})
	return u
}
