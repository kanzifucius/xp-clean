package options

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/trace"
)

func samplePolicy(mode cleanup.DeletionMode, scope cleanup.DeletionScope) conf.Resolved {
	return conf.Resolved{
		Name:   "dbxworkspace-local",
		DryRun: true,
		Deletion: cleanup.DeletionConfig{
			Mode:  mode,
			Scope: scope,
		},
	}
}

func xrStep(kind, ns, name string) cleanup.DeleteStep {
	return cleanup.DeleteStep{
		Ref: trace.ObjectRef{
			Group:     "demo.example.io",
			Version:   "v1alpha1",
			Kind:      kind,
			Namespace: ns,
			Name:      name,
		},
		Kind:   trace.NodeXR,
		Action: "delete",
		Reason: "instance_root",
	}
}

func TestFormatStepObjectMatchesNotifyLine(t *testing.T) {
	step := xrStep("DBXWorkspace", "ns", "name")
	got := formatStepObject(step)
	want := "DBXWorkspace ns/name — instance_root"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	bare := cleanup.DeleteStep{
		Ref:    trace.ObjectRef{Kind: "Composition", Name: "c"},
		Kind:   trace.NodeComposition,
		Action: "delete",
	}
	if formatStepObject(bare) != "Composition c" {
		t.Fatalf("empty reason must omit separator: %q", formatStepObject(bare))
	}
}

func TestPrintSummaryQuietRootsInstancesOnly(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeInstancesOnly)
	result := cleanup.RunResult{
		Traces: []trace.Trace{
			{ID: "xr/sofia", Root: xrStep("DBXWorkspace", "dai-platform-jorge", "sofia-workspace").Ref},
			{ID: "xr/cross", Root: xrStep("DBXWorkspace", "dai-platform-thomas", "cross-workspace").Ref},
		},
		Plans: []cleanup.DeletePlan{
			{TraceID: "xr/sofia", DryRun: true, Steps: []cleanup.DeleteStep{xrStep("DBXWorkspace", "dai-platform-jorge", "sofia-workspace")}},
			{TraceID: "xr/cross", DryRun: true, Steps: []cleanup.DeleteStep{xrStep("DBXWorkspace", "dai-platform-thomas", "cross-workspace")}},
		},
	}

	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{})
	got := buf.String()

	wantLines := []string{
		"policy=dbxworkspace-local dryRun=true mode=roots scope=instances-only",
		"would delete 2",
		"  DBXWorkspace dai-platform-jorge/sofia-workspace — instance_root",
		"  DBXWorkspace dai-platform-thomas/cross-workspace — instance_root",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Fatalf("missing %q in:\n%s", line, got)
		}
	}
	for _, ban := range []string{"trace ", "steps=", "stacksMatched", "1. delete", "apiVersion"} {
		if strings.Contains(got, ban) {
			t.Fatalf("quiet summary must not contain %q:\n%s", ban, got)
		}
	}
}

func TestPrintSummaryQuietEmpty(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeInstancesOnly)
	var buf bytes.Buffer
	PrintSummary(&buf, cleanup.RunResult{}, pol, SummaryOptions{})
	got := buf.String()
	if !strings.Contains(got, "would delete 0\n") {
		t.Fatalf("expected empty would-delete line, got:\n%s", got)
	}
	if strings.Contains(got, "trace ") || strings.Contains(got, "steps=") {
		t.Fatalf("empty quiet run must stay compact:\n%s", got)
	}
}

func TestPrintSummaryBlockedStillListed(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeInstancesOnly)
	result := cleanup.RunResult{
		Plans: []cleanup.DeletePlan{
			{
				TraceID:   "xr/paused",
				BlockedBy: []string{"crossplane.io/paused (deletePaused=false)"},
			},
			{
				TraceID: "xr/ok",
				DryRun:  true,
				Steps:   []cleanup.DeleteStep{xrStep("XDemo", "ns", "ok")},
			},
		},
	}
	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{})
	got := buf.String()
	if !strings.Contains(got, "would delete 1") || !strings.Contains(got, "  XDemo ns/ok — instance_root") {
		t.Fatalf("compact list missing:\n%s", got)
	}
	if !strings.Contains(got, "stack xr/paused blocked:") {
		t.Fatalf("blocked plan missing:\n%s", got)
	}
}

