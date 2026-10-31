package conf

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/match"
	"gopkg.in/yaml.v3"
)

// Policy is the MVP configuration surface (file or ConfigMap data).
// Mirrors CleanupPolicy.spec from the architecture plan.
type Policy struct {
	Name          string            `json:"name" yaml:"name"`
	Schedule      string            `json:"schedule" yaml:"schedule"`
	DryRun        *bool             `json:"dryRun" yaml:"dryRun"`
	Timezone      string            `json:"timezone" yaml:"timezone"`
	Selectors     match.Selectors   `json:"selectors" yaml:"selectors"`
	TTL           TTLFile           `json:"ttl" yaml:"ttl"`
	Deletion      DeletionFile      `json:"deletion" yaml:"deletion"`
	Observability ObservabilityFile `json:"observability" yaml:"observability"`
}

// TTLFile is YAML-friendly TTL config with duration strings.
type TTLFile struct {
	RespectResourceTTL       *bool  `json:"respectResourceTTL" yaml:"respectResourceTTL"`
	DefaultTTL               string `json:"defaultTTL" yaml:"defaultTTL"`
	ApplyDefaultTTLIfMissing bool   `json:"applyDefaultTTLIfMissing" yaml:"applyDefaultTTLIfMissing"`
}

// DeletionFile is YAML-friendly deletion config.
type DeletionFile struct {
	Scope            string `json:"scope" yaml:"scope"`
	MaxTracesPerRun  int    `json:"maxTracesPerRun" yaml:"maxTracesPerRun"`
	MaxDeletesPerRun int    `json:"maxDeletesPerRun" yaml:"maxDeletesPerRun"`
	OrphanMode       string `json:"orphanMode" yaml:"orphanMode"`
	DeletePaused     bool   `json:"deletePaused" yaml:"deletePaused"`
	AllowXRDDeletion bool   `json:"allowXRDDeletion" yaml:"allowXRDDeletion"`
	WaitTimeout      string `json:"waitTimeout" yaml:"waitTimeout"`
}

// ObservabilityFile holds logging verbosity and optional chat notifications.
// Audit retention sinks (ConfigMap / object storage) are deferred — stdout JSON only for now.
type ObservabilityFile struct {
	Verbosity     string            `json:"verbosity" yaml:"verbosity"`
	Notifications NotificationsFile `json:"notifications" yaml:"notifications"`
}

// NotificationsFile is the non-secret chat notification block under observability.
// Webhook URLs are never inlined — sinks name an env var via webhookURLFromEnv.
type NotificationsFile struct {
	Enabled                  *bool      `json:"enabled" yaml:"enabled"`
	NotifyOn                 []string   `json:"notifyOn" yaml:"notifyOn"`
	OnDryRun                 *bool      `json:"onDryRun" yaml:"onDryRun"`
	OnSuccessWithZeroDeletes *bool      `json:"onSuccessWithZeroDeletes" yaml:"onSuccessWithZeroDeletes"`
	OnPartialErrors          *bool      `json:"onPartialErrors" yaml:"onPartialErrors"`
	Sinks                    []SinkFile `json:"sinks" yaml:"sinks"`
}

// SinkFile is one chat sink entry (multi-sink schema from day one).
type SinkFile struct {
	Type              string `json:"type" yaml:"type"` // teams | slack
	WebhookURLFromEnv string `json:"webhookURLFromEnv" yaml:"webhookURLFromEnv"`
	LogURLTemplate    string `json:"logURLTemplate" yaml:"logURLTemplate"`
}

// NotificationsResolved is runtime notification settings after defaults.
type NotificationsResolved struct {
	Enabled                  bool
	NotifyOn                 []string
	OnDryRun                 bool
	OnSuccessWithZeroDeletes bool
	OnPartialErrors          bool
	Sinks                    []SinkFile
}

// Resolved is the runtime policy after defaults and parsing.
type Resolved struct {
	Name          string
	Schedule      string
	DryRun        bool
	Timezone      string
	Selectors     match.Selectors
	TTL           match.TTLConfig
	Deletion      cleanup.DeletionConfig
	Observability ObservabilityFile
	Notifications NotificationsResolved
}

// LoadFile reads a policy YAML file.
func LoadFile(path string) (Resolved, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Resolved{}, err
	}
	return Parse(data)
}

// Parse unmarshals and resolves defaults.
func Parse(data []byte) (Resolved, error) {
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Resolved{}, fmt.Errorf("parse policy: %w", err)
	}
	return Resolve(p)
}

