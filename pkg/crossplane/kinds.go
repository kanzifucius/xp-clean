package crossplane

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// XRDGroup is the preferred Crossplane apiextensions group.
	XRDGroup = "apiextensions.crossplane.io"
	// LabelComposite is set on composed resources.
	LabelComposite = "crossplane.io/composite"
	// LabelCompositionName groups CompositionRevisions.
	LabelCompositionName = "crossplane.io/composition-name"
)

// XRDGVKs are the API versions Xp-clean will attempt when listing XRDs.
var XRDGVKs = []schema.GroupVersionResource{
	{Group: XRDGroup, Version: "v2", Resource: "compositeresourcedefinitions"},
	{Group: XRDGroup, Version: "v1", Resource: "compositeresourcedefinitions"},
}

// CompositionGVR is the Composition resource.
var CompositionGVR = schema.GroupVersionResource{
	Group: XRDGroup, Version: "v1", Resource: "compositions",
}

// CompositionRevisionGVR is the CompositionRevision resource.
var CompositionRevisionGVR = schema.GroupVersionResource{
	Group: XRDGroup, Version: "v1", Resource: "compositionrevisions",
}

// XRServeFromXRD derives the served XR GVK from an XRD unstructured object.
// Claim names are ignored entirely (no claim support).
func XRServeFromXRD(xrd *unstructured.Unstructured) (schema.GroupVersionKind, error) {
	group, _, err := unstructured.NestedString(xrd.Object, "spec", "group")
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	kind, _, err := unstructured.NestedString(xrd.Object, "spec", "names", "kind")
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if group == "" || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("XRD %s missing spec.group or spec.names.kind", xrd.GetName())
	}

	version, err := preferredServedVersion(xrd)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}, nil
}

func preferredServedVersion(xrd *unstructured.Unstructured) (string, error) {
	versions, found, err := unstructured.NestedSlice(xrd.Object, "spec", "versions")
	if err != nil {
		return "", err
	}
	if !found || len(versions) == 0 {
		return "", fmt.Errorf("XRD %s has no versions", xrd.GetName())
	}

	var fallback string
	for _, raw := range versions {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		if fallback == "" {
			fallback = name
		}
		served, _ := m["served"].(bool)
		storage, _ := m["storage"].(bool)
		if storage && (served || m["served"] == nil) {
			return name, nil
		}
		if served {
			fallback = name
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("XRD %s has no usable version", xrd.GetName())
	}
	return fallback, nil
}

// LooksLikeManagedResource is a heuristic for provider MR API groups
// (namespaced *.m.* groups in Crossplane v2).
func LooksLikeManagedResource(group string) bool {
	// Crossplane v2 namespaced MRs often live under *.m.* API groups.
	return strings.Contains(group, ".m.")
}

// IsLegacyClusterScope reports whether an XRD uses LegacyCluster scope.
// Xp-clean does not support claim paths for these XRDs.
func IsLegacyClusterScope(xrd *unstructured.Unstructured) bool {
	scope, _, _ := unstructured.NestedString(xrd.Object, "spec", "scope")
	return scope == "LegacyCluster"
}

// PluralFromXRD returns the plural resource name for listing XRs.
func PluralFromXRD(xrd *unstructured.Unstructured) (string, error) {
	plural, _, err := unstructured.NestedString(xrd.Object, "spec", "names", "plural")
	if err != nil {
		return "", err
	}
	if plural == "" {
		return "", fmt.Errorf("XRD %s missing plural", xrd.GetName())
	}
	return plural, nil
}
