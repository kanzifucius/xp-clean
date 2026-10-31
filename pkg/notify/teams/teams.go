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
	title := titleFor(ev)
	colour := colourFor(ev)
	facts := []map[string]string{
		{"title": "Policy", "value": emptyDash(ev.Policy)},
		{"title": "Result", "value": resultLabel(ev)},
		{"title": "Dry-run", "value": strconv.FormatBool(ev.DryRun)},
		{"title": "Scope", "value": emptyDash(ev.Scope)},
		{"title": "Duration", "value": durationLabel(ev)},
		{"title": "Traces", "value": strconv.Itoa(ev.Traces)},
		{"title": "Planned deletes", "value": strconv.Itoa(ev.Planned)},
		{"title": "Deleted", "value": strconv.Itoa(ev.Deleted)},
		{"title": "Skipped", "value": strconv.Itoa(ev.Skipped)},
		{"title": "Blocked", "value": strconv.Itoa(ev.Blocked)},
		{"title": "Errors", "value": strconv.Itoa(ev.ErrorCount)},
		{"title": "Run ID", "value": emptyDash(ev.RunID)},
	}

	body := []any{
		map[string]any{
			"type":   "TextBlock",
			"size":   "Medium",
			"weight": "Bolder",
			"text":   title,
			"color":  colour,
		},
		map[string]any{
			"type":  "FactSet",
			"facts": facts,
		},
	}
	if summary := subtleSummary(ev); summary != "" {
		body = append(body, map[string]any{
			"type":     "TextBlock",
			"text":     summary,
			"wrap":     true,
			"isSubtle": true,
		})
	}

	card := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    body,
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

func titleFor(ev notify.Event) string {
	switch ev.Kind {
	case notify.KindStarted:
		return "xp-clean: run started"
	case notify.KindFailed:
		return "xp-clean: run failed"
	default:
		return "xp-clean: run finished"
	}
}

func colourFor(ev notify.Event) string {
	switch ev.Kind {
	case notify.KindFailed:
		return "Attention"
	case notify.KindStarted:
		return "Accent"
	default:
		if ev.ErrorCount > 0 {
			return "Warning"
		}
		return "Good"
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

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
