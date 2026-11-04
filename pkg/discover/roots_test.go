package discover_test

import (
	"context"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			wantReason:   "label:xp-clean.io/eligible=true",
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
			wantReason:   "label:xp-clean.io/eligible=true",
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
			name:         "ttl annotation elapsed",
			labels:       map[string]string{match.LabelEligible: "true"},
			ann:          map[string]string{match.AnnotTTL: "24h"},
			created:      now.Add(-48 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: true},
			wantEligible: true,
			wantExpired:  true,
			wantReason:   "label:xp-clean.io/eligible=true+ttl_elapsed",
		},
		{
			name:         "expires-at annotation elapsed",
			labels:       map[string]string{match.LabelEligible: "true"},
			ann:          map[string]string{match.AnnotExpires: now.Add(-time.Hour).Format(time.RFC3339)},
			created:      now.Add(-72 * time.Hour),
			ttl:          match.TTLConfig{RespectResourceTTL: true},
			wantEligible: true,
			wantExpired:  true,
			wantReason:   "label:xp-clean.io/eligible=true+expires-at",
		},
		{
			name: "default TTL elapsed",
			labels: map[string]string{match.LabelEligible: "true"},
			created: now.Add(-72 * time.Hour),
			ttl: match.TTLConfig{
				RespectResourceTTL:       true,
				DefaultTTL:               24 * time.Hour,
				ApplyDefaultTTLIfMissing: true,
			},
			wantEligible: true,
			wantExpired:  true,
			wantReason:   "label:xp-clean.io/eligible=true+default_ttl",
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

func xdbXRKinds(t *testing.T) map[schema.GroupKind]struct{} {
	t.Helper()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	return discover.XRKindsFromXRDInfos([]discover.XRDInfo{info})
}

func TestHighestParents(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	xrd := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	xrKinds := discover.XRKindsFromXRDInfos([]discover.XRDInfo{info})

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
	roots := discover.HighestParents(candidates, xrKinds)
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
	xrKinds := discover.XRKindsFromXRDInfos([]discover.XRDInfo{info})
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
	}, xrKinds)
	if len(roots) != 2 {
		t.Fatalf("non-controller owner should not demote child; got %d", len(roots))
	}
}

func TestHighestParentsExcludesChildWhenParentNotInCandidates(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	tenantXRD := sampleXRDCustom("datalabtenants.datalab.dev.example.org",
		map[string]string{match.LabelEligible: "true"}, "Namespaced",
		"datalab.dev.example.org", "DataLabTenant", "datalabtenants", "v1alpha1")
	wsXRD := sampleXRDCustom("dbxworkspaces.dbx.dev.example.org",
		map[string]string{match.LabelEligible: "true"}, "Namespaced",
		"dbx.dev.example.org", "DBXWorkspace", "dbxworkspaces", "v1alpha1")
	tenantInfo, err := discover.InspectXRD(tenantXRD, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	wsInfo, err := discover.InspectXRD(wsXRD, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	// Full cluster XRD kind set includes the parent kind even when the parent
	// XR is filtered out of candidates by selectors.
	xrKinds := discover.XRKindsFromXRDInfos([]discover.XRDInfo{tenantInfo, wsInfo})

	child := sampleXRCustom("dbx.dev.example.org/v1alpha1", "DBXWorkspace", "xp-test-1", "thomas-workspace",
		map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	ctrl := true
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "datalab.dev.example.org/v1alpha1",
		Kind:       "DataLabTenant",
		Name:       "thomas",
		UID:        types.UID("uid-thomas"),
		Controller: &ctrl,
	}})

	roots := discover.HighestParents([]discover.RootCandidate{
		{XR: child, XRD: wsInfo, Eligible: true},
	}, xrKinds)
	if len(roots) != 0 {
		t.Fatalf("child owned by filtered-out XR kind must not be a root; got %#v", roots)
	}
}

func TestHighestParentsNonXRControllerOwnerStillRoot(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	infoXRD := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(infoXRD, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	xrKinds := xdbXRKinds(t)
	xr := sampleXR("xp-test-1", "deploy-owned", map[string]string{match.LabelEligible: "true"}, nil, now)
	ctrl := true
	xr.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "some-deploy",
		UID:        types.UID("uid-deploy"),
		Controller: &ctrl,
	}})
	roots := discover.HighestParents([]discover.RootCandidate{
		{XR: xr, XRD: info, Eligible: true},
	}, xrKinds)
	if len(roots) != 1 {
		t.Fatalf("non-XR controller owner must not demote XR; got %d", len(roots))
	}
}

