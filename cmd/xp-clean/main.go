package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/internal/controller"
	"github.com/kanzifucius/xp-clean/internal/version"
	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/metrics"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "plan":
		os.Exit(cmdPlan(os.Args[2:]))
	case "trace":
		os.Exit(cmdTrace(os.Args[2:]))
	case "manager":
		os.Exit(cmdManager(os.Args[2:]))
	case "version":
		fmt.Println(version.Version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `xp-clean — discover and clean Crossplane composition trees (XR-only / v2-native)

Usage:
  xp-clean run --once [--config PATH] [--dry-run] [--scope instances-only|all]
  xp-clean plan [--config PATH] [--scope instances-only|all]
  xp-clean trace --name namespace/name [--config PATH]
  xp-clean manager [--config PATH]
  xp-clean version

Dry-run is the default for run/plan unless the config sets dryRun: false
and you pass --dry-run=false (or omit override while config disables it).

Claims / LegacyCluster claim paths are not supported.
`)
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	once := fs.Bool("once", true, "run a single pass and exit")
	configPath := fs.String("config", "config/samples/cleanuppolicy.yaml", "path to policy YAML")
	dryRun := fs.String("dry-run", "", "override dry-run (true|false); empty keeps config default")
	scope := fs.String("scope", "", "override deletion.scope (instances-only|all)")
	kubeconfig := fs.String("kubeconfig", "", "path to kubeconfig (defaults to in-cluster or ~/.kube/config)")
	_ = fs.Parse(args)
	if !*once {
		fmt.Fprintln(os.Stderr, "only --once is supported in MVP; use manager for scheduled runs")
		return 2
	}

	policy, err := conf.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}
	if err := applyOverrides(&policy, *dryRun, *scope); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}

	backend, err := liveBackend(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubernetes client: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := &cleanup.Runner{
		Backend: backend,
		Audit:   audit.NewLogger(os.Stdout),
		Out:     os.Stdout,
	}
	result, err := runner.RunOnce(ctx, toPolicyView(policy))
	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		return 1
	}
	printSummary(result, policy)
	return 0
}

func cmdPlan(args []string) int {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	configPath := fs.String("config", "config/samples/cleanuppolicy.yaml", "path to policy YAML")
	scope := fs.String("scope", "", "override deletion.scope")
	kubeconfig := fs.String("kubeconfig", "", "path to kubeconfig")
	_ = fs.Parse(args)

	policy, err := conf.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}
	policy.DryRun = true
	if err := applyOverrides(&policy, "true", *scope); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}

	backend, err := liveBackend(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubernetes client: %v\n", err)
		return 1
	}
	ctx := context.Background()
	runner := &cleanup.Runner{Backend: backend, Audit: audit.NewLogger(os.Stderr), Out: os.Stderr}
	result, err := runner.RunOnce(ctx, toPolicyView(policy))
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result.Plans)
	return 0
}

func cmdTrace(args []string) int {
	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	name := fs.String("name", "", "XR namespace/name")
	group := fs.String("group", "", "XR API group")
	version := fs.String("version", "v1alpha1", "XR API version")
	kind := fs.String("kind", "", "XR kind")
	kubeconfig := fs.String("kubeconfig", "", "path to kubeconfig")
	_ = fs.Parse(args)
	if *name == "" || *group == "" || *kind == "" {
		fmt.Fprintln(os.Stderr, "trace requires --name, --group, and --kind")
		return 2
	}
	ns, n, err := discover.ParseNamespacedName(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}
	backend, err := liveBackend(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubernetes client: %v\n", err)
		return 1
	}
	ctx := context.Background()
	gvr, err := backend.GVR(*group, *version, *kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "map GVK: %v\n", err)
		return 1
	}
	u, err := backend.Dyn.Get(ctx, gvr, ns, n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get XR: %v\n", err)
		return 1
	}
	tr, err := backend.BuildTrace(ctx, u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "trace: %v\n", err)
		return 1
	}
	fmt.Print(trace.RenderText(tr))
	return 0
}

func cmdManager(args []string) int {
	fs := flag.NewFlagSet("manager", flag.ExitOnError)
	configPath := fs.String("config", "config/samples/cleanuppolicy.yaml", "path to policy YAML")
	kubeconfig := fs.String("kubeconfig", "", "path to kubeconfig")
	_ = fs.Parse(args)

	policy, err := conf.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}
	backend, err := liveBackend(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubernetes client: %v\n", err)
		return 1
	}
	runner := &cleanup.Runner{Backend: backend, Audit: audit.NewLogger(os.Stdout), Out: os.Stdout}
	runFn := func(ctx context.Context, p conf.Resolved) error {
		_, err := runner.RunOnce(ctx, toPolicyView(p))
		return err
	}
	rec := &controller.CleanupPolicyReconciler{Policy: policy, Run: runFn}
	sched := controller.NewScheduler(policy, runFn, nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	_ = rec // reconciler stub available for future CRD watch
	fmt.Fprintf(os.Stderr, "xp-clean manager %s starting (schedule stub interval from %q)\n", version.Version, policy.Schedule)
	if err := sched.Start(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "manager: %v\n", err)
		return 1
	}
	snap := metrics.Read()
	fmt.Fprintf(os.Stderr, "metrics snapshot: %+v\n", snap)
	return 0
}

func liveBackend(kubeconfig string) (*discover.LiveBackend, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	return discover.NewLiveBackend(cfg)
}

func applyOverrides(policy *conf.Resolved, dryRun, scope string) error {
	switch dryRun {
	case "":
	case "true", "1", "yes":
		policy.DryRun = true
	case "false", "0", "no":
		policy.DryRun = false
	default:
		return fmt.Errorf("invalid --dry-run %q", dryRun)
	}
	if scope != "" {
		s, err := cleanup.ParseScope(scope)
		if err != nil {
			return err
		}
		policy.Deletion.Scope = s
	}
	return nil
}

func toPolicyView(p conf.Resolved) cleanup.PolicyView {
	return cleanup.PolicyView{
		Name:      p.Name,
		DryRun:    p.DryRun,
		Selectors: p.Selectors,
		TTL:       p.TTL,
		Deletion:  p.Deletion,
	}
}

func printSummary(result cleanup.RunResult, policy conf.Resolved) {
	fmt.Fprintf(os.Stderr, "policy=%s dryRun=%v scope=%s traces=%d plans=%d\n",
		policy.Name, policy.DryRun, policy.Deletion.Scope, len(result.Traces), len(result.Plans))
	for _, tr := range result.Traces {
		fmt.Fprint(os.Stderr, trace.RenderText(tr))
	}
	for _, p := range result.Plans {
		if len(p.BlockedBy) > 0 {
			fmt.Fprintf(os.Stderr, "plan %s blocked: %v\n", p.TraceID, p.BlockedBy)
			continue
		}
		fmt.Fprintf(os.Stderr, "plan %s steps=%d dryRun=%v\n", p.TraceID, len(p.Steps), p.DryRun)
		for i, s := range p.Steps {
			fmt.Fprintf(os.Stderr, "  %d. %s %s/%s\n", i+1, s.Action, s.Kind, s.Ref.Key())
		}
	}
}
