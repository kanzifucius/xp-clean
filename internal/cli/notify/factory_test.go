package notify_test

import (
	"testing"

	clinotify "github.com/kanzifucius/xp-clean/internal/cli/notify"
	"github.com/kanzifucius/xp-clean/internal/conf"
)

func TestBuildDisabledNoop(t *testing.T) {
	n, err := clinotify.Build(conf.NotificationsResolved{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if n == nil {
		t.Fatal("expected notifier")
	}
}

func TestBuildTeamsFromEnv(t *testing.T) {
	t.Setenv("XP_CLEAN_TEAMS_WEBHOOK_URL", "https://example.logic.azure.com/workflows/x")
	n, err := clinotify.Build(conf.NotificationsResolved{
		Enabled:  true,
		NotifyOn: []string{"started", "finished", "failed"},
		OnDryRun: true,
		Sinks: []conf.SinkFile{{
			Type:              "teams",
			WebhookURLFromEnv: "XP_CLEAN_TEAMS_WEBHOOK_URL",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == nil {
		t.Fatal("expected notifier")
	}
}

func TestBuildSlackConfiguredErrors(t *testing.T) {
	t.Setenv("XP_CLEAN_SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/x")
	_, err := clinotify.Build(conf.NotificationsResolved{
		Enabled: true,
		Sinks: []conf.SinkFile{{
			Type:              "slack",
			WebhookURLFromEnv: "XP_CLEAN_SLACK_WEBHOOK_URL",
		}},
	})
	if err == nil {
		t.Fatal("expected slack not implemented error")
	}
}
