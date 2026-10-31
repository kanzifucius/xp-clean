package main

import (
	"encoding/json"
	"fmt"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func newTraceCommand(g *options.Global) *cobra.Command {
	var (
		name    string
		group   string
		version string
		kind    string
		output  string
	)
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Print one composition tree from an XR",
		Long: `Trace walks spec.crossplane.resourceRefs from a root XR and prints the DAG.
Use --name namespace/name, or a bare name with --namespace / -n.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" || group == "" || kind == "" {
				return exit.UsageErr(fmt.Errorf("trace requires --name, --group, and --kind"))
			}
			ns, n, err := discover.ParseNamespacedName(name)
			if err != nil {
				return exit.UsageErr(err)
			}
			if ns == "" {
				ns = g.Kube.Namespace
			}
			// Namespaced XRs need a namespace; leave empty for cluster-scoped attempts.

			cfg, err := g.RESTConfig()
			if err != nil {
				return exit.RuntimeErr(err)
			}
			backend, err := options.LiveBackend(cfg)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("kubernetes client: %w", err))
			}

			gvr, err := backend.GVR(group, version, kind)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("map GVK: %w", err))
			}
			u, err := backend.Dyn.Get(cmd.Context(), gvr, ns, n)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("get XR: %w", err))
			}
			tr, err := backend.BuildTrace(cmd.Context(), u)
			if err != nil {
				return exit.RuntimeErr(fmt.Errorf("trace: %w", err))
			}

			switch output {
			case "", "text":
				_, err = fmt.Fprint(cmd.OutOrStdout(), trace.RenderText(tr))
				return err
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(tr)
			case "yaml":
				b, err := yaml.Marshal(tr)
				if err != nil {
					return exit.RuntimeErr(err)
				}
				_, err = cmd.OutOrStdout().Write(b)
				return err
			default:
				return exit.UsageErr(fmt.Errorf("unknown --output %q (want text|json|yaml)", output))
			}
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "XR name or namespace/name")
	cmd.Flags().StringVar(&group, "group", "", "XR API group")
	cmd.Flags().StringVar(&version, "version", "v1alpha1", "XR API version")
	cmd.Flags().StringVar(&kind, "kind", "", "XR kind")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "output format: text|json|yaml")
	return cmd
}
