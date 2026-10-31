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
	"github.com/kanzifucius/xp-clean/internal/controller"
	"github.com/kanzifucius/xp-clean/internal/version"
	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/metrics"
	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/spf13/cobra"
)

func newManagerCommand(g *options.Global) *cobra.Command {
	var pf options.PolicyFlags

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Run the scheduled cleanup manager (stub schedule until controller-runtime)",
		Long: `Manager loads a CleanupPolicy file and runs the shared cleanup.Runner on an
interval derived from policy.schedule. Full controller-runtime (leader election,
health probes, CR watches) is deferred; flags for those will land with the CRD.
Each scheduled RunOnce uses the same notification hooks as run --once.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.RejectNamespaceOnRunPlan(g.Kube.Namespace); err != nil {
				return exit.UsageErr(err)
			}

			policy, err := pf.Load(cmd.Flags(), false)
			if err != nil {
				return classifyConfigErr(err)
			}

			notifier, err := clinotify.Build(policy.Notifications)
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

			runner := &cleanup.Runner{
				Backend: backend,
				Audit:   audit.NewLogger(os.Stdout),
				Out:     os.Stdout,
			}
			runFn := func(ctx context.Context, p conf.Resolved) error {
				meta := notify.RunMeta{
					Policy: p.Name,
					DryRun: p.DryRun,
					Scope:  string(p.Deletion.Scope),
				}
				_, err := notify.AroundRun(ctx, notifier, meta, func(ctx context.Context) (cleanup.RunResult, error) {
					return runner.RunOnce(ctx, clipolicy.ToPolicyView(p))
				}, func(nerr error) {
					g.Logger.Info("notify warning", "err", nerr)
				})
				return err
			}
			_ = &controller.CleanupPolicyReconciler{Policy: policy, Run: runFn}
			sched := controller.NewScheduler(policy, runFn, nil)

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			g.Logger.Info("manager starting", "version", version.Version, "schedule", policy.Schedule, "policy", policy.Name, "notify", policy.Notifications.Enabled)
			if err := sched.Start(ctx); err != nil && err != context.Canceled {
				return exit.RuntimeErr(fmt.Errorf("manager: %w", err))
			}
			g.Logger.Info("manager stopped", "metrics", metrics.Read())
			return nil
		},
	}
	pf.Bind(cmd.Flags(), true)
	return cmd
}
