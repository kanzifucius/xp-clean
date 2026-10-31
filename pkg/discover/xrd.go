package discover

import (
	"context"
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/crossplane"
	"github.com/kanzifucius/xp-clean/pkg/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// XRDInfo is a discovered CompositeResourceDefinition and its XR GVK.
type XRDInfo struct {
	XRD              *unstructured.Unstructured
	XRGVK            schema.GroupVersionKind
	XRResource       string
	LegacyCluster    bool
	Eligible         bool
	MatchReason      string
}

// ListXRDs tries v2 then v1 XRD APIs.
func ListXRDs(ctx context.Context, dyn *DynamicClient) ([]*unstructured.Unstructured, error) {
	var lastErr error
	for _, gvr := range crossplane.XRDGVKs {
		list, err := dyn.List(ctx, gvr, "", metav1.ListOptions{})
		if err != nil {
			lastErr = err
			continue
		}
		out := make([]*unstructured.Unstructured, 0, len(list.Items))
		for i := range list.Items {
			out = append(out, &list.Items[i])
		}
		return out, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no XRD API available")
}

// InspectXRD derives XR GVK metadata and optional eligibility for an XRD.
func InspectXRD(xrd *unstructured.Unstructured, sel match.Selectors) (XRDInfo, error) {
	gvk, err := crossplane.XRServeFromXRD(xrd)
	if err != nil {
		return XRDInfo{}, err
	}
	plural, err := crossplane.PluralFromXRD(xrd)
	if err != nil {
		return XRDInfo{}, err
	}
	info := XRDInfo{
		XRD:           xrd,
		XRGVK:         gvk,
		XRResource:    plural,
		LegacyCluster: crossplane.IsLegacyClusterScope(xrd),
	}

	res, err := match.Evaluate(match.Input{
		Meta: match.ObjectMeta{
			Name:        xrd.GetName(),
			Labels:      xrd.GetLabels(),
			Annotations: xrd.GetAnnotations(),
		},
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
		Target:  match.TargetXRD,
	}, sel)
	if err != nil {
		return info, err
	}
	info.Eligible = res.Eligible
	info.MatchReason = res.Reason
	return info, nil
}

// FilterEligibleXRDs returns XRDs that match selectors.
// LegacyCluster XRDs may still appear for XR GVK discovery but claim edges are never walked.
func FilterEligibleXRDs(infos []XRDInfo) []XRDInfo {
	out := make([]XRDInfo, 0, len(infos))
	for _, info := range infos {
		if info.Eligible {
			out = append(out, info)
		}
	}
	return out
}
