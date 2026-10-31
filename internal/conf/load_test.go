package conf_test

import (
	"testing"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
)

func TestParseDefaults(t *testing.T) {
	raw := []byte(`
name: test-stacks
dryRun: true
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
deletion:
  scope: instances-only
`)
	r, err := conf.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !r.DryRun {
		t.Fatal("dryRun default/override")
	}
	if r.Deletion.Scope != cleanup.ScopeInstancesOnly {
		t.Fatalf("scope %q", r.Deletion.Scope)
	}
	if r.TTL.DefaultTTL == 0 {
		t.Fatal("expected default TTL")
	}
}

func TestParseRequiresSelectors(t *testing.T) {
	_, err := conf.Parse([]byte(`name: x`))
	if err == nil {
		t.Fatal("expected error for empty selectors")
	}
}

func TestParseScopeAll(t *testing.T) {
	r, err := conf.Parse([]byte(`
name: x
selectors:
  globs:
    xrdNames: ["*.dev.example.org"]
deletion:
  scope: all
  allowXRDDeletion: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Deletion.Scope != cleanup.ScopeAll || !r.Deletion.AllowXRDDeletion {
		t.Fatalf("%#v", r.Deletion)
	}
}