// Resolve applies MVP defaults.
func Resolve(p Policy) (Resolved, error) {
	r := Resolved{
		Name:          p.Name,
		Schedule:      p.Schedule,
		Timezone:      p.Timezone,
		Selectors:     p.Selectors,
		Observability: p.Observability,
	}
	if r.Name == "" {
		r.Name = "default"
	}
	if r.Schedule == "" {
		r.Schedule = "0 */6 * * *"
	}
	if r.Timezone == "" {
		r.Timezone = "UTC"
	}
	r.DryRun = true
	if p.DryRun != nil {
		r.DryRun = *p.DryRun
	}

	ttl := match.TTLConfig{
		RespectResourceTTL:       true,
		ApplyDefaultTTLIfMissing: p.TTL.ApplyDefaultTTLIfMissing,
	}
	if p.TTL.RespectResourceTTL != nil {
		ttl.RespectResourceTTL = *p.TTL.RespectResourceTTL
	}
	if p.TTL.DefaultTTL != "" {
		d, err := time.ParseDuration(p.TTL.DefaultTTL)
		if err != nil {
			return Resolved{}, fmt.Errorf("ttl.defaultTTL: %w", err)
		}
		ttl.DefaultTTL = d
	} else {
		ttl.DefaultTTL = 168 * time.Hour
	}
	r.TTL = ttl

	del := cleanup.DefaultDeletionConfig()
	if p.Deletion.Scope != "" {
		scope, err := cleanup.ParseScope(p.Deletion.Scope)
		if err != nil {
			return Resolved{}, err
		}
		del.Scope = scope
	}
	if p.Deletion.MaxTracesPerRun > 0 {
		del.MaxTracesPerRun = p.Deletion.MaxTracesPerRun
	}
	if p.Deletion.MaxDeletesPerRun > 0 {
		del.MaxDeletesPerRun = p.Deletion.MaxDeletesPerRun
	}
	if p.Deletion.OrphanMode != "" {
		del.OrphanMode = p.Deletion.OrphanMode
	}
	del.DeletePaused = p.Deletion.DeletePaused
	del.AllowXRDDeletion = p.Deletion.AllowXRDDeletion
	if p.Deletion.WaitTimeout != "" {
		del.WaitTimeout = p.Deletion.WaitTimeout
	}
	r.Deletion = del

	notif, err := resolveNotifications(p.Observability.Notifications)
	if err != nil {
		return Resolved{}, err
	}
	r.Notifications = notif

	if len(r.Selectors.RequireLabels) == 0 && !hasGlobs(r.Selectors) {
		return Resolved{}, fmt.Errorf("selectors must include requireLabels and/or globs")
	}
	return r, nil
}

func resolveNotifications(n NotificationsFile) (NotificationsResolved, error) {
	out := NotificationsResolved{
		Enabled:                  false,
		NotifyOn:                 []string{"started", "finished", "failed"},
		OnDryRun:                 true,
		OnSuccessWithZeroDeletes: false,
		OnPartialErrors:          true,
		Sinks:                    nil,
	}
	if n.Enabled != nil {
		out.Enabled = *n.Enabled
	}
	if n.NotifyOn != nil {
		out.NotifyOn = append([]string(nil), n.NotifyOn...)
	}
	if n.OnDryRun != nil {
		out.OnDryRun = *n.OnDryRun
	}
	if n.OnSuccessWithZeroDeletes != nil {
		out.OnSuccessWithZeroDeletes = *n.OnSuccessWithZeroDeletes
	}
	if n.OnPartialErrors != nil {
		out.OnPartialErrors = *n.OnPartialErrors
	}
	for _, kind := range out.NotifyOn {
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "started", "finished", "failed":
		default:
			return NotificationsResolved{}, fmt.Errorf("observability.notifications.notifyOn: unknown event %q (want started|finished|failed)", kind)
		}
	}
	for i, s := range n.Sinks {
		typ := strings.ToLower(strings.TrimSpace(s.Type))
		switch typ {
		case "teams", "slack":
			// ok — teams implemented; slack accepted for schema readiness
		case "":
			return NotificationsResolved{}, fmt.Errorf("observability.notifications.sinks[%d]: type is required", i)
		default:
			return NotificationsResolved{}, fmt.Errorf("observability.notifications.sinks[%d]: unknown type %q (want teams|slack)", i, s.Type)
		}
		sink := s
		sink.Type = typ
		if sink.WebhookURLFromEnv == "" {
			switch sink.Type {
			case "teams":
				sink.WebhookURLFromEnv = "XP_CLEAN_TEAMS_WEBHOOK_URL"
			case "slack":
				sink.WebhookURLFromEnv = "XP_CLEAN_SLACK_WEBHOOK_URL"
			}
		}
		out.Sinks = append(out.Sinks, sink)
	}
	return out, nil
}

func hasGlobs(s match.Selectors) bool {
	g := s.Globs
	return len(g.Namespaces) > 0 || len(g.XRDNames) > 0 || len(g.XRNames) > 0 || len(g.GVKs) > 0
}
