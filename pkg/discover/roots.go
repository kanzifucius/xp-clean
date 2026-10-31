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
	}
	return c, nil
}

// HighestParents filters candidates that are not themselves composed children
// of another XR in the candidate set (controller ownerReference to another XR).
func HighestParents(candidates []RootCandidate) []RootCandidate {
	byUID := map[string]*unstructured.Unstructured{}
	for _, c := range candidates {
		byUID[string(c.XR.GetUID())] = c.XR
	}

	out := make([]RootCandidate, 0, len(candidates))
	for _, c := range candidates {
		if isOwnedByOtherXR(c.XR, byUID) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func isOwnedByOtherXR(xr *unstructured.Unstructured, byUID map[string]*unstructured.Unstructured) bool {
	for _, or := range xr.GetOwnerReferences() {
		if or.Controller == nil || !*or.Controller {
			continue
		}
		if _, ok := byUID[string(or.UID)]; ok {
			return true
		}
		// Also treat owner kind matching common XR patterns via API version group.
		if or.UID != "" {
			// If owner is in our XR map by name+kind heuristic when UID missing from set.
			_ = or
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
	return HighestParents(filterEligibleOnly(all)), nil
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
