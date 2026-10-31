package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ObjectFetcher loads an object by reference. Implementations may use a
// dynamic client or an in-memory fixture map.
type ObjectFetcher interface {
	Get(ctx context.Context, ref ObjectRef) (*unstructured.Unstructured, error)
}

// BuildOptions control how an instance tree is walked.
type BuildOptions struct {
	Now time.Time
}

// BuildFromXR walks resourceRefs from a root XR into a Trace.
// Claim edges are never followed (XR-only / v2-native).
func BuildFromXR(ctx context.Context, root *unstructured.Unstructured, fetch ObjectFetcher, opts BuildOptions) (Trace, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}

	rootRef := RefFromUnstructured(root)
	t := Trace{
		ID:      fmt.Sprintf("xr/%s", rootRef.Key()),
		Root:    rootRef,
		BuiltAt: opts.Now,
	}

	rootNode := nodeFromUnstructured(root, NodeXR)
	t.Nodes = append(t.Nodes, rootNode)

	type item struct {
		obj  *unstructured.Unstructured
		kind NodeKind
	}
	queue := []item{{obj: root, kind: NodeXR}}
	seen := map[string]struct{}{rootRef.Key(): {}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		parentRef := RefFromUnstructured(cur.obj)

		if cur.kind != NodeXR {
			continue
		}

		refs, err := crossplane.ResourceRefs(cur.obj)
		if err != nil {
			t.Warnings = append(t.Warnings, fmt.Sprintf("resourceRefs on %s: %v", parentRef.Key(), err))
			continue
		}

		for _, rr := range refs {
			childRef := ObjectRef{
				Group:     rr.APIVersionGroup(),
				Version:   rr.APIVersionVersion(),
				Kind:      rr.Kind,
				Namespace: rr.Namespace,
				Name:      rr.Name,
			}
			if childRef.Namespace == "" {
				childRef.Namespace = parentRef.Namespace
			}

			child, err := fetch.Get(ctx, childRef)
			if err != nil {
				t.Warnings = append(t.Warnings, fmt.Sprintf("missing resourceRef %s: %v", childRef.Key(), err))
				t.Nodes = append(t.Nodes, Node{
					Ref:        childRef,
					Kind:       NodeComposed,
					OrphanHint: "in resourceRefs but missing",
				})
				continue
			}

			childRef = RefFromUnstructured(child)
			if _, ok := seen[childRef.Key()]; ok {
				continue
			}
			seen[childRef.Key()] = struct{}{}

			kind := classifyComposed(child)
			n := nodeFromUnstructured(child, kind)
			t.Nodes = append(t.Nodes, n)
			t.Edges = append(t.Edges, Edge{From: parentRef, To: childRef, Reason: "resourceRef"})

			if kind == NodeXR {
				queue = append(queue, item{obj: child, kind: NodeXR})
			}
		}
	}

	return t, nil
}

// RefFromUnstructured builds an ObjectRef from an unstructured object.
func RefFromUnstructured(u *unstructured.Unstructured) ObjectRef {
	gv := u.GroupVersionKind()
	return ObjectRef{
		Group:     gv.Group,
		Version:   gv.Version,
		Kind:      gv.Kind,
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
		UID:       u.GetUID(),
	}
}

func nodeFromUnstructured(u *unstructured.Unstructured, kind NodeKind) Node {
	labels := u.GetLabels()
	ann := u.GetAnnotations()
	n := Node{
		Ref:      RefFromUnstructured(u),
		Kind:     kind,
		Labels:   labels,
		Keep:     match.IsKeep(labels),
		Paused:   match.IsPaused(labels, ann),
		Eligible: true,
	}
	return n
}

func classifyComposed(u *unstructured.Unstructured) NodeKind {
	// Nested XR: has spec.crossplane.resourceRefs
	if _, found, _ := unstructured.NestedSlice(u.Object, "spec", "crossplane", "resourceRefs"); found {
		return NodeXR
	}
	gvk := u.GroupVersionKind()
	if crossplane.LooksLikeManagedResource(gvk.Group) {
		return NodeMR
	}
	return NodeComposed
}
