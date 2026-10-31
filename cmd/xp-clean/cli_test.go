package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/cli/exit"
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
	for _, want := range []string{"run", "plan", "trace", "manager", "version", "completion"} {
		if !strings.Contains(s, want) {
			t.Fatalf("help missing %q:\n%s", want, s)
		}
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
