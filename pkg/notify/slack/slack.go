// Package slack is a phase-2 stub for Slack Incoming Webhooks + Block Kit.
// The sink type is accepted in policy schema; the implementation lands later.
package slack

import (
	"context"
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/notify"
)

// Sink is a placeholder Notifier for the slack sink type.
type Sink struct {
	WebhookURL     string
	LogURLTemplate string
}

// Notify returns a clear not-implemented error so misconfiguration fails loudly
// rather than silently dropping Slack events.
func (s *Sink) Notify(context.Context, notify.Event) error {
	return fmt.Errorf("slack sink is not implemented yet (schema-ready; use teams for MVP)")
}
