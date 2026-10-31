package options

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/internal/cli/kube"
	"github.com/kanzifucius/xp-clean/internal/cli/log"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// Global holds root persistent flags shared by cluster-facing commands.
type Global struct {
	Kube kube.Options
	Log  log.Options

	Logger logr.Logger
}

// NewGlobal returns Globals with safe defaults.
func NewGlobal() *Global {
	return &Global{
		Log: log.Options{Devel: true, Level: "info"},
	}
}

// BindRootFlags registers kubectl-style and logging persistent flags.
func (g *Global) BindRootFlags(fs *pflag.FlagSet) {
	fs.StringVar(&g.Kube.Kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: KUBECONFIG, ~/.kube/config, or in-cluster)")
	fs.StringVar(&g.Kube.Context, "context", "", "kubeconfig context name to use")
	fs.StringVar(&g.Kube.Cluster, "cluster", "", "kubeconfig cluster name override")
	fs.StringVar(&g.Kube.User, "user", "", "kubeconfig user (auth info) override")
	fs.StringVarP(&g.Kube.Namespace, "namespace", "n", "", "namespace for namespaced lookups on trace only (rejected on run/plan)")
	fs.DurationVar(&g.Kube.Timeout, "request-timeout", 0, "timeout for an individual Kubernetes API request (0 = client default)")

	fs.BoolVar(&g.Log.Devel, "zap-devel", true, "development logging (console encoder, stacktraces on warn+)")
	fs.StringVar(&g.Log.Level, "zap-log-level", "info", "log level (debug|info|warn|error)")
	fs.StringVar(&g.Log.Encoder, "zap-encoder", "", "log encoder (json|console); default depends on --zap-devel")
}

// CompleteLogging initialises the process logger.
func (g *Global) CompleteLogging() error {
	logger, _, err := log.Setup(g.Log)
	if err != nil {
		return err
	}
	g.Logger = logger
	return nil
}

// RESTConfig builds a Kubernetes REST config from kube flags.
func (g *Global) RESTConfig() (*rest.Config, error) {
	return kube.RESTConfig(g.Kube)
}

// EnsureTimeout is a helper for tests / future use.
func EnsureTimeout(d time.Duration) time.Duration { return d }
