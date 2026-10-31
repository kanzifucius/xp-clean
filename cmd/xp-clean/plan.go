package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	clipolicy "github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func newPlanCommand(g *options.Global) *cobra.Command {
	var (
		pf     options.PolicyFlags
		output string
	)
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print DeletePlans for all eligible traces (always dry-run)",
		Long: `Plan builds DeletePlans for every eligible trace and prints them on stdout.
Always forces planner dry-run (no deletes). Loads labelled ConfigMaps from
--policy-namespace unless --config is set. --scope selects deletion.scope for
the plan contents; it does not mean "plan only definitions".
--fail-on-blocked exits 3 when any plan is blocked.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.RejectNamespaceOnRunPlan(g.Kube.Namespace); err != nil {
				return exit.UsageErr(err)
			}

			policies, cfg, err := loadPoliciesAndREST(cmd, g, &pf, true)
			if err != nil {
				return err
			}
			backend, err := options.LiveBackend(cfg)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("kubernetes client: %w", err))
			}

			runner := &cleanup.Runner{
				Backend: backend,
				Audit:   audit.NewLogger(os.Stderr),
				Out:     os.Stderr,
			}

			var allPlans []cleanup.DeletePlan
			for _, policy := range policies {
				g.Logger.Info("plan", "policy", policy.Name, "scope", policy.Deletion.Scope)
				result, err := runner.RunOnce(cmd.Context(), clipolicy.ToPolicyView(policy))
				if err != nil {
					return exit.RuntimeErr(fmt.Errorf("plan failed (%s): %w", policy.Name, err))
				}
				allPlans = append(allPlans, result.Plans...)
			}

			switch output {
			case "", "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(allPlans); err != nil {
					return exit.RuntimeErr(err)
				}
			case "yaml":
				b, err := yaml.Marshal(allPlans)
				if err != nil {
					return exit.RuntimeErr(err)
				}
				if _, err = cmd.OutOrStdout().Write(b); err != nil {
					return exit.RuntimeErr(err)
				}
			default:
				return exit.UsageErr(fmt.Errorf("unknown --output %q (want json|yaml)", output))
			}

			if pf.FailOnBlocked && cleanup.HasBlockedPlans(allPlans) {
				return exit.BlockedErr(fmt.Errorf("completed with blocked plans (--fail-on-blocked)"))
			}
			return nil
		},
	}
	pf.Bind(cmd.Flags(), false)
	cmd.Flags().StringVarP(&output, "output", "o", "json", "output format: json|yaml")
	return cmd
}
