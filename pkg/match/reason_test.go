package match_test

import (
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/match"
)

func TestMatchedReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		sel  match.Selectors
		want string
	}{
		{
			name: "single label",
			sel:  match.Selectors{RequireLabels: map[string]string{match.LabelEligible: "true"}},
			want: "label:xp-clean.io/eligible=true",
		},
		{
			name: "multi labels sorted",
			sel: match.Selectors{RequireLabels: map[string]string{
				"team": "platform",
				"env":  "lab",
			}},
			want: "labels:env=lab,team=platform",
		},
		{
			name: "globs only",
			sel:  match.Selectors{Globs: match.GlobPatterns{XRNames: []string{"test-*"}}},
			want: "globs",
		},
		{
			name: "labels and globs",
			sel: match.Selectors{
				RequireLabels: map[string]string{match.LabelEligible: "true"},
				Globs:         match.GlobPatterns{GVKs: []string{"db.dev.example.org/*"}},
			},
			want: "label:xp-clean.io/eligible=true+globs",
		},
		{
			name: "empty",
			sel:  match.Selectors{},
			want: "matched",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := match.MatchedReason(tt.sel); got != tt.want {
				t.Fatalf("MatchedReason=%q want %q", got, tt.want)
			}
		})
	}
}

func TestTTLElapsedReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	exp := now.Add(-time.Hour)
	tests := []struct {
		name string
		in   match.ExpiryResult
		want string
	}{
		{name: "expires-at", in: match.ExpiryResult{Expired: true, Reason: "expires_at", ExpiresAt: &exp}, want: "expires-at"},
		{name: "ttl annotation", in: match.ExpiryResult{Expired: true, Reason: "ttl"}, want: "ttl_elapsed"},
		{name: "default ttl", in: match.ExpiryResult{Expired: true, Reason: "default_ttl"}, want: "default_ttl"},
		{name: "no ttl gate", in: match.ExpiryResult{Expired: true, Reason: "no_ttl"}, want: ""},
		{name: "ttl not required", in: match.ExpiryResult{Expired: true, Reason: "ttl_not_required"}, want: ""},
		{name: "not expired", in: match.ExpiryResult{Expired: false, Reason: "ttl"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := match.TTLElapsedReason(tt.in); got != tt.want {
				t.Fatalf("TTLElapsedReason=%q want %q", got, tt.want)
			}
		})
	}
}

func TestJoinEligibility(t *testing.T) {
	t.Parallel()
	if got := match.JoinEligibility("label:xp-clean.io/eligible=true", "ttl_elapsed"); got != "label:xp-clean.io/eligible=true+ttl_elapsed" {
		t.Fatalf("got %q", got)
	}
	if got := match.JoinEligibility("globs", ""); got != "globs" {
		t.Fatalf("got %q", got)
	}
	if got := match.JoinEligibility("", "expires-at"); got != "expires-at" {
		t.Fatalf("got %q", got)
	}
	if got := match.JoinEligibility("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestEvaluateUsesMatchedReason(t *testing.T) {
	t.Parallel()
	res, err := match.Evaluate(match.Input{
		Meta: match.ObjectMeta{
			Name:   "test-db",
			Labels: map[string]string{match.LabelEligible: "true"},
		},
		Target: match.TargetXR,
	}, match.Selectors{RequireLabels: map[string]string{match.LabelEligible: "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible || res.Reason != "label:xp-clean.io/eligible=true" {
		t.Fatalf("got %#v", res)
	}
}
