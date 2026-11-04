package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	g := options.NewGlobal()

	cmd := &cobra.Command{
		Use:   rootUse(),
		Short: "Discover and clean Crossplane composition trees (XR-only / v2-native)",
		Long: `xp-clean discovers Crossplane v2-native composition stacks, matches them via
labels and/or globs, builds parent→child traces, and deletes leftovers on a
schedule.

Day-to-day: "xp-clean run --once" prints a dry-run plan of stacks matched /
would delete (roots mode by default). Live deletes need --apply and policy
dryRun: false. Full composition traces are opt-in via --mode=trace.

Claims / LegacyCluster claim paths are not supported.

In-cluster policy comes from labelled ConfigMaps in the xp-clean namespace
(multi-tenant / per-team). Pass --config for an explicit file override.

--dry-run is a boolean planner dry-run (client-side DeletePlan only), not
kubectl API-server dry-run. Prefer --apply for the live switch. Live Deletes
use foreground propagation; --wait is opt-in. --scope=all means
definitions+instances. --namespace / -n is valid only on debug trace.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := g.CompleteLogging(cmd); err != nil {
				return exit.UsageErr(err)
			}
			return nil
		},
	}

	g.BindRootFlags(cmd.PersistentFlags())
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	cmd.AddCommand(
		newRunCommand(g),
		newPlanCommand(g),
		newDebugCommand(g),
		newHiddenRootTraceCommand(g),
		newManagerCommand(g),
		newSkillCommand(),
		newVersionCommand(),
	)

	return cmd
}

func rootUse() string {
	base := filepath.Base(os.Args[0])
	if strings.HasPrefix(base, "kubectl-") {
		name := strings.TrimPrefix(base, "kubectl-")
		name = strings.ReplaceAll(name, "-", " ")
		return "kubectl " + name
	}
	return "xp-clean"
}

func execute() int {
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return exit.Status(err)
	}
	return exit.Success
}
