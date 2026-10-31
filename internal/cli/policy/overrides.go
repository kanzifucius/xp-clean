package policy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// FlagOverrides are Cobra/env product overrides applied after loading policy.
// Precedence: flags > env > policy document > defaults (architecture §8).
type FlagOverrides struct {
	// ConfigPath is the --config value (file override). Empty means use
	// in-cluster ConfigMaps (or XP_CLEAN_CONFIG when set).
	ConfigPath string

	// PolicyNamespace is the namespace for labelled policy ConfigMaps.
	// Empty defaults to conf.DefaultPolicyNamespace ("xp-clean").
	PolicyNamespace string

	// DryRunSet is true when --dry-run was explicitly passed on the argv.
	DryRunSet bool
	DryRun    bool

	// Scope is empty when unset on the argv.
	Scope string

	// ForceDryRun forces dry-run on after merge (used by plan).
	ForceDryRun bool
}

// ResolveConfigPath picks --config, then XP_CLEAN_CONFIG. Empty means no file
// override — callers should load in-cluster ConfigMaps instead.
// There is no implicit repo-relative sample path.
func ResolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	return strings.TrimSpace(os.Getenv("XP_CLEAN_CONFIG"))
}

// ResolvePolicyNamespace picks --policy-namespace, then XP_CLEAN_POLICY_NAMESPACE,
// then the dedicated xp-clean default.
func ResolvePolicyNamespace(flagNS string) string {
	if flagNS != "" {
		return flagNS
	}
	if v := strings.TrimSpace(os.Getenv("XP_CLEAN_POLICY_NAMESPACE")); v != "" {
		return v
	}
	return conf.DefaultPolicyNamespace
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

// LoadAndMerge loads a single policy from an explicit file path (--config /
// XP_CLEAN_CONFIG) and applies env + flag overrides.
// Callers that want in-cluster ConfigMaps should use LoadPolicies.
func LoadAndMerge(o FlagOverrides) (conf.Resolved, error) {
	if err := ApplyEnv(&o); err != nil {
		return conf.Resolved{}, err
	}
	path := ResolveConfigPath(o.ConfigPath)
	if path == "" {
		return conf.Resolved{}, fmt.Errorf("policy source required: pass --config PATH (or set XP_CLEAN_CONFIG), or omit both to load labelled ConfigMaps from the xp-clean namespace")
	}
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

// LoadPolicies resolves the policy source:
//   - explicit --config / XP_CLEAN_CONFIG → one file policy
//   - otherwise → all labelled ConfigMaps in the policy namespace (multi-tenant)
func LoadPolicies(ctx context.Context, o FlagOverrides, restCfg *rest.Config) ([]conf.Resolved, error) {
	if err := ApplyEnv(&o); err != nil {
		return nil, err
	}
	path := ResolveConfigPath(o.ConfigPath)
	if path != "" {
		p, err := conf.LoadFile(path)
		if err != nil {
			return nil, err
		}
		if err := applyFlagOverrides(&p, o); err != nil {
			return nil, err
		}
		if o.ForceDryRun {
			p.DryRun = true
		}
		return []conf.Resolved{p}, nil
	}

	if restCfg == nil {
		return nil, fmt.Errorf("policy source required: pass --config PATH (or set XP_CLEAN_CONFIG), or provide cluster access to load ConfigMaps")
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client for ConfigMaps: %w", err)
	}
	ns := ResolvePolicyNamespace(o.PolicyNamespace)
	policies, err := conf.LoadFromConfigMaps(ctx, conf.NewClientsetLister(client), ns)
	if err != nil {
		return nil, err
	}
	for i := range policies {
		if err := applyFlagOverrides(&policies[i], o); err != nil {
			return nil, err
		}
		if o.ForceDryRun {
			policies[i].DryRun = true
		}
	}
	return policies, nil
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
