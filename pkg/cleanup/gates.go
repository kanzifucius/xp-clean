package cleanup

import (
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/trace"
)

// DeletionConfig holds safety gates and budgets for a run.
type DeletionConfig struct {
	Scope            DeletionScope  `json:"scope" yaml:"scope"`
	MaxTracesPerRun  int            `json:"maxTracesPerRun" yaml:"maxTracesPerRun"`
	MaxDeletesPerRun int            `json:"maxDeletesPerRun" yaml:"maxDeletesPerRun"`
	OrphanMode       string         `json:"orphanMode" yaml:"orphanMode"`
	DeletePaused     bool           `json:"deletePaused" yaml:"deletePaused"`
	AllowXRDDeletion bool           `json:"allowXRDDeletion" yaml:"allowXRDDeletion"`
	WaitTimeout      string         `json:"waitTimeout" yaml:"waitTimeout"`
}

// DefaultDeletionConfig returns safer MVP defaults.
func DefaultDeletionConfig() DeletionConfig {
	return DeletionConfig{
		Scope:            ScopeInstancesOnly,
		MaxTracesPerRun:  20,
		MaxDeletesPerRun: 100,
		OrphanMode:       "report",
		DeletePaused:     false,
		AllowXRDDeletion: false,
		WaitTimeout:      "30m",
	}
}

// GateResult is the outcome of safety checks for one trace.
type GateResult struct {
	OK        bool
	BlockedBy []string
}

// CheckGates evaluates MVP safety gates for a planned trace deletion.
func CheckGates(t trace.Trace, cfg DeletionConfig, dryRun bool) GateResult {
	var blocked []string

	if t.HasKeep() {
		blocked = append(blocked, "xp-clean.io/keep on trace path")
	}
	if t.HasPaused() && !cfg.DeletePaused {
		blocked = append(blocked, "crossplane.io/paused (deletePaused=false)")
	}
	if !rootEligible(t) {
		blocked = append(blocked, "root not eligible")
	}

	_ = dryRun // dry-run is handled by the executor; gates still apply for planning visibility
	return GateResult{OK: len(blocked) == 0, BlockedBy: blocked}
}

func rootEligible(t trace.Trace) bool {
	for _, n := range t.Nodes {
		if n.Ref.Key() == t.Root.Key() {
			return n.Eligible && !n.Keep
		}
	}
	return false
}

// ValidateDefinitionStep returns an error if a definition delete is not allowed.
func ValidateDefinitionStep(kind trace.NodeKind, cfg DeletionConfig) error {
	if cfg.Scope != ScopeAll {
		return fmt.Errorf("definition delete blocked: deletion.scope is %s", cfg.Scope)
	}
	if kind == trace.NodeXRD && !cfg.AllowXRDDeletion {
		return fmt.Errorf("XRD delete blocked: allowXRDDeletion=false")
	}
	return nil
}
