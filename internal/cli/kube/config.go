package kube

import (
	"fmt"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Options hold kubectl-style cluster access flags.
type Options struct {
	Kubeconfig string
	Context    string
	Cluster    string
	User       string
	Namespace  string
	Timeout    time.Duration
}

// RESTConfig builds a *rest.Config using clientcmd default loading rules.
// Honours KUBECONFIG when Kubeconfig is empty; supports in-cluster configs.
func RESTConfig(opts Options) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: opts.Context,
	}
	if opts.Cluster != "" {
		overrides.Context.Cluster = opts.Cluster
	}
	if opts.User != "" {
		overrides.Context.AuthInfo = opts.User
	}
	if opts.Namespace != "" {
		overrides.Context.Namespace = opts.Namespace
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		overrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}
	return cfg, nil
}
