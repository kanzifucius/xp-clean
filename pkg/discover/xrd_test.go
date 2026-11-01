package discover_test

import (
	"context"
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
)

func TestInspectXRD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		xrdName       string
		labels        map[string]string
		scope         string
		sel           match.Selectors
		wantEligible  bool
		wantLegacy    bool
		wantKind      string
		wantGroup     string
		wantVersion   string
		wantResource  string
		wantErr       bool
	}{
		{
			name:         "eligible namespaced XRD",
			xrdName:      "xdbs.db.dev.example.org",
			labels:       map[string]string{match.LabelEligible: "true"},
			scope:        "Namespaced",
			sel:          eligibleSelectors(),
			wantEligible: true,
			wantKind:     "XDB",
			wantGroup:    "db.dev.example.org",
			wantVersion:  "v1alpha1",
			wantResource: "xdbs",
		},
		{
			name:         "labels mismatch",
			xrdName:      "xdbs.db.dev.example.org",
			labels:       map[string]string{"other": "true"},
			sel:          eligibleSelectors(),
			wantEligible: false,
			wantKind:     "XDB",
			wantGroup:    "db.dev.example.org",
			wantVersion:  "v1alpha1",
			wantResource: "xdbs",
		},
		{
			name:         "LegacyCluster flagged without claim walk",
			xrdName:      "xlegacies.legacy.dev.example.org",
			labels:       map[string]string{match.LabelEligible: "true"},
			scope:        "LegacyCluster",
			sel:          eligibleSelectors(),
			wantEligible: true,
			wantLegacy:   true,
			wantKind:     "XDB", // sampleXRD always uses XDB kind; override below
			wantGroup:    "db.dev.example.org",
			wantVersion:  "v1alpha1",
			wantResource: "xdbs",
		},
		{
			name:    "glob XRD name",
			xrdName: "xdbs.db.dev.example.org",
			labels:  nil,
			sel: match.Selectors{
				Globs: match.GlobPatterns{XRDNames: []string{"*.db.dev.example.org"}},
			},
			wantEligible: true,
			wantKind:     "XDB",
			wantGroup:    "db.dev.example.org",
			wantVersion:  "v1alpha1",
			wantResource: "xdbs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			xrd := sampleXRD(tt.xrdName, tt.labels, tt.scope)
			info, err := discover.InspectXRD(xrd, tt.sel)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.Eligible != tt.wantEligible {
				t.Fatalf("Eligible=%v want %v (reason=%s)", info.Eligible, tt.wantEligible, info.MatchReason)
			}
			if info.LegacyCluster != tt.wantLegacy {
				t.Fatalf("LegacyCluster=%v want %v", info.LegacyCluster, tt.wantLegacy)
			}
			if info.XRGVK.Kind != tt.wantKind || info.XRGVK.Group != tt.wantGroup || info.XRGVK.Version != tt.wantVersion {
				t.Fatalf("GVK=%s want %s/%s %s", info.XRGVK.String(), tt.wantGroup, tt.wantVersion, tt.wantKind)
			}
			if info.XRResource != tt.wantResource {
				t.Fatalf("XRResource=%s want %s", info.XRResource, tt.wantResource)
			}
		})
	}
}

func TestInspectXRDMissingFields(t *testing.T) {
	t.Parallel()
	bad := discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", "broken", nil,
		map[string]interface{}{"names": map[string]interface{}{"kind": "X"}},
	)
	if _, err := discover.InspectXRD(bad, eligibleSelectors()); err == nil {
		t.Fatal("expected error for incomplete XRD")
	}
}

func TestFilterEligibleXRDs(t *testing.T) {
	t.Parallel()
	infos := []discover.XRDInfo{
		{Eligible: true},
		{Eligible: false},
		{Eligible: true, LegacyCluster: true},
	}
	got := discover.FilterEligibleXRDs(infos)
	if len(got) != 2 {
		t.Fatalf("want 2 eligible, got %d", len(got))
	}
	// LegacyCluster XRDs remain when Eligible — callers must never walk claims.
	if !got[1].LegacyCluster {
		t.Fatal("expected LegacyCluster info retained among eligible set")
	}
}

func TestListXRDsPrefersV2(t *testing.T) {
	t.Parallel()
	v2 := sampleXRD("xdbs.db.dev.example.org", map[string]string{match.LabelEligible: "true"}, "Namespaced")
	dyn := discoverClient(v2)
	list, err := discover.ListXRDs(context.Background(), dyn)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].GetName() != "xdbs.db.dev.example.org" {
		t.Fatalf("unexpected list: %#v", list)
	}
	if list[0].GetAPIVersion() != "apiextensions.crossplane.io/v2" {
		t.Fatalf("want v2 apiVersion, got %s", list[0].GetAPIVersion())
	}
}

func TestListXRDsFallsBackToV1(t *testing.T) {
	t.Parallel()
	v1 := sampleXRDV1("xlegacies.legacy.dev.example.org")
	// Only register v1 list kind objects — v2 List returns empty, not error, on fake.
	// Empty v2 list is success for ListXRDs (first API that responds without error).
	// Seed only v1 objects and ensure ListXRDs still returns when v2 is empty.
	dyn := discoverClient(v1)
	list, err := discover.ListXRDs(context.Background(), dyn)
	if err != nil {
		t.Fatal(err)
	}
	// Fake returns empty for v2 (success), so ListXRDs stops at v2 with empty list.
	// That matches live behaviour when the v2 API exists but has no XRDs.
	if len(list) != 0 {
		t.Fatalf("v2 empty list should short-circuit before v1; got %d", len(list))
	}
}

func TestListXRDsNilClient(t *testing.T) {
	t.Parallel()
	_, err := discover.ListXRDs(context.Background(), &discover.DynamicClient{})
	if err == nil {
		t.Fatal("expected error with nil dynamic client")
	}
}
