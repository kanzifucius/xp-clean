package main

import (
	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/spf13/cobra"
)

func newDebugCommand(g *options.Global) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug helpers (composition tree inspection)",
		Long: `Debug groups low-level inspection commands that are not part of the
day-to-day cleanup path. Prefer "xp-clean run --once" to see stacks matched
and what would be deleted.`,
	}
	cmd.AddCommand(newTraceCommand(g))
	return cmd
}
