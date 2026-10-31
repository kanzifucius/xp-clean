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
Always forces planner dry-run (no deletes). --scope selects deletion.scope for the plan
contents; it does not mean "plan only definitions".`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := options.RejectNamespaceOnRunPlan(g.Kube.Namespace); err != nil {
				return exit.UsageErr(err)
			}

			policy, err := pf.Load(cmd.Flags(), true)
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
				Audit:   audit.NewLogger(os.Stderr),
				Out:     os.Stderr,
			}
			g.Logger.Info("plan", "policy", policy.Name, "scope", policy.Deletion.Scope)
			result, err := runner.RunOnce(cmd.Context(), clipolicy.ToPolicyView(policy))
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("plan failed: %w", err))
			}

			switch output {
			case "", "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result.Plans)
			case "yaml":
				b, err := yaml.Marshal(result.Plans)
				if err != nil {
					return exit.RuntimeErr(err)
				}
				_, err = cmd.OutOrStdout().Write(b)
				return err
			default:
				return exit.UsageErr(fmt.Errorf("unknown --output %q (want json|yaml)", output))
			}
		},
	}
	pf.Bind(cmd.Flags(), false)
	cmd.Flags().StringVarP(&output, "output", "o", "json", "output format: json|yaml")
	return cmd
}
