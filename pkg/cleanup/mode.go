package cleanup

import "fmt"

// DeletionMode selects how stacks are planned and (optionally) inspected.
type DeletionMode string

const (
	// ModeRoots discovers highest-parent XRs only and plans root deletes
	// (Crossplane GC / foreground propagation handles dependents). Default.
	ModeRoots DeletionMode = "roots"
	// ModeTrace builds full parent→child composition traces for detailed
	// plans, nested keep/paused gates, orphans, and richer execute.
	ModeTrace DeletionMode = "trace"
)

// ParseMode validates a mode string; empty defaults to roots.
func ParseMode(s string) (DeletionMode, error) {
	switch DeletionMode(s) {
	case "", ModeRoots:
		return ModeRoots, nil
	case ModeTrace:
		return ModeTrace, nil
	default:
		return "", fmt.Errorf("invalid deletion.mode %q (want roots|trace)", s)
	}
}
