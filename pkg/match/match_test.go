package match_test

import (
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/match"
)

func TestLabelsAndGlobsAND(t *testing.T) {
	sel := match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
		Globs: match.GlobPatterns{
			XRNames: []string{"test-*"},
		},
	}
	in := match.Input{
		Meta: match.ObjectMeta{
			Name:   "test-db",
			Labels: map[string]string{match.LabelEligible: "true"},
		},
		Group:   "db.dev.example.org",
		Version: "v1alpha1",
		Kind:    "XPostgreSQLInstance",
		Target:  match.TargetXR,
	}
	res, err := match.Evaluate(in, sel)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible {
		t.Fatalf("expected eligible, got %#v", res)
	}

	in.Meta.Name = "prod-db"
	res, err = match.Evaluate(in, sel)
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible {
		t.Fatalf("expected globs_mismatch, got %#v", res)
	}
}

func TestGlobsOnly(t *testing.T) {
	sel := match.Selectors{
		Globs: match.GlobPatterns{
			XRDNames: []string{"*.dev.example.org"},
		},
	}
	res, err := match.Evaluate(match.Input{
		Meta:    match.ObjectMeta{Name: "mydb.dev.example.org"},
		Group:   "mydb.dev.example.org",
		Version: "v1alpha1",
		Kind:    "XMyDB",
		Target:  match.TargetXRD,
	}, sel)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible {
		t.Fatalf("expected eligible via globs alone, got %#v", res)
	}
}

func TestKeepAndNamespaceDeny(t *testing.T) {
	sel := match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
		NamespaceDeny: []string{"kube-system"},
	}
	res, err := match.Evaluate(match.Input{
		Meta: match.ObjectMeta{
			Name:      "x",
			Namespace: "kube-system",
			Labels:    map[string]string{match.LabelEligible: "true"},
		},
		Target: match.TargetXR,
	}, sel)
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || !res.Denied {
		t.Fatalf("expected namespace deny, got %#v", res)
	}

	res, err = match.Evaluate(match.Input{
		Meta: match.ObjectMeta{
			Name:   "x",
			Labels: map[string]string{match.LabelEligible: "true", match.LabelKeep: "true"},
		},
		Target: match.TargetXR,
	}, sel)
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || !res.Keep {
		t.Fatalf("expected keep, got %#v", res)
	}
}

func TestEmptySelectorsRejected(t *testing.T) {
	res, err := match.Evaluate(match.Input{
		Meta:   match.ObjectMeta{Name: "x"},
		Target: match.TargetXR,
	}, match.Selectors{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || res.Reason != "empty_selectors" {
		t.Fatalf("got %#v", res)
	}
}

func TestTTLExpiry(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	created := now.Add(-48 * time.Hour)
	meta := match.ObjectMeta{
		CreationTimestamp: created,
		Annotations:       map[string]string{match.AnnotTTL: "24h"},
	}
	exp, err := match.EvaluateExpiry(meta, match.TTLConfig{RespectResourceTTL: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Expired {
		t.Fatalf("expected expired, got %#v", exp)
	}

	meta.Annotations = map[string]string{match.AnnotExpires: now.Add(time.Hour).Format(time.RFC3339)}
	exp, err = match.EvaluateExpiry(meta, match.TTLConfig{RespectResourceTTL: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Expired {
		t.Fatalf("expected not expired, got %#v", exp)
	}
}

func TestMatchGVK(t *testing.T) {
	ok, err := match.MatchGVK("db.dev.example.org", "v1alpha1", "XDB", []string{"*.dev.example.org/*"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected GVK glob match")
	}
}
