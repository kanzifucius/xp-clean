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

Claims / LegacyCluster claim paths are not supported.

In-cluster policy comes from labelled ConfigMaps in the xp-clean namespace
(multi-tenant / per-team). Pass --config for an explicit file override.

--dry-run is a boolean planner dry-run (client-side DeletePlan only), not
kubectl API-server dry-run. --scope=all means definitions+instances; the
plan subcommand plans all eligible traces (not the same as scope=all).
--namespace / -n is valid only on trace.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := g.CompleteLogging(); err != nil {
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
		newTraceCommand(g),
		newManagerCommand(g),
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
