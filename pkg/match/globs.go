package match

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MatchAny returns true if value matches any pattern, or if patterns is empty.
// Empty patterns mean "no constraint from this dimension".
func MatchAny(value string, patterns []string) (bool, error) {
	if len(patterns) == 0 {
		return true, nil
	}
	for _, p := range patterns {
		ok, err := filepath.Match(p, value)
		if err != nil {
			return false, fmt.Errorf("invalid glob %q: %w", p, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// MatchGVK matches a resource against GVK glob patterns.
// First-cut patterns are group/version (kind not required), e.g.
// "apps.example.org/v1" or "*.dev.example.org/*". Kind-bearing patterns
// remain accepted if operators supply them. '*' does not cross '/'.
func MatchGVK(group, version, kind string, patterns []string) (bool, error) {
	if len(patterns) == 0 {
		return true, nil
	}
	candidates := []string{
		fmt.Sprintf("%s/%s", group, version),
		fmt.Sprintf("%s/%s/%s", group, version, kind),
	}
	// Also allow matching group/* style by testing each candidate.
	for _, c := range candidates {
		ok, err := MatchAny(c, patterns)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// NamespaceDenied reports whether namespace is in the deny list (exact or glob).
func NamespaceDenied(namespace string, deny []string) (bool, error) {
	if namespace == "" {
		return false, nil
	}
	for _, d := range deny {
		if strings.ContainsAny(d, "*?[") {
			ok, err := filepath.Match(d, namespace)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
			continue
		}
		if d == namespace {
			return true, nil
		}
	}
	return false, nil
}
