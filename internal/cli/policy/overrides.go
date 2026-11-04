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

	// DryRunSet is true when --dry-run=true (or XP_CLEAN_DRY_RUN=true) was set.
	// DryRun=false from flags/env never overrides a policy dryRun: true — use
	// ForceDryRun / AlwaysDryRun instead. Live deletes need policy dryRun: false
	// plus --apply (or --dry-run=false clearing ForceDryRun).
	DryRunSet bool
	DryRun    bool

	// Scope is empty when unset on the argv.
	Scope string

	// Mode is empty when unset on the argv (deletion.mode: roots|trace).
	Mode string

	// WaitSet is true when --wait was explicitly passed.
	WaitSet bool
	Wait    bool

	// ForceDryRun forces dry-run on after merge. Cleared by --apply /
	// --dry-run=false / XP_CLEAN_DRY_RUN=false unless AlwaysDryRun is set.
	ForceDryRun bool

	// AlwaysDryRun is true for plan: never cleared by env or live flags.
	AlwaysDryRun bool

	// LiveControlsFromFlags is true when --apply or --dry-run was set on argv
	// (env must not override those).
	LiveControlsFromFlags bool

	// NotifySet is true when --notify was explicitly passed.
	NotifySet bool
	Notify    bool

	// NotifyOnDryRunSet is true when --notify-on-dry-run was explicitly passed.
	NotifyOnDryRunSet bool
	NotifyOnDryRun    bool

	// GVKGlobs, when non-empty, replaces selectors.globs.gvks entirely
	// (no merge with the policy document). When empty/omitted, policy globs stay.
	GVKGlobs []string
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

// ApplyEnv overlays XP_CLEAN_* env vars when the corresponding flag was not set.
// Call before ApplyFlags or merge inside LoadAndMerge.
func ApplyEnv(o *FlagOverrides) error {
	if !o.LiveControlsFromFlags && !o.AlwaysDryRun {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_DRY_RUN")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("XP_CLEAN_DRY_RUN: %w", err)
			}
			if b {
				o.DryRunSet = true
				o.DryRun = true
				o.ForceDryRun = true
			} else {
				// Allow live if policy permits; do not override policy dryRun: true.
				o.ForceDryRun = false
			}
		}
	}
	if o.Scope == "" {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_SCOPE")); v != "" {
			o.Scope = v
		}
	}
	if o.Mode == "" {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_MODE")); v != "" {
			o.Mode = v
		}
	}
	if !o.WaitSet {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_WAIT")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("XP_CLEAN_WAIT: %w", err)
			}
			o.WaitSet = true
			o.Wait = b
		}
	}
	if !o.NotifySet {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_NOTIFY")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("XP_CLEAN_NOTIFY: %w", err)
			}
			o.NotifySet = true
			o.Notify = b
		}
	}
	if !o.NotifyOnDryRunSet {
		if v := strings.TrimSpace(os.Getenv("XP_CLEAN_NOTIFY_ON_DRY_RUN")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("XP_CLEAN_NOTIFY_ON_DRY_RUN: %w", err)
			}
			o.NotifyOnDryRunSet = true
			o.NotifyOnDryRun = b
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
	applyDryRunGate(&policy, o)
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
		applyDryRunGate(&p, o)
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
		applyDryRunGate(&policies[i], o)
	}
	return policies, nil
}

func applyFlagOverrides(policy *conf.Resolved, o FlagOverrides) error {
	// Only --dry-run=true / XP_CLEAN_DRY_RUN=true may raise dryRun; never force live
	// past a policy that still has dryRun: true.
	if o.DryRunSet && o.DryRun {
		policy.DryRun = true
	}
	if o.Scope != "" {
		s, err := cleanup.ParseScope(o.Scope)
		if err != nil {
			return fmt.Errorf("invalid --scope: %w", err)
		}
		policy.Deletion.Scope = s
	}
	if o.Mode != "" {
		m, err := cleanup.ParseMode(o.Mode)
		if err != nil {
			return fmt.Errorf("invalid --mode: %w", err)
		}
		policy.Deletion.Mode = m
	}
	if o.WaitSet {
		policy.Deletion.Wait = o.Wait
	}
	if o.NotifySet {
		policy.Notifications.Enabled = o.Notify
	}
	if o.NotifyOnDryRunSet {
		policy.Notifications.OnDryRun = o.NotifyOnDryRun
	}
	if len(o.GVKGlobs) > 0 {
		// Replace, do not merge — CLI values become the sole GVK glob list.
		policy.Selectors.Globs.GVKs = append([]string(nil), o.GVKGlobs...)
	}
	return nil
}

func applyDryRunGate(policy *conf.Resolved, o FlagOverrides) {
	if o.AlwaysDryRun || o.ForceDryRun {
		policy.DryRun = true
	}
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
