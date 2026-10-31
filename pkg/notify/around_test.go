package notify_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	"github.com/kanzifucius/xp-clean/pkg/notify"
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

func TestAroundRunNotifyErrorDoesNotFailSuccess(t *testing.T) {
	n := &recordingNotifier{err: errors.New("boom")}
	_, err := notify.AroundRun(context.Background(), n, notify.RunMeta{RunID: "x"}, func(context.Context) (cleanup.RunResult, error) {
		return cleanup.RunResult{}, nil
	}, nil)
	if err != nil {
		t.Fatalf("cleanup must succeed, got %v", err)
	}
}
