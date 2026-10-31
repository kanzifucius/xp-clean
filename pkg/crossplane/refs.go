package crossplane

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceRef is one entry from spec.crossplane.resourceRefs (v2 XR shape).
type ResourceRef struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

// APIVersionGroup returns the group portion of apiVersion.
func (r ResourceRef) APIVersionGroup() string {
	group, _, _ := SplitAPIVersion(r.APIVersion)
	return group
}

// APIVersionVersion returns the version portion of apiVersion.
func (r ResourceRef) APIVersionVersion() string {
	_, version, _ := SplitAPIVersion(r.APIVersion)
	return version
}

// SplitAPIVersion splits "group/version" or "version" (core) into parts.
func SplitAPIVersion(apiVersion string) (group, version string, err error) {
	if apiVersion == "" {
		return "", "", fmt.Errorf("empty apiVersion")
	}
	if !strings.Contains(apiVersion, "/") {
		return "", apiVersion, nil
	}
	parts := strings.SplitN(apiVersion, "/", 2)
	return parts[0], parts[1], nil
}

// ResourceRefs extracts spec.crossplane.resourceRefs from an XR.
// Claim references are never read — Xp-clean is XR-only / v2-native.
func ResourceRefs(u *unstructured.Unstructured) ([]ResourceRef, error) {
	raw, found, err := unstructured.NestedSlice(u.Object, "spec", "crossplane", "resourceRefs")
	if err != nil {
		return nil, err
	}
	if !found {
		// Fall back to legacy-ish spec.resourceRefs if present on some XRs.
		raw, found, err = unstructured.NestedSlice(u.Object, "spec", "resourceRefs")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
	}

	out := make([]ResourceRef, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("resourceRefs[%d]: not an object", i)
		}
		rr := ResourceRef{
			APIVersion: asString(m["apiVersion"]),
			Kind:       asString(m["kind"]),
			Name:       asString(m["name"]),
			Namespace:  asString(m["namespace"]),
		}
		if rr.APIVersion == "" || rr.Kind == "" || rr.Name == "" {
			return nil, fmt.Errorf("resourceRefs[%d]: incomplete ref", i)
		}
		out = append(out, rr)
	}
	return out, nil
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// CompositionRefName returns the composition name from spec.crossplane.compositionRef.name.
func CompositionRefName(u *unstructured.Unstructured) string {
	name, _, _ := unstructured.NestedString(u.Object, "spec", "crossplane", "compositionRef", "name")
	if name != "" {
		return name
	}
	name, _, _ = unstructured.NestedString(u.Object, "spec", "compositionRef", "name")
	return name
}
