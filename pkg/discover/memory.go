package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
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

// Get implements trace.ObjectFetcher.
func (m *MemoryBackend) Get(_ context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	k := ObjectKey(ref.Group, ref.Version, ref.Kind, ref.Namespace, ref.Name)
	u, ok := m.Objects[k]
	if !ok {
		return nil, fmt.Errorf("not found: %s", k)
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
		return fmt.Errorf("not found: %s", k)
	}
	delete(m.Objects, k)
	return nil
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
