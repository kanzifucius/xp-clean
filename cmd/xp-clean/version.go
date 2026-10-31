package main

import (
	"fmt"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/kanzifucius/xp-clean/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print xp-clean version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := version.Format(output)
			if err != nil {
				return exit.UsageErr(err)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			if err != nil {
				return err
			}
			if output == "" || output == "short" {
				_, err = fmt.Fprintln(cmd.OutOrStdout())
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "short", "output format: short|json|yaml")
	return cmd
}