func TestHighestParentsNilControllerStillRoot(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	infoXRD := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	info, err := discover.InspectXRD(infoXRD, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	xrKinds := xdbXRKinds(t)
	xr := sampleXR("xp-test-1", "nil-ctrl", map[string]string{match.LabelEligible: "true"}, nil, now)
	xr.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "db.dev.example.org/v1alpha1",
		Kind:       "XDB",
		Name:       "parent",
		UID:        types.UID("uid-parent"),
		Controller: nil,
	}})
	roots := discover.HighestParents([]discover.RootCandidate{
		{XR: xr, XRD: info, Eligible: true},
	}, xrKinds)
	if len(roots) != 1 {
		t.Fatalf("nil Controller must not demote XR; got %d", len(roots))
	}
}

func TestHighestParentsOwnerVersionMismatchStillChild(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	// XRD serves v1alpha1; ownerReference uses v1beta1 of the same group/kind.
	xrd := sampleXRDCustom("dbxworkspaces.dbx.dev.example.org",
		map[string]string{match.LabelEligible: "true"}, "Namespaced",
		"dbx.dev.example.org", "DBXWorkspace", "dbxworkspaces", "v1alpha1")
	info, err := discover.InspectXRD(xrd, eligibleSelectors())
	if err != nil {
		t.Fatal(err)
	}
	xrKinds := discover.XRKindsFromXRDInfos([]discover.XRDInfo{info})

	child := sampleXRCustom("dbx.dev.example.org/v1alpha1", "DBXWorkspace", "xp-test-1", "ws",
		map[string]string{match.LabelEligible: "true"}, nil, now)
	ctrl := true
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "dbx.dev.example.org/v1beta1",
		Kind:       "DBXWorkspace",
		Name:       "parent-ws",
		UID:        types.UID("uid-parent-ws"),
		Controller: &ctrl,
	}})
	roots := discover.HighestParents([]discover.RootCandidate{
		{XR: child, XRD: info, Eligible: true},
	}, xrKinds)
	if len(roots) != 0 {
		t.Fatalf("same group/kind different version must still be a composed child; got %#v", roots)
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

func TestDiscoverRootsExcludesChildWhenParentXRDFiltered(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tenantXRD := sampleXRDCustom("datalabtenants.datalab.dev.example.org",
		map[string]string{match.LabelEligible: "true"}, "Namespaced",
		"datalab.dev.example.org", "DataLabTenant", "datalabtenants", "v1alpha1")
	wsXRD := sampleXRDCustom("dbxworkspaces.dbx.dev.example.org",
		map[string]string{match.LabelEligible: "true"}, "Namespaced",
		"dbx.dev.example.org", "DBXWorkspace", "dbxworkspaces", "v1alpha1")

	tenant := sampleXRCustom("datalab.dev.example.org/v1alpha1", "DataLabTenant", "xp-test-1", "thomas",
		map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	ws := sampleXRCustom("dbx.dev.example.org/v1alpha1", "DBXWorkspace", "xp-test-1", "thomas-workspace",
		map[string]string{match.LabelEligible: "true"}, nil, now.Add(-72*time.Hour))
	ctrl := true
	ws.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "datalab.dev.example.org/v1alpha1",
		Kind:       "DataLabTenant",
		Name:       tenant.GetName(),
		UID:        tenant.GetUID(),
		Controller: &ctrl,
	}})

	// Policy selects only dbxworkspaces XRDs — parent tenant XRD is filtered out,
	// so the parent XR is never a candidate; child must still not be a root.
	sel := match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
		Globs: match.GlobPatterns{
			XRDNames: []string{"dbxworkspaces.*"},
		},
	}
	dyn := discoverClient(tenantXRD, wsXRD, tenant, ws)
	roots, err := discover.DiscoverRoots(context.Background(), dyn, sel, match.TTLConfig{RespectResourceTTL: false}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Fatalf("want 0 roots when only composed children match selectors, got %d %#v", len(roots), roots)
	}
}
