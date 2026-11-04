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
	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// PolicyFlags are product flags shared by run/plan/manager.
type PolicyFlags struct {
	ConfigPath      string
	PolicyNamespace string
	DryRun          bool
	Apply           bool
	Mode            string
	Wait            bool
	Scope           string
	FailOnBlocked   bool
	Notify          bool
	NotifyOnDryRun  bool
	GVKGlobs        []string
}

// Bind registers --config / --dry-run / --scope / ConfigMap / notify flags on a command.
// withLiveControls adds --apply and --dry-run (run/manager). Plan omits them and always dry-runs.
func (p *PolicyFlags) Bind(fs *pflag.FlagSet, withLiveControls bool) {
	fs.StringVar(&p.ConfigPath, "config", "", "path to policy YAML file override (explicit; no implicit sample path). When unset, load labelled ConfigMaps from --policy-namespace")
	fs.StringVar(&p.PolicyNamespace, "policy-namespace", "", "namespace for labelled policy ConfigMaps (default: xp-clean or XP_CLEAN_POLICY_NAMESPACE)")
	if withLiveControls {
		// Default false is irrelevant; use Changed("dry-run") / Apply to detect overrides.
		fs.BoolVar(&p.DryRun, "dry-run", false, "compatibility: --dry-run=true forces a dry-run plan; --dry-run=false allows live deletes only when policy dryRun is false (prefer --apply). Not kubectl API-server dry-run")
		fs.BoolVar(&p.Apply, "apply", false, "allow live deletes when the policy also has dryRun: false (happy-path live switch; default is dry-run plan)")
	}
	fs.StringVar(&p.Mode, "mode", "", "deletion.mode override: roots|trace (default from policy, then roots)")
	fs.BoolVar(&p.Wait, "wait", false, "wait for each delete to complete (honours deletion.waitTimeout / stuck-finalizer audit); default is no wait")
	fs.StringVar(&p.Scope, "scope", "", "deletion.scope override: instances-only|all (default from policy, then instances-only)")
	fs.StringArrayVar(&p.GVKGlobs, "gvk-glob", nil, "replace selectors.globs.gvks with this group/version glob (repeatable; when set, policy GVK globs are ignored)")
	fs.BoolVar(&p.FailOnBlocked, "fail-on-blocked", false, "exit 3 when the run completed with blocked plans")
	fs.BoolVar(&p.Notify, "notify", false, "override observability.notifications.enabled (webhook URL still from env)")
	fs.BoolVar(&p.NotifyOnDryRun, "notify-on-dry-run", true, "override observability.notifications.onDryRun")
}

