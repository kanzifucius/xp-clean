package notify

import (
	"context"
	"strings"
)

// Filter decides whether an Event should be delivered given policy notify settings.
type Filter struct {
	Enabled                  bool
	NotifyOn                 []string
	OnDryRun                 bool
	OnSuccessWithZeroDeletes bool
	OnPartialErrors          bool
}

// ShouldNotify returns true when the event should be sent.
func (f Filter) ShouldNotify(ev Event) bool {
	if !f.Enabled {
		return false
	}
	if !f.kindAllowed(ev.Kind) {
		return false
	}
	switch ev.Kind {
	case KindStarted:
		return true
	case KindFailed:
		return true
	case KindFinished:
		return f.shouldNotifyFinished(ev)
	default:
		return false
	}
}

func (f Filter) kindAllowed(k EventKind) bool {
	want := strings.ToLower(string(k))
	for _, n := range f.NotifyOn {
		if strings.ToLower(strings.TrimSpace(n)) == want {
			return true
		}
	}
	return false
}

func (f Filter) shouldNotifyFinished(ev Event) bool {
	if ev.DryRun && !f.OnDryRun {
		return false
	}
	if ev.ErrorCount > 0 {
		return f.OnPartialErrors
	}
	// Locked: when OnDryRun, dry-run finished events always go to chat.
	if ev.DryRun {
		return true
	}
	if ev.Deleted == 0 && !f.OnSuccessWithZeroDeletes {
		return false
	}
	return true
}

// Filtering wraps a Notifier and skips events that fail Filter.
type Filtering struct {
	Inner  Notifier
	Filter Filter
}

// Notify implements Notifier.
func (f Filtering) Notify(ctx context.Context, ev Event) error {
	if f.Inner == nil || !f.Filter.ShouldNotify(ev) {
		return nil
	}
	return f.Inner.Notify(ctx, ev)
}
