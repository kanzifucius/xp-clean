package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
)

func TestFlagOverridesDryRunAndScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := []byte(`
name: t
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
deletion:
  scope: instances-only
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath: path,
		DryRunSet:  true,
		DryRun:     false,
		Scope:      "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.DryRun {
		t.Fatal("flag should force dryRun false")
	}
	if r.Deletion.Scope != cleanup.ScopeAll {
		t.Fatalf("scope %q", r.Deletion.Scope)
	}
}

func TestPlanForceDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: t
dryRun: false
selectors:
  globs:
    xrNames: ["test-*"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := policy.LoadAndMerge(policy.FlagOverrides{ConfigPath: path, ForceDryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.DryRun {
		t.Fatal("plan must force dry-run")
	}
}

func TestEnvConfigPath(t *testing.T) {
	t.Setenv("XP_CLEAN_CONFIG", "/tmp/from-env.yaml")
	got := policy.ResolveConfigPath("", "default.yaml")
	if got != "/tmp/from-env.yaml" {
		t.Fatalf("got %q", got)
	}
	got = policy.ResolveConfigPath("flag.yaml", "default.yaml")
	if got != "flag.yaml" {
		t.Fatalf("got %q", got)
	}
}

func TestNotifyOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: t
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
observability:
  notifications:
    enabled: false
    onDryRun: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath:        path,
		NotifySet:         true,
		Notify:            true,
		NotifyOnDryRunSet: true,
		NotifyOnDryRun:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Notifications.Enabled {
		t.Fatal("notify override")
	}
	if r.Notifications.OnDryRun {
		t.Fatal("onDryRun override")
	}
}
