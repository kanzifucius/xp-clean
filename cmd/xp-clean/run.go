package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	clipolicy "github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/spf13/cobra"
)

func newRunCommand(g *options.Global) *cobra.Command {
	var (
		pf   options.PolicyFlags
		once bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single cleanup pass (CronJob / Job friendly)",
		Long: `Run invokes the shared cleanup.Runner once and exits.

Dry-run defaults come from the policy file (default true) unless --dry-run is set.
--scope overrides deletion.scope (instances-only|all). Use manager for scheduled loops.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.RejectNamespaceOnRunPlan(g.Kube.Namespace); err != nil {
				return exit.UsageErr(err)
			}
			if !once {
				return exit.UsageErr(fmt.Errorf("only --once is supported in MVP; use manager for scheduled runs"))
			}

			policy, err := pf.Load(cmd.Flags(), false)
			if err != nil {
				return classifyConfigErr(err)
			}

			cfg, err := g.RESTConfig()
			if err != nil {
				return exit.RuntimeErr(err)
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
			g.Logger.Info("run once", "policy", policy.Name, "dryRun", policy.DryRun, "scope", policy.Deletion.Scope)
			result, err := runner.RunOnce(ctx, clipolicy.ToPolicyView(policy))
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("run failed: %w", err))
			}
			options.PrintSummary(cmd.ErrOrStderr(), result, policy)
			return nil
		},
	}
	cmd.Flags().BoolVar(&once, "once", true, "run a single pass and exit (required true in MVP)")
	pf.Bind(cmd.Flags(), true)
	return cmd
}
