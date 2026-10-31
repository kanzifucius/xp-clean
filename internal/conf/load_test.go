package conf_test

import (
	"strings"
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

func TestParseNotificationsDefaults(t *testing.T) {
	r, err := conf.Parse([]byte(`
name: x
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Notifications.Enabled {
		t.Fatal("notifications disabled by default")
	}
	if !r.Notifications.OnDryRun {
		t.Fatal("onDryRun default true")
	}
	if len(r.Notifications.NotifyOn) != 3 {
		t.Fatalf("notifyOn %#v", r.Notifications.NotifyOn)
	}
}

func TestParseNotificationsTeamsSink(t *testing.T) {
	r, err := conf.Parse([]byte(`
name: x
selectors:
  globs:
    xrNames: ["test-*"]
observability:
  notifications:
    enabled: true
    sinks:
      - type: teams
        logURLTemplate: "https://logs.example?q={{runId}}"
`))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Notifications.Enabled || len(r.Notifications.Sinks) != 1 {
		t.Fatalf("%#v", r.Notifications)
	}
	s := r.Notifications.Sinks[0]
	if s.Type != "teams" || s.WebhookURLFromEnv != "XP_CLEAN_TEAMS_WEBHOOK_URL" {
		t.Fatalf("%#v", s)
	}
	if s.LogURLTemplate == "" {
		t.Fatal("logURLTemplate")
	}
}

func TestParseNotificationsUnknownSink(t *testing.T) {
	_, err := conf.Parse([]byte(`
name: x
selectors:
  globs:
    xrNames: ["test-*"]
observability:
  notifications:
    enabled: true
    sinks:
      - type: discord
`))
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("got %v", err)
	}
}
