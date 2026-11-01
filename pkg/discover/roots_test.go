package discover_test

import (
	"context"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestEvaluateXRTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		labels       map[string]string
		ann          map[string]string
		created      time.Time
		ttl          match.TTLConfig
		wantEligible bool
		wantKeep     bool
		wantPaused   bool
		wantExpired  bool
		wantReason   string
	}{
		{
			name:         "eligible expired via default TTL",
			labels:       map[string]string{match.LabelEligible: "true"},
			created:      now.Add(-72 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: false},
			wantEligible: true,
			wantExpired:  true,
			wantReason:   "matched",
		},
		{
			name:         "keep label blocks eligibility",
			labels:       map[string]string{match.LabelEligible: "true", match.LabelKeep: "true"},
			created:      now.Add(-72 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: false},
			wantEligible: false,
			wantKeep:     true,
			wantReason:   "keep",
		},
		{
			name:         "paused still eligible for planning gates",
			labels:       map[string]string{match.LabelEligible: "true"},
			ann:          map[string]string{match.AnnotPaused: "true"},
			created:      now.Add(-72 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: false},
			wantEligible: true,
			wantPaused:   true,
			wantExpired:  true,
			wantReason:   "matched",
		},
		{
			name:         "ttl not elapsed",
			labels:       map[string]string{match.LabelEligible: "true"},
			ann:          map[string]string{match.AnnotTTL: "48h"},
			created:      now.Add(-1 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: true},
			wantEligible: false,
			wantExpired:  false,
			wantReason:   "ttl_not_elapsed",
		},
		{
			name:         "labels mismatch",
			labels:       map[string]string{"env": "prod"},
			created:      now.Add(-72 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: false},
			wantEligible: false,
			wantReason:   "labels_mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			xr := sampleXR("xp-test-1", "test-db", tt.labels, tt.ann, tt.created)
			c, err := discover.EvaluateXR(xr, info, eligibleSelectors(), tt.ttl, now)
			if err != nil {
				t.Fatal(err)
			}
			if c.Eligible != tt.wantEligible {
				t.Fatalf("Eligible=%v want %v (reason=%s)", c.Eligible, tt.wantEligible, c.Reason)
			}
			if c.Keep != tt.wantKeep {
				t.Fatalf("Keep=%v want %v", c.Keep, tt.wantKeep)
			}
			if c.Paused != tt.wantPaused {
				t.Fatalf("Paused=%v want %v", c.Paused, tt.wantPaused)
			}
			if c.Expired != tt.wantExpired {
				t.Fatalf("Expired=%v want %v", c.Expired, tt.wantExpired)
			}
			if tt.wantReason != "" && c.Reason != tt.wantReason {
				t.Fatalf("Reason=%q want %q", c.Reason, tt.wantReason)
			}
		})
	}
}

func TestHighestParents(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}

	parent := sampleXR("xp-test-1", "parent-xr", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	child := sampleXR("xp-test-1", "child-xr", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	ctrl := true
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "db.dev.example.org/v1alpha1",
		Kind:       "XDB",
		Name:       parent.GetName(),
		UID:        parent.GetUID(),
		Controller: &ctrl,
	}})
	unrelated := sampleXR("xp-test-1", "solo-xr", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))

	candidates := []discover.RootCandidate{
		{XR: parent, XRD: info, Eligible: true},
		{XR: child, XRD: info, Eligible: true},
		{XR: unrelated, XRD: info, Eligible: true},
	}
	roots := discover.HighestParents(candidates)
	if len(roots) != 2 {
		t.Fatalf("want 2 highest parents, got %d", len(roots))
	}
	names := map[string]bool{}
	for _, r := range roots {
		names[r.XR.GetName()] = true
	}
	if !names["parent-xr"] || !names["solo-xr"] || names["child-xr"] {
		t.Fatalf("unexpected roots: %v", names)
	}
}

func TestHighestParentsIgnoresNonControllerOwners(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	a := sampleXR("xp-test-1", "a", map[string]string{match.LabelEligible: "true"}, nil, now)
	b := sampleXR("xp-test-1", "b", map[string]string{match.LabelEligible: "true"}, nil, now)
	ctrl := false
	b.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "db.dev.example.org/v1alpha1",
		Kind:       "XDB",
		Name:       a.GetName(),
		UID:        a.GetUID(),
		Controller: &ctrl,
	}})
	roots := discover.HighestParents([]discover.RootCandidate{
		{XR: a, XRD: info, Eligible: true},
		{XR: b, XRD: info, Eligible: true},
	})
	if len(roots) != 2 {
		t.Fatalf("non-controller owner should not demote child; got %d", len(roots))
	}
}

func TestDiscoverRoots(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	parent := sampleXR("xp-test-1", "parent-xr", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	child := sampleXR("xp-test-1", "child-xr", map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	ctrl := true
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "db.dev.example.org/v1alpha1",
		Kind:       "XDB",
		Name:       parent.GetName(),
		UID:        types.UID("uid-parent-xr"),
		Controller: &ctrl,
	}})
	keep := sampleXR("xp-test-1", "keep-xr", map[string]string{
		match.LabelEligible: "true",
		match.LabelKeep:     "true",
	}, nil, now.Add(-72*time.Hour))
	ineligibleXRD := sampleXRD("other.db.dev.example.org", map[string]string{"team": "platform"}, "Namespaced")

	dyn := discoverClient(xrd, ineligibleXRD, parent, child, keep)
	roots, err := discover.DiscoverRoots(context.Background(), dyn, eligibleSelectors(), match.TTLConfig{RespectResourceTTL: false}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("want 1 root (parent only; child owned, keep filtered), got %d %#v", len(roots), roots)
	}
	if roots[0].XR.GetName() != "parent-xr" {
		t.Fatalf("want parent-xr, got %s", roots[0].XR.GetName())
	}
}

func TestDiscoverRootsLegacyClusterXRDEligibleButNoClaims(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	// LegacyCluster XRD may match selectors for XR GVK discovery; claim edges are never walked.
	legacyV2 := sampleXRD("xlegacies.legacy.dev.example.org", map[string]string{match.LabelEligible: "true"}, "LegacyCluster")
	info, err := discover.InspectXRD(legacyV2, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	if !info.LegacyCluster || !info.Eligible {
		t.Fatalf("expected eligible LegacyCluster XRD, got %#v", info)
	}
	dyn := discoverClient(legacyV2)
	roots, err := discover.DiscoverRoots(context.Background(), dyn, eligibleSelectors(), match.TTLConfig{RespectResourceTTL: false}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Fatalf("no XR instances seeded; want 0 roots, got %d", len(roots))
	}
}
