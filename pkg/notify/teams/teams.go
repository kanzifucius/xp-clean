// Package teams posts Adaptive Cards to Microsoft Teams via Workflows webhooks.
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/notify"
)

const (
	defaultAttemptTimeout = 3 * time.Second
	defaultBudget         = 5 * time.Second
	maxRetries            = 2
)

// Sink POSTs Adaptive Card JSON to a Teams Workflows webhook URL.
type Sink struct {
	WebhookURL     string
	LogURLTemplate string
	HTTPClient     *http.Client
	Budget         time.Duration
}

// Notify implements notify.Notifier.
func (s *Sink) Notify(ctx context.Context, ev notify.Event) error {
	if s == nil || strings.TrimSpace(s.WebhookURL) == "" {
		return nil
	}
	if err := validateHTTPS(s.WebhookURL); err != nil {
		return err
	}
	if ev.LogURL == "" {
		ev.LogURL = notify.ExpandLogURL(s.LogURLTemplate, ev.RunID)
	}
	body, err := BuildPayload(ev)
	if err != nil {
		return err
	}

	budget := s.Budget
	if budget <= 0 {
		budget = defaultBudget
	}
	deadline := time.Now().Add(budget)
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: defaultAttemptTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if time.Now().After(deadline) {
			break
		}
		remain := time.Until(deadline)
		attemptCtx, cancel := context.WithTimeout(ctx, minDuration(defaultAttemptTimeout, remain))
		status, err := s.postOnce(attemptCtx, client, body)
		cancel()
		if err == nil && status >= 200 && status < 300 {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("teams webhook status %d", status)
		}
		if status > 0 && status < 500 && status != 429 {
			return lastErr
		}
		if attempt == maxRetries {
			break
		}
		backoff := time.Duration(200*(1<<attempt)) * time.Millisecond
		if status == 429 {
			// Honour Retry-After when present on a previous response is handled in postOnce via lastErr only;
			// use fixed backoff for MVP.
			backoff = 800 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(minDuration(backoff, time.Until(deadline))):
		}
	}
	return fmt.Errorf("teams notify: %w (url=%s)", lastErr, notify.RedactWebhookURL(s.WebhookURL))
}

func (s *Sink) postOnce(ctx context.Context, client *http.Client, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode, nil
}

func validateHTTPS(raw string) error {
	if !strings.HasPrefix(strings.ToLower(raw), "https://") {
		return fmt.Errorf("teams webhook URL must be https (got %s)", notify.RedactWebhookURL(raw))
	}
	return nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// BuildPayload builds the Workflows Adaptive Card wrapper JSON for ev.
func BuildPayload(ev notify.Event) ([]byte, error) {
	style := bannerStyle(ev)
	facts := primaryFacts(ev)

	body := []any{
		bannerBlock(ev, style),
		map[string]any{
			"type":    "Container",
			"spacing": "Medium",
			"items": []any{
				map[string]any{
					"type":  "FactSet",
					"facts": facts,
				},
			},
		},
	}
	body = append(body, objectListBlocks(ev)...)
	if summary := subtleSummary(ev); summary != "" {
		body = append(body, map[string]any{
			"type":     "TextBlock",
			"text":     summary,
			"wrap":     true,
			"isSubtle": true,
			"spacing":  "Medium",
		})
	}

	card := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    body,
		"msteams": map[string]any{
			"width": "Full",
		},
	}
	if ev.LogURL != "" {
		card["actions"] = []any{
			map[string]any{
				"type":  "Action.OpenUrl",
				"title": "Open logs",
				"url":   ev.LogURL,
			},
		}
	}

	payload := map[string]any{
		"type": "message",
		"attachments": []any{
			map[string]any{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"contentUrl":  nil,
				"content":     card,
			},
		},
	}
	return json.Marshal(payload)
}

// bannerBlock is a full-bleed status header so the card pops in the Teams feed.
func bannerBlock(ev notify.Event, style string) map[string]any {
	items := []any{
		map[string]any{
			"type":   "TextBlock",
			"size":   "Small",
			"weight": "Bolder",
			"text":   "XP-CLEAN",
			"color":  "Light",
		},
		map[string]any{
			"type":    "TextBlock",
			"size":    "Large",
			"weight":  "Bolder",
			"text":    titleFor(ev),
			"color":   "Light",
			"wrap":    true,
			"spacing": "None",
		},
		map[string]any{
			"type":     "TextBlock",
			"text":     bannerSubtitle(ev),
			"wrap":     true,
			"color":    "Light",
			"isSubtle": true,
			"spacing":  "Small",
		},
	}
	return map[string]any{
		"type":    "Container",
		"style":   style,
		"bleed":   true,
		"spacing": "None",
		"items":   items,
	}
}

func bannerSubtitle(ev notify.Event) string {
	policy := emptyDash(ev.Policy)
	result := resultLabel(ev)
	scope := emptyDash(ev.Scope)
	return fmt.Sprintf("%s · %s · scope %s", policy, result, scope)
}