// Overrides builds FlagOverrides from flags (including dry-run / --apply Changed state).
// alwaysDryRun is true for plan (never cleared by env or --apply).
// For run/manager, alwaysDryRun is false and ForceDryRun defaults to true unless
// --apply or --dry-run=false requests live (policy dryRun: false still required).
func (p *PolicyFlags) Overrides(fs *pflag.FlagSet, alwaysDryRun bool) policy.FlagOverrides {
	o := policy.FlagOverrides{
		ConfigPath:      p.ConfigPath,
		PolicyNamespace: p.PolicyNamespace,
		Scope:           p.Scope,
		Mode:            p.Mode,
		GVKGlobs:        append([]string(nil), p.GVKGlobs...),
		AlwaysDryRun:    alwaysDryRun,
		// Day-to-day default: dry-run plan. Cleared by --apply / --dry-run=false
		// (policy dryRun: false still required for live). Plan sets AlwaysDryRun.
		ForceDryRun: true,
	}
	if fs != nil && fs.Lookup("dry-run") != nil && fs.Changed("dry-run") {
		o.LiveControlsFromFlags = true
		if p.DryRun {
			o.DryRunSet = true
			o.DryRun = true
			o.ForceDryRun = true
		} else {
			// --dry-run=false: allow live if policy permits; never override policy to false.
			o.ForceDryRun = false
		}
	} else if fs != nil && fs.Lookup("apply") != nil && fs.Changed("apply") {
		o.LiveControlsFromFlags = true
		o.ForceDryRun = !p.Apply
	}
	if fs != nil && fs.Lookup("wait") != nil && fs.Changed("wait") {
		o.WaitSet = true
		o.Wait = p.Wait
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

// SummaryOptions controls how PrintSummary renders run results.
type SummaryOptions struct {
	// Verbose restores full trace dumps and per-stack step listings.
	Verbose bool
}

// PrintSummary writes a human summary of a run.
//
// Quiet default (roots + instances-only, no -v): one policy line, a would-delete
// count, and a flat Kind namespace/name list. Trace trees and "stack … steps=N"
// blocks are omitted when every plan is a single root XR delete.
//
// Richer output returns for mode=trace, scope=all, multi-step plans, or Verbose.
func PrintSummary(w io.Writer, result cleanup.RunResult, pol conf.Resolved, opts SummaryOptions) {
	if w == nil {
		w = os.Stderr
	}
	mode := pol.Deletion.Mode
	if mode == "" {
		mode = cleanup.ModeRoots
	}
	scope := pol.Deletion.Scope
	if scope == "" {
		scope = cleanup.ScopeInstancesOnly
	}

	_, _ = fmt.Fprintf(w, "policy=%s dryRun=%v mode=%s scope=%s\n",
		pol.Name, pol.DryRun, mode, scope)

	showTraces := opts.Verbose || mode == cleanup.ModeTrace
	detailStacks := opts.Verbose || mode == cleanup.ModeTrace || scope == cleanup.ScopeAll

	if showTraces {
		for _, tr := range result.Traces {
			_, _ = fmt.Fprint(w, trace.RenderText(tr))
		}
	}

	var (
		compactSteps []cleanup.DeleteStep
		compactDry   = true
		sawCompact   bool
		detailed     []cleanup.DeletePlan
		blocked      []cleanup.DeletePlan
	)
	for _, p := range result.Plans {
		if len(p.BlockedBy) > 0 {
			blocked = append(blocked, p)
			continue
		}
		if detailStacks || len(p.Steps) != 1 {
			detailed = append(detailed, p)
			continue
		}
		if !sawCompact {
			compactDry = p.DryRun
			sawCompact = true
		}
		compactSteps = append(compactSteps, p.Steps...)
	}

	if sawCompact || (!detailStacks && len(detailed) == 0 && len(blocked) == 0) {
		verb := "would delete"
		if sawCompact && !compactDry {
			verb = "deleting"
		}
		_, _ = fmt.Fprintf(w, "%s %d\n", verb, len(compactSteps))
		for _, s := range compactSteps {
			_, _ = fmt.Fprintf(w, "  %s\n", formatStepObject(s))
		}
	}

	for _, p := range blocked {
		_, _ = fmt.Fprintf(w, "stack %s blocked: %v\n", p.TraceID, p.BlockedBy)
	}
	for _, p := range detailed {
		verb := "would delete"
		if !p.DryRun {
			verb = "deleting"
		}
		_, _ = fmt.Fprintf(w, "stack %s %s steps=%d dryRun=%v\n", p.TraceID, verb, len(p.Steps), p.DryRun)
		for i, s := range p.Steps {
			_, _ = fmt.Fprintf(w, "  %d. %s %s\n", i+1, s.Action, formatStepObject(s))
		}
	}
}

// formatStepObject renders Kind namespace/name — reason using the shared notify
// formatter so quiet CLI and Teams/Slack Event object lines stay consistent.
func formatStepObject(s cleanup.DeleteStep) string {
	kind := s.Ref.Kind
	if kind == "" {
		kind = string(s.Kind)
	}
	return notify.FormatObjectLine(notify.ObjectSummary{
		Kind:      kind,
		Namespace: s.Ref.Namespace,
		Name:      s.Ref.Name,
		Reason:    s.Reason,
	})
}

// RejectNamespaceOnRunPlan enforces the locked rule that -n / --namespace is
// valid only on debug trace. Eligibility never comes from a CLI namespace flag.
func RejectNamespaceOnRunPlan(ns string) error {
	if ns == "" {
		return nil
	}
	return fmt.Errorf("--namespace / -n is not valid on run/plan; eligibility comes from policy labels/globs only (-n is allowed on debug trace)")
}