func TestPrintSummaryVerboseShowsTracesAndStacks(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeInstancesOnly)
	ref := xrStep("XDemo", "ns", "one").Ref
	result := cleanup.RunResult{
		Traces: []trace.Trace{{
			ID:   "xr/one",
			Root: ref,
			Nodes: []trace.Node{{
				Ref:  ref,
				Kind: trace.NodeXR,
			}},
		}},
		Plans: []cleanup.DeletePlan{{
			TraceID: "xr/one",
			DryRun:  true,
			Steps:   []cleanup.DeleteStep{xrStep("XDemo", "ns", "one")},
		}},
	}
	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{Verbose: true})
	got := buf.String()
	if !strings.Contains(got, "trace xr/one") {
		t.Fatalf("verbose must print trace:\n%s", got)
	}
	if !strings.Contains(got, "stack xr/one would delete steps=1") {
		t.Fatalf("verbose must print stack block:\n%s", got)
	}
	if !strings.Contains(got, "1. delete XDemo ns/one — instance_root") {
		t.Fatalf("verbose must number steps with reason:\n%s", got)
	}
}

func TestPrintSummaryModeTraceShowsTraces(t *testing.T) {
	pol := samplePolicy(cleanup.ModeTrace, cleanup.ScopeInstancesOnly)
	ref := xrStep("XDemo", "ns", "one").Ref
	result := cleanup.RunResult{
		Traces: []trace.Trace{{
			ID:   "xr/one",
			Root: ref,
			Nodes: []trace.Node{{
				Ref:  ref,
				Kind: trace.NodeXR,
			}},
		}},
		Plans: []cleanup.DeletePlan{{
			TraceID: "xr/one",
			DryRun:  true,
			Steps:   []cleanup.DeleteStep{xrStep("XDemo", "ns", "one")},
		}},
	}
	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{})
	got := buf.String()
	if !strings.Contains(got, "trace xr/one") {
		t.Fatalf("mode=trace must print trace without -v:\n%s", got)
	}
	if !strings.Contains(got, "stack xr/one would delete steps=1") {
		t.Fatalf("mode=trace must keep stack detail:\n%s", got)
	}
}

func TestPrintSummaryMultiStepKeepsDetail(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeInstancesOnly)
	result := cleanup.RunResult{
		Plans: []cleanup.DeletePlan{{
			TraceID: "xr/multi",
			DryRun:  true,
			Steps: []cleanup.DeleteStep{
				xrStep("XDemo", "ns", "root"),
				{
					Ref:    trace.ObjectRef{Kind: "Composition", Name: "c"},
					Kind:   trace.NodeComposition,
					Action: "delete",
				},
			},
		}},
	}
	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{})
	got := buf.String()
	if !strings.Contains(got, "stack xr/multi would delete steps=2") {
		t.Fatalf("multi-step must keep stack block:\n%s", got)
	}
	if strings.Contains(got, "would delete 2\n") {
		t.Fatalf("multi-step must not use compact aggregate:\n%s", got)
	}
}

func TestPrintSummaryScopeAllKeepsDetail(t *testing.T) {
	pol := samplePolicy(cleanup.ModeRoots, cleanup.ScopeAll)
	result := cleanup.RunResult{
		Plans: []cleanup.DeletePlan{{
			TraceID: "xr/one",
			DryRun:  true,
			Steps:   []cleanup.DeleteStep{xrStep("XDemo", "ns", "one")},
		}},
	}
	var buf bytes.Buffer
	PrintSummary(&buf, result, pol, SummaryOptions{})
	got := buf.String()
	if !strings.Contains(got, "stack xr/one would delete steps=1") {
		t.Fatalf("scope=all must keep stack detail:\n%s", got)
	}
	if strings.Contains(got, "trace ") {
		t.Fatalf("scope=all alone must not dump traces:\n%s", got)
	}
}
