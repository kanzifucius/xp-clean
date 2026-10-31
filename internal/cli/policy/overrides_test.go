package policy_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/cli/policy"
	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
	got := policy.ResolveConfigPath("")
	if got != "/tmp/from-env.yaml" {
		t.Fatalf("got %q", got)
	}
	got = policy.ResolveConfigPath("flag.yaml")
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

func TestNoImplicitSamplePath(t *testing.T) {
	t.Setenv("XP_CLEAN_CONFIG", "")
	if got := policy.ResolveConfigPath(""); got != "" {
		t.Fatalf("expected empty path without --config/env, got %q", got)
	}
	_, err := policy.LoadAndMerge(policy.FlagOverrides{})
	if err == nil {
		t.Fatal("expected error without policy source")
	}
}

func TestResolvePolicyNamespace(t *testing.T) {
	t.Setenv("XP_CLEAN_POLICY_NAMESPACE", "")
	if got := policy.ResolvePolicyNamespace(""); got != conf.DefaultPolicyNamespace {
		t.Fatalf("got %q", got)
	}
	t.Setenv("XP_CLEAN_POLICY_NAMESPACE", "custom")
	if got := policy.ResolvePolicyNamespace(""); got != "custom" {
		t.Fatalf("got %q", got)
	}
	if got := policy.ResolvePolicyNamespace("flag-ns"); got != "flag-ns" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadPoliciesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: file-pol
selectors:
  globs:
    xrNames: ["test-*"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := policy.LoadPolicies(context.Background(), policy.FlagOverrides{ConfigPath: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "file-pol" {
		t.Fatalf("%#v", got)
	}
}

func TestLoadPoliciesRequiresSourceWithoutCluster(t *testing.T) {
	t.Setenv("XP_CLEAN_CONFIG", "")
	_, err := policy.LoadPolicies(context.Background(), policy.FlagOverrides{}, nil)
	if err == nil {
		t.Fatal("expected missing source error")
	}
}

// Ensure LoadFromConfigMaps path stays wired for multi-tenant samples.
func TestConfigMapListerRoundTrip(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
			Namespace: "xp-clean",
			Labels:    map[string]string{conf.PolicyLabelKey: conf.PolicyLabelValue},
		},
		Data: map[string]string{
			conf.PolicyDataKey: `
name: p1
selectors:
  globs:
    gvks: ["apps.example.org/v1"]
`,
		},
	})
	got, err := conf.LoadFromConfigMaps(context.Background(), conf.NewClientsetLister(client), "xp-clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
}
