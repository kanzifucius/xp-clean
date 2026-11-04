package notify_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/audit"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/notify"
	"github.com/kanzifucius/xp-clean/pkg/trace"
)

type recordingNotifier struct {
	mu     sync.Mutex
	events []notify.Event
	err    error
}

func (r *recordingNotifier) Notify(_ context.Context, ev notify.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return r.err
}

func TestAroundRunPreservesCleanupError(t *testing.T) {
	n := &recordingNotifier{err: errors.New("webhook down")}
	want := errors.New("discover failed")
	var notifyErrs []error

	_, err := notify.AroundRun(context.Background(), n, notify.RunMeta{
		Policy: "p",
		DryRun: true,
		Scope:  "instances-only",
		RunID:  "run-1",
	}, func(context.Context) (cleanup.RunResult, error) {
		return cleanup.RunResult{}, want
	}, func(e error) {
		notifyErrs = append(notifyErrs, e)
	})
	if !errors.Is(err, want) {
		t.Fatalf("cleanup err = %v, want %v", err, want)
	}
	if len(n.events) != 2 {
		t.Fatalf("events = %d want 2 (started+failed)", len(n.events))
	}
	if n.events[0].Kind != notify.KindStarted || n.events[1].Kind != notify.KindFailed {
		t.Fatalf("kinds %#v", n.events)
	}
	if len(notifyErrs) == 0 {
		t.Fatal("expected notify warnings")
	}
}

func TestAroundRunFinishedOnSuccess(t *testing.T) {
	n := &recordingNotifier{}
	_, err := notify.AroundRun(context.Background(), n, notify.RunMeta{
		Policy: "p",
		DryRun: true,
		Scope:  "instances-only",
		RunID:  "run-2",
	}, func(context.Context) (cleanup.RunResult, error) {
		return cleanup.RunResult{
			Results: []cleanup.ExecuteResult{{Planned: 3, Skipped: 3, DryRun: true}},
		}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.events) != 2 {
		t.Fatalf("events = %d", len(n.events))
	}
	if n.events[1].Kind != notify.KindFinished || n.events[1].Planned != 3 {
		t.Fatalf("%#v", n.events[1])
	}
}

func TestEventFromResultDryRunObjectsFromPlans(t *testing.T) {
	ev := notify.EventFromResult(
		notify.KindFinished,
		notify.RunMeta{Policy: "p", DryRun: true, Scope: "instances-only"},
		"run-objs",
		time.Unix(0, 0).UTC(),
		time.Unix(5, 0).UTC(),
		cleanup.RunResult{
			Plans: []cleanup.DeletePlan{{
				TraceID: "t1",
				DryRun:  true,
				Steps: []cleanup.DeleteStep{{
					Ref:    trace.ObjectRef{Group: "example.org", Version: "v1", Kind: "XDemo", Namespace: "demo", Name: "app-a"},
					Kind:   trace.NodeXR,
					Action: "delete",
					Reason: "instance_root",
				}},
			}},
			Results: []cleanup.ExecuteResult{{Planned: 1, Skipped: 1, DryRun: true}},
		},
		nil,
	)
	if ev.ObjectsTotal != 1 || len(ev.Objects) != 1 {
		t.Fatalf("objects=%#v total=%d", ev.Objects, ev.ObjectsTotal)
	}
	o := ev.Objects[0]
	if o.Kind != "XDemo" || o.Namespace != "demo" || o.Name != "app-a" || o.Reason != "instance_root" {
		t.Fatalf("%#v", o)
	}
}

func TestEventFromResultLiveUsesDeletedAuditOnly(t *testing.T) {
	ev := notify.EventFromResult(
		notify.KindFinished,
		notify.RunMeta{Policy: "p", DryRun: false},
		"run-live",
		time.Unix(0, 0).UTC(),
		time.Unix(5, 0).UTC(),
		cleanup.RunResult{
			Plans: []cleanup.DeletePlan{{
				Steps: []cleanup.DeleteStep{
					{Ref: trace.ObjectRef{Kind: "XDemo", Name: "planned-only"}, Reason: "instance_root"},
					{Ref: trace.ObjectRef{Kind: "XDemo", Name: "gone"}, Reason: "instance_root"},
				},
			}},
			Results: []cleanup.ExecuteResult{{
				Planned: 2,
				Deleted: 1,
				Skipped: 1,
				AuditEntries: []audit.Entry{
					{Action: "delete", Reason: "instance_root", Object: audit.ObjectRef{Kind: "XDemo", Name: "gone"}},
					{Action: "skip", Reason: "wait_timeout", Object: audit.ObjectRef{Kind: "XDemo", Name: "planned-only"}},
				},
			}},
		},
		nil,
	)
	if ev.ObjectsTotal != 1 || len(ev.Objects) != 1 || ev.Objects[0].Name != "gone" {
		t.Fatalf("%#v", ev.Objects)
	}
}

func TestEventFromResultCapsObjects(t *testing.T) {
	steps := make([]cleanup.DeleteStep, notify.MaxObjectsInEvent+5)
	for i := range steps {
		steps[i] = cleanup.DeleteStep{
			Ref:    trace.ObjectRef{Kind: "XDemo", Name: "obj-" + strconv.Itoa(i)},
			Reason: "instance_root",
		}
	}
	ev := notify.EventFromResult(
		notify.KindFinished,
		notify.RunMeta{DryRun: true},
		"run-cap",
		time.Unix(0, 0).UTC(),
		time.Unix(1, 0).UTC(),
		cleanup.RunResult{Plans: []cleanup.DeletePlan{{Steps: steps}}},
		nil,
	)
	if ev.ObjectsTotal != notify.MaxObjectsInEvent+5 {
		t.Fatalf("total=%d", ev.ObjectsTotal)
	}
	if len(ev.Objects) != notify.MaxObjectsInEvent {
		t.Fatalf("len=%d", len(ev.Objects))
	}
}

func TestAroundRunNotifyErrorDoesNotFailSuccess(t *testing.T) {
	n := &recordingNotifier{err: errors.New("boom")}
	_, err := notify.AroundRun(context.Background(), n, notify.RunMeta{RunID: "x"}, func(context.Context) (cleanup.RunResult, error) {
		return cleanup.RunResult{}, nil
	}, nil)
	if err != nil {
		t.Fatalf("cleanup must succeed, got %v", err)
	}
}
