package teams_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/kanzifucius/xp-clean/pkg/notify/teams"
)

func TestBuildPayloadGoldenShape(t *testing.T) {
	ev := notify.Event{
		Kind:       notify.KindFinished,
		RunID:      "01JTEST",
		Policy:     "test-stacks",
		DryRun:     true,
		Scope:      "instances-only",
		StartedAt:  time.Unix(0, 0).UTC(),
		FinishedAt: time.Unix(12, 0).UTC(),
		Traces:     3,
		Planned:    14,
		Deleted:    0,
		Skipped:    14,
		Objects: []notify.ObjectSummary{
			{APIVersion: "example.org/v1", Kind: "XDemo", Namespace: "demo", Name: "app-a", Reason: "instance_root; label:xp-clean.io/eligible=true+ttl_elapsed"},
			{APIVersion: "example.org/v1", Kind: "XDemo", Namespace: "demo", Name: "app-b", Reason: "instance_root; globs+expires-at"},
		},
		ObjectsTotal: 16,
		LogURL:       "https://logs.example/search?q=01JTEST",
	}
	raw, err := teams.BuildPayload(ev)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "message" {
		t.Fatalf("type=%v", payload["type"])
	}
	atts, ok := payload["attachments"].([]any)
	if !ok || len(atts) != 1 {
		t.Fatalf("attachments=%v", payload["attachments"])
	}
	att := atts[0].(map[string]any)
	if att["contentType"] != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("contentType=%v", att["contentType"])
	}
	content := att["content"].(map[string]any)
	if content["type"] != "AdaptiveCard" {
		t.Fatalf("card type=%v", content["type"])
	}
	s := string(raw)
	for _, want := range []string{
		"XP-CLEAN",
		"Dry-run finished",
		"test-stacks",
		`"style":"emphasis"`,
		`"bleed":true`,
		`"width":"Full"`,
		"01JTEST",
		"Open logs",
		"No objects deleted",
		"Would delete",
		"XDemo demo/app-a — instance_root; label:xp-clean.io/eligible=true+ttl_elapsed",
		"XDemo demo/app-b — instance_root; globs+expires-at",
		"… and 14 more (see Open logs)",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload missing %q:\n%s", want, s)
		}
	}
}

func TestBuildPayloadBannerStyles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		ev    notify.Event
		style string
		title string
	}{
		{
			name:  "failed attention",
			ev:    notify.Event{Kind: notify.KindFailed, Policy: "p"},
			style: "attention",
			title: "Run failed",
		},
		{
			name:  "started accent",
			ev:    notify.Event{Kind: notify.KindStarted, Policy: "p"},
			style: "accent",
			title: "Run started",
		},
		{
			name:  "live success good",
			ev:    notify.Event{Kind: notify.KindFinished, Policy: "p"},
			style: "good",
			title: "Run finished",
		},
		{
			name:  "partial warning",
			ev:    notify.Event{Kind: notify.KindFinished, Policy: "p", ErrorCount: 2},
			style: "warning",
			title: "Run finished with errors",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw, err := teams.BuildPayload(tt.ev)
			if err != nil {
				t.Fatal(err)
			}
			s := string(raw)
			if !strings.Contains(s, `"style":"`+tt.style+`"`) {
				t.Fatalf("missing banner style %q in:\n%s", tt.style, s)
			}
			if !strings.Contains(s, tt.title) {
				t.Fatalf("missing title %q in:\n%s", tt.title, s)
			}
		})
	}
}

func TestBuildPayloadStartedOmitsObjectList(t *testing.T) {
	raw, err := teams.BuildPayload(notify.Event{
		Kind:    notify.KindStarted,
		RunID:   "r-start",
		Policy:  "p",
		DryRun:  true,
		Objects: []notify.ObjectSummary{{Kind: "XDemo", Name: "should-not-appear", Reason: "instance_root; labels"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "Would delete") || strings.Contains(s, "should-not-appear") {
		t.Fatalf("started card must stay lean:\n%s", s)
	}
}

func TestBuildPayloadLiveDeletedHeading(t *testing.T) {
	raw, err := teams.BuildPayload(notify.Event{
		Kind: notify.KindFinished,
		Objects: []notify.ObjectSummary{
			{Kind: "XDemo", Namespace: "ns", Name: "gone", Reason: "instance_root; label:xp-clean.io/eligible=true+default_ttl"},
		},
		ObjectsTotal: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "Deleted") || !strings.Contains(s, "XDemo ns/gone — instance_root; label:xp-clean.io/eligible=true+default_ttl") {
		t.Fatalf("live finished payload:\n%s", s)
	}
	if strings.Contains(s, "Would delete") {
		t.Fatal("live finished must not use Would delete")
	}
}

func TestSinkPostsJSONAndRetries429(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "AdaptiveCard") {
			t.Errorf("body=%s", body)
		}
		if hits.Load() == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	// httptest is http:// — bypass https check by using a custom client path:
	// validateHTTPS rejects http; use https URL with a transport that redirects to srv.
	// Instead, test post via Build + temporary override: call Notify with https and
	// inject HTTPClient that rewrites — simplest: unit-test retry by exporting behaviour
	// through a Sink whose WebhookURL is https://example.invalid and Transport dials srv.
	sink := &teams.Sink{
		WebhookURL: "https://example.invalid/hooks/abcd",
		HTTPClient: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Budget: 5 * time.Second,
	}
	err := sink.Notify(context.Background(), notify.Event{
		Kind:   notify.KindFinished,
		RunID:  "r1",
		Policy: "p",
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() < 2 {
		t.Fatalf("hits=%d want retry", hits.Load())
	}
}

func TestSinkRejectsNonHTTPS(t *testing.T) {
	s := &teams.Sink{WebhookURL: "http://evil.example/hook"}
	err := s.Notify(context.Background(), notify.Event{Kind: notify.KindStarted})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("got %v", err)
	}
}

func TestSinkEmptyURLNoop(t *testing.T) {
	s := &teams.Sink{}
	if err := s.Notify(context.Background(), notify.Event{Kind: notify.KindStarted}); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
