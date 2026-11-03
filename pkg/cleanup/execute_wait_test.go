package cleanup_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestParseWaitTimeout(t *testing.T) {
	d, err := cleanup.ParseWaitTimeout("")
	if err != nil || d != 30*time.Minute {
		t.Fatalf("default: got %v %v", d, err)
	}
	d, err = cleanup.ParseWaitTimeout("50ms")
	if err != nil || d != 50*time.Millisecond {
		t.Fatalf("got %v %v", d, err)
	}
	if _, err := cleanup.ParseWaitTimeout("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitUntilGoneSucceedsWhenMissing(t *testing.T) {
	g := &stickyGetter{missing: true}
	ref := trace.ObjectRef{Group: "db.dev.example.org", Version: "v1alpha1", Kind: "XDB", Name: "x"}
	res := cleanup.WaitUntilGone(context.Background(), g, ref, time.Second, time.Millisecond)
	if !res.Gone || res.TimedOut || res.Err != nil {
		t.Fatalf("%#v", res)
	}
}

func TestWaitUntilGoneTimesOutWithFinalizers(t *testing.T) {
	g := &stickyGetter{
		obj: &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "db.dev.example.org/v1alpha1",
			"kind":       "XDB",
			"metadata": map[string]interface{}{
				"name":       "x",
				"finalizers": []interface{}{"composite.crossplane.io", "a.b/c"},
			},
		}},
	}
	ref := trace.ObjectRef{Group: "db.dev.example.org", Version: "v1alpha1", Kind: "XDB", Name: "x"}
	res := cleanup.WaitUntilGone(context.Background(), g, ref, 25*time.Millisecond, 5*time.Millisecond)
	if !res.TimedOut || res.Gone {
		t.Fatalf("%#v", res)
	}
	if len(res.Finalizers) != 2 {
		t.Fatalf("finalizers %#v", res.Finalizers)
	}
	reason := cleanup.FormatStuckFinalizers(res.Finalizers)
	if !strings.Contains(reason, "composite.crossplane.io") {
		t.Fatalf("reason %q", reason)
	}
}

func TestExecuteWaitTimeoutAuditsFinalizersAndStops(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	cfg.Scope = cleanup.ScopeAll
	cfg.AllowXRDDeletion = true
	plan := cleanup.BuildPlan(tr, cfg, false)

	del := &countingDeleter{}
	g := &stickyGetter{
		obj: &unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": []interface{}{"composite.crossplane.io"},
			},
		}},
	}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del, cleanup.ExecuteOptions{
		WaitTimeout:  20 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
		Getter:       g,
	})
	if del.n != 1 {
		t.Fatalf("should delete XR once then stop, got %d deletes", del.n)
	}
	if res.Deleted != 0 {
		t.Fatalf("timed-out delete must not count as Deleted: %#v", res)
	}
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0], "stuck finalizers") {
		t.Fatalf("errors %#v", res.Errors)
	}
	foundTimeout := false
	for _, e := range res.AuditEntries {
		if e.Action == "wait-timeout" && strings.Contains(e.Reason, "composite.crossplane.io") {
			foundTimeout = true
		}
	}
	if !foundTimeout {
		t.Fatalf("audit %#v", res.AuditEntries)
	}
}

func TestExecuteWaitsUntilGone(t *testing.T) {
	tr := sampleTrace()
	plan := cleanup.BuildPlan(tr, cleanup.DefaultDeletionConfig(), false)
	del := &countingDeleter{}
	g := &stickyGetter{goneAfter: 2}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del, cleanup.ExecuteOptions{
		WaitTimeout:  time.Second,
		PollInterval: time.Millisecond,
		Getter:       g,
	})
	if res.Deleted != 1 || len(res.Errors) != 0 {
		t.Fatalf("%#v polls=%d", res, g.calls)
	}
}

func TestExecuteDefinitionGateBlocksWhenXRsRemain(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	cfg.Scope = cleanup.ScopeAll
	cfg.AllowXRDDeletion = true
	plan := cleanup.BuildPlan(tr, cfg, false)

	del := &countingDeleter{}
	g := &goneGetter{}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del, cleanup.ExecuteOptions{
		WaitTimeout:  time.Second,
		PollInterval: time.Millisecond,
		Getter:       g,
		CountRemainingXRs: func(context.Context) (int, error) {
			return 3, nil
		},
	})
	// XR deleted; definition steps skipped.
	if del.n != 1 {
		t.Fatalf("want 1 delete (XR only), got %d", del.n)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted=%d errors=%v", res.Deleted, res.Errors)
	}
	blocked := false
	for _, e := range res.Errors {
		if strings.Contains(e, "3 XR(s) still remain") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("errors %#v", res.Errors)
	}
}

func TestExecuteDefinitionGateAllowsWhenZeroXRs(t *testing.T) {
	tr := sampleTrace()
	cfg := cleanup.DefaultDeletionConfig()
	cfg.Scope = cleanup.ScopeAll
	cfg.AllowXRDDeletion = true
	plan := cleanup.BuildPlan(tr, cfg, false)

	del := &countingDeleter{}
	g := &goneGetter{}
	res := cleanup.Execute(context.Background(), plan, "test-stacks", del, cleanup.ExecuteOptions{
		WaitTimeout:  time.Second,
		PollInterval: time.Millisecond,
		Getter:       g,
		CountRemainingXRs: func(context.Context) (int, error) {
			return 0, nil
		},
	})
	if del.n != 4 {
		t.Fatalf("want XR+rev+comp+XRD = 4 deletes, got %d", del.n)
	}
	if res.Deleted != 4 || len(res.Errors) != 0 {
		t.Fatalf("%#v", res)
	}
}

type stickyGetter struct {
	obj       *unstructured.Unstructured
	missing   bool
	goneAfter int
	calls     int
}

func (s *stickyGetter) Get(_ context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	s.calls++
	if s.missing {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: ref.Group, Resource: ref.Kind}, ref.Name)
	}
	if s.goneAfter > 0 && s.calls >= s.goneAfter {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: ref.Group, Resource: ref.Kind}, ref.Name)
	}
	if s.obj != nil {
		return s.obj.DeepCopy(), nil
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": ref.Name},
	}}, nil
}

type goneGetter struct{}

func (goneGetter) Get(_ context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: ref.Group, Resource: ref.Kind}, ref.Name)
}
