package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	clinotify "github.com/kanzifucius/xp-clean/internal/cli/notify"
	clipolicy "github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
)

func newRunCommand(g *options.Global) *cobra.Command {
	var (
		pf   options.PolicyFlags
		once bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Day-to-day cleanup: dry-run plan by default; live deletes with --apply",
		Long: `Run is the primary day-to-day command. It discovers eligible stacks
(highest-parent roots by default) and prints a dry-run plan of what would be
deleted. Use --mode=trace for full parent→child composition trees.

Live deletes require both --apply (or compatibility --dry-run=false) and
policy dryRun: false. Without --apply, run always plans only — even when the
policy document already has dryRun: false. Live Deletes use foreground
propagation; --wait opts into poll-until-gone (default is no wait).

Policies load from labelled ConfigMaps in --policy-namespace (default xp-clean)
unless --config points at an explicit file override. --scope overrides
deletion.scope (instances-only|all). --gvk-glob (repeatable) replaces
selectors.globs.gvks when set. --fail-on-blocked exits 3 when any plan
is blocked. Optional Teams/Slack notifications wrap each RunOnce and never
change cleanup exit codes. Use manager for scheduled loops.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.RejectNamespaceOnRunPlan(g.Kube.Namespace); err != nil {
				return exit.UsageErr(err)
			}
			if !once {
				return exit.UsageErr(fmt.Errorf("only --once is supported in MVP; use manager for scheduled runs"))
			}

			policies, cfg, err := loadPoliciesAndREST(cmd, g, &pf, false)
			if err != nil {
				return err
			}
			backend, err := options.LiveBackend(cfg)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("kubernetes client: %w", err))
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			runner := &cleanup.Runner{
				Backend: backend,
				Audit:   audit.NewLogger(os.Stdout),
				Out:     os.Stdout,
			}

			var merged cleanup.RunResult
			for _, policy := range policies {
				notifier, err := clinotify.Build(policy.Notifications)
				if err != nil {
					return classifyConfigErr(err)
				}
				g.Logger.Info("run once", "policy", policy.Name, "dryRun", policy.DryRun, "scope", policy.Deletion.Scope, "notify", policy.Notifications.Enabled)
				meta := notify.RunMeta{
					Policy: policy.Name,
					DryRun: policy.DryRun,
					Scope:  string(policy.Deletion.Scope),
				}
				result, err := notify.AroundRun(ctx, notifier, meta, func(ctx context.Context) (cleanup.RunResult, error) {
					return runner.RunOnce(ctx, clipolicy.ToPolicyView(policy))
				}, func(nerr error) {
					g.Logger.Info("notify warning", "err", nerr)
				})
				if err != nil {
					return exit.RuntimeErr(fmt.Errorf("run failed (%s): %w", policy.Name, err))
				}
				options.PrintSummary(cmd.ErrOrStderr(), result, policy)
				merged = cleanup.MergeRunResults(merged, result)
			}

			if pf.FailOnBlocked && cleanup.HasBlockedPlans(merged.Plans) {
				return exit.BlockedErr(fmt.Errorf("completed with blocked plans (--fail-on-blocked)"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&once, "once", true, "run a single pass and exit (required true in MVP)")
	pf.Bind(cmd.Flags(), true)
	return cmd
}

// loadPoliciesAndREST loads file policies before touching kubeconfig when
// --config / XP_CLEAN_CONFIG is set, so usage errors (bad --scope, etc.) win
// over missing kubeconfig. ConfigMap mode needs REST first.
// alwaysDryRun is true for plan (never live).
func loadPoliciesAndREST(cmd *cobra.Command, g *options.Global, pf *options.PolicyFlags, alwaysDryRun bool) ([]conf.Resolved, *rest.Config, error) {
	o := pf.Overrides(cmd.Flags(), alwaysDryRun)
	path := clipolicy.ResolveConfigPath(o.ConfigPath)

	if path != "" {
		policies, err := clipolicy.LoadPolicies(cmd.Context(), o, nil)
		if err != nil {
			return nil, nil, classifyConfigErr(err)
		}
		cfg, err := g.RESTConfig()
		if err != nil {
			return nil, nil, exit.RuntimeErr(err)
		}
		return policies, cfg, nil
	}

	cfg, err := g.RESTConfig()
	if err != nil {
		return nil, nil, exit.RuntimeErr(err)
	}
	policies, err := clipolicy.LoadPolicies(cmd.Context(), o, cfg)
	if err != nil {
		return nil, nil, classifyConfigErr(err)
	}
	return policies, cfg, nil
}
