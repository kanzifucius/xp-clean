// Package notify sends best-effort chat summaries of cleanup runs.
// Failures here must never change cleanup results or CLI exit codes.
package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EventKind is a run lifecycle notification kind.
type EventKind string

const (
	KindStarted  EventKind = "started"
	KindFinished EventKind = "finished"
	KindFailed   EventKind = "failed"
)

// Event is the shared chat payload (sink formatters map from this struct).
type Event struct {
	Kind       EventKind
	RunID      string
	Policy     string
	DryRun     bool
	Scope      string
	StartedAt  time.Time
	FinishedAt time.Time
	Traces     int
	Planned    int
	Deleted    int
	Skipped    int
	ErrorCount int
	Errors     []string // truncated snippets; never include secrets or object YAML
	Blocked    int
	LogURL     string // optional; filled by sink from its logURLTemplate when empty upstream
	Err        error  // hard RunOnce failure (KindFailed)
}

// Notifier delivers one Event to a chat sink.
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
}

// Noop is a silent notifier used when notifications are disabled or misconfigured.
type Noop struct{}

// Notify implements Notifier.
func (Noop) Notify(context.Context, Event) error { return nil }

// Multi fans out to several sinks, isolating per-sink errors.
type Multi struct {
	Sinks []Notifier
}

// Notify calls each sink; returns a joined error of sink failures (never panics).
func (m Multi) Notify(ctx context.Context, ev Event) error {
	var errs []error
	for i, s := range m.Sinks {
		if s == nil {
			continue
		}
		if err := safeNotify(ctx, s, ev); err != nil {
			errs = append(errs, fmt.Errorf("sink[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func safeNotify(ctx context.Context, n Notifier, ev Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("notify panic: %v", r)
		}
	}()
	return n.Notify(ctx, ev)
}

// TruncateErrors keeps at most max entries, each at most maxRunes runes.
func TruncateErrors(errs []string, max, maxRunes int) []string {
	if max <= 0 {
		return nil
	}
	if len(errs) > max {
		errs = errs[:max]
	}
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, TruncateRunes(e, maxRunes))
	}
	return out
}

// TruncateRunes shortens s to at most max runes.
func TruncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// ExpandLogURL replaces {{runId}} (case-insensitive) in template.
func ExpandLogURL(template, runID string) string {
	if strings.TrimSpace(template) == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"{{runId}}", runID,
		"{{runID}}", runID,
		"{{RunID}}", runID,
	)
	return replacer.Replace(template)
}

// RedactWebhookURL returns a log-safe form: scheme+host + last 4 path chars.
func RedactWebhookURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Avoid importing net/url just for redaction edge cases that fail parse.
	schemeHost := raw
	path := ""
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		slash := strings.Index(rest, "/")
		if slash < 0 {
			return raw[:i+3] + rest
		}
		schemeHost = raw[:i+3] + rest[:slash]
		path = rest[slash:]
	}
	if path == "" {
		return schemeHost
	}
	suffix := path
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	return schemeHost + "/…/" + suffix
}
