package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RootCandidate is an XR that may be a highest instance parent.
type RootCandidate struct {
	XR        *unstructured.Unstructured
	XRD       XRDInfo
	Eligible  bool
	Expired   bool
	Keep      bool
	Paused    bool
	Reason    string
	ExpiresAt *time.Time
}

// ListXRs lists composite instances for an XRD-derived GVR.
func ListXRs(ctx context.Context, dyn *DynamicClient, info XRDInfo, namespace string) ([]*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    info.XRGVK.Group,
		Version:  info.XRGVK.Version,
		Resource: info.XRResource,
	}
	list, err := dyn.List(ctx, gvr, namespace, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, &list.Items[i])
	}
	return out, nil
}

// EvaluateXR applies selectors + TTL to one XR instance.
func EvaluateXR(xr *unstructured.Unstructured, info XRDInfo, sel match.Selectors, ttl match.TTLConfig, now time.Time) (RootCandidate, error) {
	c := RootCandidate{XR: xr, XRD: info}
	res, err := match.Evaluate(match.Input{
		Meta: match.ObjectMeta{
			Name:              xr.GetName(),
			Namespace:         xr.GetNamespace(),
			Labels:            xr.GetLabels(),
			Annotations:       xr.GetAnnotations(),
			CreationTimestamp: xr.GetCreationTimestamp().Time,
		},
		Group:   info.XRGVK.Group,
		Version: info.XRGVK.Version,
		Kind:    info.XRGVK.Kind,
		Target:  match.TargetXR,
	}, sel)
	if err != nil {
		return c, err
	}
	c.Eligible = res.Eligible
	c.Keep = res.Keep
	c.Paused = res.Paused
	c.Reason = res.Reason
	if !res.Eligible {
		return c, nil
	}

	exp, err := match.EvaluateExpiry(match.ObjectMeta{
		Name:              xr.GetName(),
		Namespace:         xr.GetNamespace(),
		Labels:            xr.GetLabels(),
		Annotations:       xr.GetAnnotations(),
		CreationTimestamp: xr.GetCreationTimestamp().Time,
	}, ttl, now)
	if err != nil {
		return c, err
	}
	c.Expired = exp.Expired
	c.ExpiresAt = exp.ExpiresAt
	if !exp.Expired {
		c.Reason = "ttl_not_elapsed"
		c.Eligible = false
		return c, nil
	}
	// Eligible: keep selector cause and append TTL fragment when it contributed.
	c.Reason = match.JoinEligibility(res.Reason, match.TTLElapsedReason(exp))
	return c, nil
}

// XRKindsFromXRDInfos builds the set of XR GroupKinds (version ignored) from
// inspected XRDs. Empty XRGVKs are skipped.
func XRKindsFromXRDInfos(infos []XRDInfo) map[schema.GroupKind]struct{} {
	out := make(map[schema.GroupKind]struct{}, len(infos))
	for _, info := range infos {
		if info.XRGVK.Empty() {
			continue
		}
		out[schema.GroupKind{Group: info.XRGVK.Group, Kind: info.XRGVK.Kind}] = struct{}{}
	}
	return out
}

// HighestParents filters candidates that are composed children of another XR
// kind (controller ownerReference whose GroupKind is in xrKinds). Parent XRs
// need not be in the candidate set — only the cluster XRD-derived kind set.
func HighestParents(candidates []RootCandidate, xrKinds map[schema.GroupKind]struct{}) []RootCandidate {
	out := make([]RootCandidate, 0, len(candidates))
	for _, c := range candidates {
		if isComposedChildOfXR(c.XR, xrKinds) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// isComposedChildOfXR reports whether xr has a controller ownerReference whose
// GroupKind is an XR kind. Non-controller owners and non-XR owners do not count.
func isComposedChildOfXR(xr *unstructured.Unstructured, xrKinds map[schema.GroupKind]struct{}) bool {
	if len(xrKinds) == 0 {
		return false
	}
	for _, or := range xr.GetOwnerReferences() {
		if or.Controller == nil || !*or.Controller {
			continue
		}
		gv, err := schema.ParseGroupVersion(or.APIVersion)
		if err != nil {
			continue
		}
		if _, ok := xrKinds[schema.GroupKind{Group: gv.Group, Kind: or.Kind}]; ok {
			return true
		}
	}
	return false
}

// DiscoverRoots lists eligible highest-parent XRs for matching XRDs.
func DiscoverRoots(ctx context.Context, dyn *DynamicClient, sel match.Selectors, ttl match.TTLConfig, now time.Time) ([]RootCandidate, error) {
	xrds, err := ListXRDs(ctx, dyn)
	if err != nil {
		return nil, fmt.Errorf("list XRDs: %w", err)
	}
	infos := make([]XRDInfo, 0, len(xrds))
	for _, xrd := range xrds {
		info, err := InspectXRD(xrd, sel)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	// Kind set from ALL XRDs (before selector filter) so a composed child is
	// never treated as a root when its parent XR kind was filtered out.
	xrKinds := XRKindsFromXRDInfos(infos)
	eligible := FilterEligibleXRDs(infos)

	var all []RootCandidate
	for _, info := range eligible {
		xrs, err := ListXRs(ctx, dyn, info, "")
		if err != nil {
			// Cluster-scoped vs namespaced: try empty namespace first; if API requires ns, skip with error wrap.
			return nil, fmt.Errorf("list XRs for %s: %w", info.XRD.GetName(), err)
		}
		for _, xr := range xrs {
			c, err := EvaluateXR(xr, info, sel, ttl, now)
			if err != nil {
				return nil, err
			}
			if c.Eligible || c.Keep {
				all = append(all, c)
			}
		}
	}
	return HighestParents(filterEligibleOnly(all), xrKinds), nil
}

func filterEligibleOnly(in []RootCandidate) []RootCandidate {
	out := make([]RootCandidate, 0, len(in))
	for _, c := range in {
		if c.Eligible {
			out = append(out, c)
		}
	}
	return out
}
