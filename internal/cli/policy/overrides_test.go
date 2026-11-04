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
dryRun: false
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
deletion:
  scope: instances-only
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// --dry-run=false / allow-live clears ForceDryRun but never overrides a
	// policy that still has dryRun: true. Here policy is already live.
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath: path,
		ForceDryRun: false,
		Scope:       "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.DryRun {
		t.Fatal("policy dryRun false + ForceDryRun false should stay live")
	}
	if r.Deletion.Scope != cleanup.ScopeAll {
		t.Fatalf("scope %q", r.Deletion.Scope)
	}
}

func TestApplyDoesNotOverridePolicyDryRunTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: t
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath:  path,
		ForceDryRun: false, // --apply
	})
	if err != nil {
		t.Fatal(err)
	}
	if !r.DryRun {
		t.Fatal("--apply must not force live when policy dryRun is true")
	}
}

func TestRunDefaultForceDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: t
dryRun: false
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath:  path,
		ForceDryRun: true, // run default without --apply
	})
	if err != nil {
		t.Fatal(err)
	}
	if !r.DryRun {
		t.Fatal("run without --apply must dry-run even when policy dryRun is false")
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
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath:   path,
		AlwaysDryRun: true,
		ForceDryRun:  true,
	})
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

func TestGVKGlobOverrideReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: t
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    gvks: ["apps.example.org/v1", "legacy.example.org/v1beta1"]
    xrNames: ["test-*"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// No flag → policy GVK globs unchanged; other globs stay.
	base, err := policy.LoadAndMerge(policy.FlagOverrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(base.Selectors.Globs.GVKs), 2; got != want {
		t.Fatalf("no-flag GVK count=%d want %d (%v)", got, want, base.Selectors.Globs.GVKs)
	}
	if got := base.Selectors.Globs.XRNames; len(got) != 1 || got[0] != "test-*" {
		t.Fatalf("xrNames should stay from policy: %v", got)
	}

	// Flag set → replace GVK globs entirely; labels / other globs untouched.
	over, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath: path,
		GVKGlobs:   []string{"*.dev.example.org/*", "team-b.example.org/v1*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantGVKs := []string{"*.dev.example.org/*", "team-b.example.org/v1*"}
	if len(over.Selectors.Globs.GVKs) != len(wantGVKs) {
		t.Fatalf("GVK globs=%v want %v", over.Selectors.Globs.GVKs, wantGVKs)
	}
	for i, w := range wantGVKs {
		if over.Selectors.Globs.GVKs[i] != w {
			t.Fatalf("GVK[%d]=%q want %q", i, over.Selectors.Globs.GVKs[i], w)
		}
	}
	// Confirm no merge with policy values.
	for _, leftover := range []string{"apps.example.org/v1", "legacy.example.org/v1beta1"} {
		for _, g := range over.Selectors.Globs.GVKs {
			if g == leftover {
				t.Fatalf("policy GVK %q must not remain after flag replace", leftover)
			}
		}
	}
	if over.Selectors.RequireLabels["xp-clean.io/eligible"] != "true" {
		t.Fatal("requireLabels must stay from policy")
	}
	if got := over.Selectors.Globs.XRNames; len(got) != 1 || got[0] != "test-*" {
		t.Fatalf("non-GVK globs must stay from policy: %v", got)
	}
}

func TestGVKGlobOverrideOnLabelOnlyPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`
name: skinny
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := policy.LoadAndMerge(policy.FlagOverrides{
		ConfigPath: path,
		GVKGlobs:   []string{"example.org/v1alpha1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Selectors.Globs.GVKs) != 1 || r.Selectors.Globs.GVKs[0] != "example.org/v1alpha1" {
		t.Fatalf("got %v", r.Selectors.Globs.GVKs)
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
