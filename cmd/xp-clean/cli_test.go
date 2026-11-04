package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanzifucius/xp-clean/cmd/xp-clean/options"
	"github.com/kanzifucius/xp-clean/internal/cli/exit"
	clipolicy "github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/spf13/pflag"
)

func writePolicy(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	err := os.WriteFile(path, []byte(`
name: test
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
deletion:
  scope: instances-only
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRootHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"run", "plan", "debug", "manager", "version", "completion", "--log-level", "-v"} {
		if !strings.Contains(s, want) {
			t.Fatalf("help missing %q:\n%s", want, s)
		}
	}
	// Root trace is hidden; happy-path help must not advertise it as a primary command.
	if strings.Contains(s, "  trace ") {
		t.Fatalf("root help must not list hidden trace as a primary command:\n%s", s)
	}
	if strings.Contains(s, "kubectl-xpclean") {
		t.Fatal("standalone docs only; kubectl-xpclean must not appear in help")
	}
}

func TestDebugTraceHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"debug", "trace", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"--name", "--group", "--kind", "--namespace"} {
		if !strings.Contains(s, want) {
			t.Fatalf("debug trace help missing %q:\n%s", want, s)
		}
	}
}

func TestRunHelpMentionsApply(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"--apply", "--mode", "--wait", "fail-on-blocked", "ConfigMap", "dry-run", "--gvk-glob"} {
		if !strings.Contains(s, want) {
			t.Fatalf("run help missing %q:\n%s", want, s)
		}
	}
}

func TestPlanHelpMentionsGVKGlob(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"plan", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--gvk-glob") {
		t.Fatalf("plan help missing --gvk-glob:\n%s", out.String())
	}
}

func TestGVKGlobFlagReplacesPolicyViaLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: test
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    gvks: ["from-policy.example.org/v1"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var pf options.PolicyFlags
	fs := pflag.NewFlagSet("test-gvk", pflag.ContinueOnError)
	pf.Bind(fs, true)
	if err := fs.Parse([]string{
		"--config", path,
		"--gvk-glob", "*.dev.example.org/*",
		"--gvk-glob", "example.org/v1alpha1",
	}); err != nil {
		t.Fatal(err)
	}
	r, err := clipolicy.LoadAndMerge(pf.Overrides(fs, false))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"*.dev.example.org/*", "example.org/v1alpha1"}
	if len(r.Selectors.Globs.GVKs) != len(want) {
		t.Fatalf("got %v want %v", r.Selectors.Globs.GVKs, want)
	}
	for i := range want {
		if r.Selectors.Globs.GVKs[i] != want[i] {
			t.Fatalf("got %v want %v", r.Selectors.Globs.GVKs, want)
		}
	}
	for _, g := range r.Selectors.Globs.GVKs {
		if g == "from-policy.example.org/v1" {
			t.Fatal("policy GVK must be replaced, not merged")
		}
	}

	// Omitted flag keeps policy globs.
	fs2 := pflag.NewFlagSet("test-gvk-omit", pflag.ContinueOnError)
	var pf2 options.PolicyFlags
	pf2.Bind(fs2, true)
	if err := fs2.Parse([]string{"--config", path}); err != nil {
		t.Fatal(err)
	}
	r2, err := clipolicy.LoadAndMerge(pf2.Overrides(fs2, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Selectors.Globs.GVKs) != 1 || r2.Selectors.Globs.GVKs[0] != "from-policy.example.org/v1" {
		t.Fatalf("omitted flag should keep policy globs, got %v", r2.Selectors.Globs.GVKs)
	}
}

func TestVersionShort(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"version"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0.1.0-dev") {
		t.Fatalf("got %q", out.String())
	}
}

func TestVersionJSON(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"version", "-o", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"version"`) {
		t.Fatalf("got %q", out.String())
	}
}

func TestInvalidScopeExitUsage(t *testing.T) {
	path := writePolicy(t)
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run", "--once", "--config", path, "--scope", "claims"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if got := exit.Status(err); got != exit.Usage {
		t.Fatalf("exit status %d for %v", got, err)
	}
	if !strings.Contains(err.Error(), "scope") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestRunRejectsNamespace(t *testing.T) {
	path := writePolicy(t)
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run", "--once", "-n", "xp-test-1", "--config", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if exit.Status(err) != exit.Usage {
		t.Fatalf("want usage exit, got %d (%v)", exit.Status(err), err)
	}
	if !strings.Contains(err.Error(), "not valid on run/plan") {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(err.Error(), "debug trace") {
		t.Fatalf("expected debug trace hint, got: %v", err)
	}
}

func TestPlanRejectsNamespace(t *testing.T) {
	path := writePolicy(t)
	cmd := newRootCommand()
	cmd.SetArgs([]string{"plan", "-n", "xp-test-1", "--config", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if exit.Status(err) != exit.Usage {
		t.Fatalf("want usage exit, got %d (%v)", exit.Status(err), err)
	}
}

func TestRunOnceFalseUsage(t *testing.T) {
	path := writePolicy(t)
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run", "--once=false", "--config", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if exit.Status(err) != exit.Usage {
		t.Fatalf("want usage, got %d (%v)", exit.Status(err), err)
	}
}

func TestMissingConfigUsage(t *testing.T) {
	t.Setenv("XP_CLEAN_CONFIG", "")
	cmd := newRootCommand()
	// No --config and no cluster ConfigMaps → usage (policy source required).
	// REST config may also fail; either way must not succeed.
	cmd.SetArgs([]string{"run", "--once", "--kubeconfig", filepath.Join(t.TempDir(), "missing")})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error without policy source")
	}
}

func TestFailOnBlockedExitCode(t *testing.T) {
	plans := []cleanup.DeletePlan{{
		TraceID:   "t1",
		BlockedBy: []string{"crossplane.io/paused (deletePaused=false)"},
	}}
	if !cleanup.HasBlockedPlans(plans) {
		t.Fatal("expected blocked")
	}
	err := exit.BlockedErr(errString("completed with blocked plans (--fail-on-blocked)"))
	if exit.Status(err) != exit.Blocked {
		t.Fatalf("want exit 3, got %d", exit.Status(err))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestCompletionBash(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"completion", "bash"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() < 100 {
		t.Fatalf("unexpected completion output length %d", out.Len())
	}
}

func TestPlanDeprecated(t *testing.T) {
	cmd := newRootCommand()
	plan, _, err := cmd.Find([]string{"plan"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Deprecated == "" {
		t.Fatal("plan should be marked deprecated")
	}
}
