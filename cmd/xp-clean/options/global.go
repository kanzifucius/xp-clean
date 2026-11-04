package options

import (
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/internal/cli/kube"
	"github.com/kanzifucius/xp-clean/internal/cli/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// Global holds root persistent flags shared by cluster-facing commands.
type Global struct {
	Kube      kube.Options
	Log       log.Options
	Verbosity int

	Logger logr.Logger
}

// NewGlobal returns Globals with quiet CLI defaults (error-only on stderr).
func NewGlobal() *Global {
	return &Global{
		Log: log.Options{Devel: false, Level: "error"},
	}
}

// BindRootFlags registers kubectl-style and logging persistent flags.
func (g *Global) BindRootFlags(fs *pflag.FlagSet) {
	fs.StringVar(&g.Kube.Kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: KUBECONFIG, ~/.kube/config, or in-cluster)")
	fs.StringVar(&g.Kube.Context, "context", "", "kubeconfig context name to use")
	fs.StringVar(&g.Kube.Cluster, "cluster", "", "kubeconfig cluster name override")
	fs.StringVar(&g.Kube.User, "user", "", "kubeconfig user (auth info) override")
	fs.StringVarP(&g.Kube.Namespace, "namespace", "n", "", "namespace for namespaced lookups on debug trace only (rejected on run/plan)")
	fs.DurationVar(&g.Kube.Timeout, "request-timeout", 0, "timeout for an individual Kubernetes API request (0 = client default)")

	fs.IntVarP(&g.Verbosity, "v", "v", 0, "log verbosity: 0=error-only (default), 1=info, 2+=debug")
	fs.StringVar(&g.Log.Level, "log-level", "error", "log level on stderr (debug|info|warn|error); default error for interactive CLI")
	fs.StringVar(&g.Log.Level, "zap-log-level", "error", "alias for --log-level")
	fs.BoolVar(&g.Log.Devel, "zap-devel", false, "development logging (console encoder, stacktraces on warn+)")
	fs.StringVar(&g.Log.Encoder, "zap-encoder", "", "log encoder (json|console); default depends on --zap-devel")
}

// CompleteLogging initialises the process logger and mutes client-go noise by default.
func (g *Global) CompleteLogging(cmd *cobra.Command) error {
	g.applyVerbosityDefaults(cmd)
	logger, _, err := log.Setup(g.Log)
	if err != nil {
		return err
	}
	g.Logger = logger
	log.ConfigureKubernetesLogging(logger, g.verbose())
	return nil
}

func (g *Global) applyVerbosityDefaults(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	levelChanged := flagChanged(cmd, "log-level") || flagChanged(cmd, "zap-log-level")
	vChanged := flagChanged(cmd, "v")

	switch {
	case levelChanged:
		// Explicit --log-level / --zap-log-level wins.
	case vChanged:
		g.Log.Level = log.LevelFromVerbosity(g.Verbosity)
	case isManagerCommand(cmd):
		// Long-running manager defaults a bit louder than one-shot CLI commands.
		g.Log.Level = "info"
	}
}

func flagChanged(cmd *cobra.Command, name string) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup(name); f != nil && f.Changed {
			return true
		}
		if f := c.PersistentFlags().Lookup(name); f != nil && f.Changed {
			return true
		}
	}
	return false
}

func (g *Global) verbose() bool {
	if g.Verbosity > 0 {
		return true
	}
	switch strings.ToLower(g.Log.Level) {
	case "debug", "info", "warn":
		return true
	default:
		return false
	}
}

func isManagerCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "manager" {
			return true
		}
	}
	return false
}

// RESTConfig builds a Kubernetes REST config from kube flags.
func (g *Global) RESTConfig() (*rest.Config, error) {
	return kube.RESTConfig(g.Kube)
}

// EnsureTimeout is a helper for tests / future use.
func EnsureTimeout(d time.Duration) time.Duration { return d }
