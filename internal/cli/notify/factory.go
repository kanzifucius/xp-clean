// Package notify builds Notifier instances from resolved policy + environment.
package notify

import (
	"fmt"
	"os"
	"strings"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/kanzifucius/xp-clean/pkg/notify/teams"
)

// Build returns a filtering Multi notifier for the resolved notifications config.
// Empty / missing webhook env ⇒ that sink is skipped (noop for that sink).
// A configured Slack webhook URL fails Build until phase 2 lands.
func Build(n conf.NotificationsResolved) (notify.Notifier, error) {
	filter := notify.Filter{
		Enabled:                  n.Enabled,
		NotifyOn:                 n.NotifyOn,
		OnDryRun:                 n.OnDryRun,
		OnSuccessWithZeroDeletes: n.OnSuccessWithZeroDeletes,
		OnPartialErrors:          n.OnPartialErrors,
	}
	if !n.Enabled {
		return notify.Filtering{Inner: notify.Noop{}, Filter: filter}, nil
	}

	var sinks []notify.Notifier
	for i, s := range n.Sinks {
		url := strings.TrimSpace(os.Getenv(s.WebhookURLFromEnv))
		switch s.Type {
		case "teams":
			if url == "" {
				continue
			}
			sinks = append(sinks, &teams.Sink{
				WebhookURL:     url,
				LogURLTemplate: s.LogURLTemplate,
			})
		case "slack":
			if url == "" {
				continue
			}
			return nil, fmt.Errorf("observability.notifications.sinks[%d]: slack sink is not implemented yet (Teams Workflows is the MVP sink)", i)
		default:
			return nil, fmt.Errorf("observability.notifications.sinks[%d]: unknown type %q", i, s.Type)
		}
	}

	inner := notify.Notifier(notify.Noop{})
	switch len(sinks) {
	case 0:
		// enabled but no usable webhook — noop
	case 1:
		inner = sinks[0]
	default:
		inner = notify.Multi{Sinks: sinks}
	}
	return notify.Filtering{Inner: inner, Filter: filter}, nil
}
