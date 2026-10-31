package options

import (
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
	ConfigPath string
	DryRun     bool
	Scope      string
}

// Bind registers --config / --dry-run / --scope on a command flag set.
func (p *PolicyFlags) Bind(fs *pflag.FlagSet, withDryRun bool) {
	fs.StringVar(&p.ConfigPath, "config", "", "path to CleanupPolicy YAML (default: XP_CLEAN_CONFIG or config/samples/cleanuppolicy.yaml)")
	if withDryRun {
		// Default false is irrelevant; use Changed("dry-run") to detect overrides.
		fs.BoolVar(&p.DryRun, "dry-run", false, "planner dry-run only (no deletes); when unset, use policy dryRun (default true). Not kubectl API-server dry-run")
	}
	fs.StringVar(&p.Scope, "scope", "", "deletion.scope override: instances-only|all (default from policy, then instances-only)")
}

// Load merges file + env + flags into a resolved policy.
func (p *PolicyFlags) Load(fs *pflag.FlagSet, forceDryRun bool) (conf.Resolved, error) {
	o := policy.FlagOverrides{
		ConfigPath:  p.ConfigPath,
		Scope:       p.Scope,
		ForceDryRun: forceDryRun,
	}
	if fs != nil && fs.Lookup("dry-run") != nil && fs.Changed("dry-run") {
		o.DryRunSet = true
		o.DryRun = p.DryRun
	}
	return policy.LoadAndMerge(o)
}

// LiveBackend constructs a discover.LiveBackend from a REST config.
func LiveBackend(cfg *rest.Config) (*discover.LiveBackend, error) {
	return discover.NewLiveBackend(cfg)
}

// PrintSummary writes a human summary of a run to stderr.
func PrintSummary(w io.Writer, result cleanup.RunResult, policy conf.Resolved) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "policy=%s dryRun=%v scope=%s traces=%d plans=%d\n",
		policy.Name, policy.DryRun, policy.Deletion.Scope, len(result.Traces), len(result.Plans))
	for _, tr := range result.Traces {
		fmt.Fprint(w, trace.RenderText(tr))
	}
	for _, p := range result.Plans {
		if len(p.BlockedBy) > 0 {
			fmt.Fprintf(w, "plan %s blocked: %v\n", p.TraceID, p.BlockedBy)
			continue
		}
		fmt.Fprintf(w, "plan %s steps=%d dryRun=%v\n", p.TraceID, len(p.Steps), p.DryRun)
		for i, s := range p.Steps {
			fmt.Fprintf(w, "  %d. %s %s/%s\n", i+1, s.Action, s.Kind, s.Ref.Key())
		}
	}
}

// RejectNamespaceOnRunPlan enforces open question Q7 option (c) for early phases:
// --namespace / -n is only meaningful on trace.
func RejectNamespaceOnRunPlan(ns string) error {
	if ns == "" {
		return nil
	}
	return fmt.Errorf("--namespace / -n is not supported on run/plan yet (use policy namespace globs; -n is allowed on trace)")
}