func primaryFacts(ev notify.Event) []map[string]string {
	facts := []map[string]string{
		{"title": "Duration", "value": durationLabel(ev)},
		{"title": "Traces", "value": strconv.Itoa(ev.Traces)},
		{"title": "Planned", "value": strconv.Itoa(ev.Planned)},
	}
	switch {
	case ev.Kind == notify.KindStarted:
		facts = append(facts, map[string]string{"title": "Dry-run", "value": strconv.FormatBool(ev.DryRun)})
	case ev.DryRun:
		facts = append(facts,
			map[string]string{"title": "Would delete", "value": strconv.Itoa(ev.Skipped)},
			map[string]string{"title": "Blocked", "value": strconv.Itoa(ev.Blocked)},
		)
	default:
		facts = append(facts,
			map[string]string{"title": "Deleted", "value": strconv.Itoa(ev.Deleted)},
			map[string]string{"title": "Skipped", "value": strconv.Itoa(ev.Skipped)},
			map[string]string{"title": "Blocked", "value": strconv.Itoa(ev.Blocked)},
		)
	}
	if ev.ErrorCount > 0 || ev.Kind == notify.KindFailed {
		facts = append(facts, map[string]string{"title": "Errors", "value": strconv.Itoa(ev.ErrorCount)})
	}
	facts = append(facts, map[string]string{"title": "Run ID", "value": emptyDash(ev.RunID)})
	return facts
}

func titleFor(ev notify.Event) string {
	switch ev.Kind {
	case notify.KindStarted:
		return "Run started"
	case notify.KindFailed:
		return "Run failed"
	default:
		if ev.DryRun {
			return "Dry-run finished"
		}
		if ev.ErrorCount > 0 {
			return "Run finished with errors"
		}
		return "Run finished"
	}
}

// bannerStyle picks an Adaptive Card container style for the status banner.
func bannerStyle(ev notify.Event) string {
	switch ev.Kind {
	case notify.KindFailed:
		return "attention"
	case notify.KindStarted:
		return "accent"
	default:
		if ev.ErrorCount > 0 {
			return "warning"
		}
		if ev.DryRun {
			return "emphasis"
		}
		return "good"
	}
}

func resultLabel(ev notify.Event) string {
	switch ev.Kind {
	case notify.KindStarted:
		return "started"
	case notify.KindFailed:
		return "failed"
	default:
		if ev.ErrorCount > 0 {
			return "partial"
		}
		if ev.DryRun {
			return "dry-run"
		}
		return "success"
	}
}

func durationLabel(ev notify.Event) string {
	if ev.FinishedAt.IsZero() || ev.StartedAt.IsZero() {
		return "—"
	}
	d := ev.FinishedAt.Sub(ev.StartedAt).Round(time.Second)
	if d < time.Second {
		d = time.Second
	}
	return d.String()
}

func subtleSummary(ev notify.Event) string {
	if ev.Kind == notify.KindFailed && ev.Err != nil {
		return notify.TruncateRunes(ev.Err.Error(), 500)
	}
	if ev.DryRun && ev.Kind == notify.KindFinished {
		return "No objects deleted (dry-run)."
	}
	if len(ev.Errors) > 0 {
		return notify.TruncateRunes(strings.Join(ev.Errors, "; "), 500)
	}
	return ""
}

// objectListBlocks adds Kind ns/name — reason rows for finished/failed cards.
// Started stays lean (no list). Cap matches notify.MaxObjectsInEvent; overflow
// is summarised as "… and N more" (full detail via Open logs / audit JSON).
func objectListBlocks(ev notify.Event) []any {
	if ev.Kind == notify.KindStarted || len(ev.Objects) == 0 {
		return nil
	}
	heading := objectsHeading(ev)
	items := []any{
		map[string]any{
			"type":   "TextBlock",
			"text":   heading,
			"weight": "Bolder",
			"size":   "Medium",
			"wrap":   true,
		},
	}
	for _, o := range ev.Objects {
		items = append(items, map[string]any{
			"type":    "TextBlock",
			"text":    "• " + notify.FormatObjectLine(o),
			"wrap":    true,
			"spacing": "Small",
			"fontType": "Monospace",
		})
	}
	total := ev.ObjectsTotal
	if total <= 0 {
		total = len(ev.Objects)
	}
	if omitted := total - len(ev.Objects); omitted > 0 {
		more := fmt.Sprintf("… and %d more", omitted)
		if ev.LogURL != "" {
			more += " (see Open logs)"
		}
		items = append(items, map[string]any{
			"type":     "TextBlock",
			"text":     more,
			"wrap":     true,
			"isSubtle": true,
			"spacing":  "Small",
		})
	}
	return []any{
		map[string]any{
			"type":    "Container",
			"style":   "emphasis",
			"spacing": "Medium",
			"items":   items,
		},
	}
}

func objectsHeading(ev notify.Event) string {
	switch {
	case ev.Kind == notify.KindFailed:
		return "Planned / attempted"
	case ev.DryRun:
		return "Would delete"
	default:
		return "Deleted"
	}
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
