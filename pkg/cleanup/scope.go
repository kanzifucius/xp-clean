package cleanup

import "fmt"

// DeletionScope controls whether definitions are deleted after instances.
type DeletionScope string

const (
	// ScopeInstancesOnly deletes eligible XR roots only (default, safer).
	ScopeInstancesOnly DeletionScope = "instances-only"
	// ScopeAll deletes instances then matching definitions (revisions → composition → XRD).
	ScopeAll DeletionScope = "all"
)

// ParseScope validates a scope string; empty defaults to instances-only.
func ParseScope(s string) (DeletionScope, error) {
	switch DeletionScope(s) {
	case "", ScopeInstancesOnly:
		return ScopeInstancesOnly, nil
	case ScopeAll:
		return ScopeAll, nil
	default:
		return "", fmt.Errorf("invalid deletion.scope %q (want instances-only|all)", s)
	}
}
