package policy

import (
	"fmt"
	"os"
	"strings"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
)

// FlagOverrides are Cobra/env product overrides applied after loading the policy file.
// Precedence: flags > env > policy document > defaults (architecture §8).
type FlagOverrides struct {
	// ConfigPath is the --config value (may be empty before env merge).
	ConfigPath string

	// DryRunSet is true when --dry-run was explicitly passed on the argv.
	DryRunSet bool
	DryRun    bool

	// Scope is empty when unset on the argv.
	Scope string

	// ForceDryRun forces dry-run on after merge (used by plan).
	ForceDryRun bool
}

// ResolveConfigPath picks --config, then XP_CLEAN_CONFIG, then defaultPath.
func ResolveConfigPath(flagPath, defaultPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if v := strings.TrimSpace(os.Getenv("XP_CLEAN_CONFIG")); v != "" {
		return v
	}
	return defaultPath
}

// ApplyEnv overlays XP_CLEAN_DRY_RUN / XP_CLEAN_SCOPE when the corresponding
// flag was not set. Call before ApplyFlags or merge inside LoadAndMerge.
func ApplyEnv(o *FlagOverrides) error {
	if !o.DryRunSet {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_DRY_RUN")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("XP_CLEAN_DRY_RUN: %w", err)
			}
			o.DryRunSet = true
			o.DryRun = b
		}
	}
	if o.Scope == "" {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_SCOPE")); v != "" {
			o.Scope = v
		}
	}
	return nil
}

// LoadAndMerge loads the policy file and applies env + flag overrides.
func LoadAndMerge(o FlagOverrides) (conf.Resolved, error) {
	if err := ApplyEnv(&o); err != nil {
		return conf.Resolved{}, err
	}
	path := ResolveConfigPath(o.ConfigPath, "config/samples/cleanuppolicy.yaml")
	policy, err := conf.LoadFile(path)
	if err != nil {
		return conf.Resolved{}, err
	}
	if err := applyFlagOverrides(&policy, o); err != nil {
		return conf.Resolved{}, err
	}
	if o.ForceDryRun {
		policy.DryRun = true
	}
	return policy, nil
}

func applyFlagOverrides(policy *conf.Resolved, o FlagOverrides) error {
	if o.DryRunSet {
		policy.DryRun = o.DryRun
	}
	if o.Scope != "" {
		s, err := cleanup.ParseScope(o.Scope)
		if err != nil {
			return fmt.Errorf("invalid --scope: %w", err)
		}
		policy.Deletion.Scope = s
	}
	return nil
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", v)
	}
}

// ToPolicyView maps a resolved policy into the cleanup runner view.
func ToPolicyView(p conf.Resolved) cleanup.PolicyView {
	return cleanup.PolicyView{
		Name:      p.Name,
		DryRun:    p.DryRun,
		Selectors: p.Selectors,
		TTL:       p.TTL,
		Deletion:  p.Deletion,
	}
}
