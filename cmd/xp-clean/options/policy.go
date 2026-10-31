package options

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// PolicyFlags are product flags shared by run/plan/manager.
type PolicyFlags struct {
	ConfigPath      string
	PolicyNamespace string
	DryRun          bool
	Scope           string
	FailOnBlocked   bool
	Notify          bool
	NotifyOnDryRun  bool
}

// Bind registers --config / --dry-run / --scope / ConfigMap / notify flags on a command.
func (p *PolicyFlags) Bind(fs *pflag.FlagSet, withDryRun bool) {
	fs.StringVar(&p.ConfigPath, "config", "", "path to policy YAML file override (explicit; no implicit sample path). When unset, load labelled ConfigMaps from --policy-namespace")
	fs.StringVar(&p.PolicyNamespace, "policy-namespace", "", "namespace for labelled policy ConfigMaps (default: xp-clean or XP_CLEAN_POLICY_NAMESPACE)")
	if withDryRun {
		// Default false is irrelevant; use Changed("dry-run") to detect overrides.
		fs.BoolVar(&p.DryRun, "dry-run", false, "planner dry-run only (no deletes); when unset, use policy dryRun (default true). Not kubectl API-server dry-run")
	}
	fs.StringVar(&p.Scope, "scope", "", "deletion.scope override: instances-only|all (default from policy, then instances-only)")
	fs.BoolVar(&p.FailOnBlocked, "fail-on-blocked", false, "exit 3 when the run completed with blocked plans")
	fs.BoolVar(&p.Notify, "notify", false, "override observability.notifications.enabled (webhook URL still from env)")
	fs.BoolVar(&p.NotifyOnDryRun, "notify-on-dry-run", true, "override observability.notifications.onDryRun")
}

// Overrides builds FlagOverrides from flags (including dry-run Changed state).
func (p *PolicyFlags) Overrides(fs *pflag.FlagSet, forceDryRun bool) policy.FlagOverrides {
	o := policy.FlagOverrides{
		ConfigPath:      p.ConfigPath,
		PolicyNamespace: p.PolicyNamespace,
		Scope:           p.Scope,
		ForceDryRun:     forceDryRun,
	}
	if fs != nil && fs.Lookup("dry-run") != nil && fs.Changed("dry-run") {
		o.DryRunSet = true
		o.DryRun = p.DryRun
	}
	if fs != nil && fs.Lookup("notify") != nil && fs.Changed("notify") {
		o.NotifySet = true
		o.Notify = p.Notify
	}
	if fs != nil && fs.Lookup("notify-on-dry-run") != nil && fs.Changed("notify-on-dry-run") {
		o.NotifyOnDryRunSet = true
		o.NotifyOnDryRun = p.NotifyOnDryRun
	}
	return o
}

// Load merges file + env + flags into a resolved policy (file override only).
func (p *PolicyFlags) Load(fs *pflag.FlagSet, forceDryRun bool) (conf.Resolved, error) {
	return policy.LoadAndMerge(p.Overrides(fs, forceDryRun))
}

// LoadAll resolves file override or in-cluster ConfigMaps (multi-tenant).
func (p *PolicyFlags) LoadAll(ctx context.Context, fs *pflag.FlagSet, restCfg *rest.Config, forceDryRun bool) ([]conf.Resolved, error) {
	return policy.LoadPolicies(ctx, p.Overrides(fs, forceDryRun), restCfg)
}

// LiveBackend constructs a discover.LiveBackend from a REST config.
func LiveBackend(cfg *rest.Config) (*discover.LiveBackend, error) {
	return discover.NewLiveBackend(cfg)
}

// PrintSummary writes a human summary of a run to stderr.
func PrintSummary(w io.Writer, result cleanup.RunResult, pol conf.Resolved) {
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "policy=%s dryRun=%v scope=%s traces=%d plans=%d\n",
		pol.Name, pol.DryRun, pol.Deletion.Scope, len(result.Traces), len(result.Plans))
	for _, tr := range result.Traces {
		_, _ = fmt.Fprint(w, trace.RenderText(tr))
	}
	for _, p := range result.Plans {
		if len(p.BlockedBy) > 0 {
			_, _ = fmt.Fprintf(w, "plan %s blocked: %v\n", p.TraceID, p.BlockedBy)
			continue
		}
		_, _ = fmt.Fprintf(w, "plan %s steps=%d dryRun=%v\n", p.TraceID, len(p.Steps), p.DryRun)
		for i, s := range p.Steps {
			_, _ = fmt.Fprintf(w, "  %d. %s %s/%s\n", i+1, s.Action, s.Kind, s.Ref.Key())
		}
	}
}

// RejectNamespaceOnRunPlan enforces the locked rule that -n / --namespace is
// valid only on trace. Eligibility never comes from a CLI namespace flag.
func RejectNamespaceOnRunPlan(ns string) error {
	if ns == "" {
		return nil
	}
	return fmt.Errorf("--namespace / -n is not valid on run/plan; eligibility comes from policy labels/globs only (-n is allowed on trace)")
}
