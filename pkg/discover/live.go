package discover

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// LiveBackend discovers and deletes against a real cluster.
type LiveBackend struct {
	Dyn     *DynamicClient
	Mapper  *restmapper.DeferredDiscoveryRESTMapper
	Fetcher *Fetcher
}

// NewLiveBackend builds a LiveBackend from a REST config.
func NewLiveBackend(cfg *rest.Config) (*LiveBackend, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))
	dc := &DynamicClient{Client: dyn}
	lb := &LiveBackend{
		Dyn:    dc,
		Mapper: mapper,
	}
	lb.Fetcher = &Fetcher{
		Dyn: dc,
		Resource: func(group, version, kind string) (schema.GroupVersionResource, error) {
			return lb.gvr(group, version, kind)
		},
	}
	return lb, nil
}

// GVR resolves group/version/kind to a GroupVersionResource via discovery.
func (l *LiveBackend) GVR(group, version, kind string) (schema.GroupVersionResource, error) {
	mapping, err := l.Mapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

func (l *LiveBackend) gvr(group, version, kind string) (schema.GroupVersionResource, error) {
	return l.GVR(group, version, kind)
}

// DiscoverRoots implements cleanup.ClusterBackend.
func (l *LiveBackend) DiscoverRoots(ctx context.Context, sel match.Selectors, ttl match.TTLConfig, now time.Time) ([]RootCandidate, error) {
	return DiscoverRoots(ctx, l.Dyn, sel, ttl, now)
}

// BuildTrace implements cleanup.ClusterBackend.
func (l *LiveBackend) BuildTrace(ctx context.Context, root *unstructured.Unstructured) (trace.Trace, error) {
	return trace.BuildFromXR(ctx, root, l.Fetcher, trace.BuildOptions{})
}

// Delete implements cleanup.ClusterBackend / cleanup.Deleter.
func (l *LiveBackend) Delete(ctx context.Context, ref trace.ObjectRef) error {
	gvr, err := l.gvr(ref.Group, ref.Version, ref.Kind)
	if err != nil {
		return err
	}
	return l.Dyn.Delete(ctx, gvr, ref.Namespace, ref.Name)
}

// Get implements cleanup.ObjectGetter for wait-after-delete polling.
func (l *LiveBackend) Get(ctx context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	if l.Fetcher == nil {
		return nil, fmt.Errorf("live backend fetcher not configured")
	}
	return l.Fetcher.Get(ctx, ref)
}

// CountXRs implements cleanup.XRCounter for the zero-XR definition gate.
func (l *LiveBackend) CountXRs(ctx context.Context, info XRDInfo) (int, error) {
	xrs, err := ListXRs(ctx, l.Dyn, info, "")
	if err != nil {
		return 0, err
	}
	return len(xrs), nil
}

// AttachDefinitions implements cleanup.DefinitionAttacher.
func (l *LiveBackend) AttachDefinitions(ctx context.Context, tr *trace.Trace, xrd, xr *unstructured.Unstructured) {
	AttachDefinitions(ctx, l.Dyn, tr, xrd, xr)
}

// AttachDefinitions enriches a trace with XRD / Composition / Revision nodes
// when deletion.scope=all planning needs them. Best-effort; warnings on failure.
func AttachDefinitions(ctx context.Context, dyn *DynamicClient, tr *trace.Trace, xrd *unstructured.Unstructured, xr *unstructured.Unstructured) {
	if xrd != nil {
		ref := trace.RefFromUnstructured(xrd)
		tr.Nodes = append(tr.Nodes, trace.Node{Ref: ref, Kind: trace.NodeXRD, Labels: xrd.GetLabels(), Eligible: true})
		tr.Edges = append(tr.Edges, trace.Edge{From: ref, To: tr.Root, Reason: "xrdMatch"})
	}
	compName := crossplane.CompositionRefName(xr)
	if compName == "" || dyn == nil {
		return
	}
	comp, err := dyn.Get(ctx, crossplane.CompositionGVR, "", compName)
	if err != nil {
		tr.Warnings = append(tr.Warnings, fmt.Sprintf("composition %s: %v", compName, err))
		return
	}
	compRef := trace.RefFromUnstructured(comp)
	tr.Nodes = append(tr.Nodes, trace.Node{Ref: compRef, Kind: trace.NodeComposition, Labels: comp.GetLabels(), Eligible: true})
	if xrd != nil {
		tr.Edges = append(tr.Edges, trace.Edge{From: trace.RefFromUnstructured(xrd), To: compRef, Reason: "compositeTypeRef"})
	}
	tr.Edges = append(tr.Edges, trace.Edge{From: compRef, To: tr.Root, Reason: "compositionRef"})

	list, err := dyn.List(ctx, crossplane.CompositionRevisionGVR, "", metav1.ListOptions{
		LabelSelector: crossplane.LabelCompositionName + "=" + compName,
	})
	if err != nil {
		tr.Warnings = append(tr.Warnings, fmt.Sprintf("compositionrevisions: %v", err))
		return
	}
	for i := range list.Items {
		rev := &list.Items[i]
		revRef := trace.RefFromUnstructured(rev)
		tr.Nodes = append(tr.Nodes, trace.Node{Ref: revRef, Kind: trace.NodeCompositionRevision, Labels: rev.GetLabels(), Eligible: true})
		tr.Edges = append(tr.Edges, trace.Edge{From: compRef, To: revRef, Reason: "compositionRevision"})
	}
}

// ParseNamespacedName parses "namespace/name" or "name".
func ParseNamespacedName(s string) (ns, name string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("empty object name")
	}
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 1:
		return "", parts[0], nil
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("invalid object name %q (want name or namespace/name)", s)
	}
}
