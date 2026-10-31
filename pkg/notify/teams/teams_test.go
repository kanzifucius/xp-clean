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
		LogURL:     "https://logs.example/search?q=01JTEST",
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
	for _, want := range []string{"xp-clean: run finished", "test-stacks", "01JTEST", "Open logs", "No objects deleted"} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload missing %q:\n%s", want, s)
		}
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
