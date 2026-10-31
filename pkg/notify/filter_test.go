package notify_test

import (
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/notify"
)

func TestFilterShouldNotify(t *testing.T) {
	base := notify.Filter{
		Enabled:                  true,
		NotifyOn:                 []string{"started", "finished", "failed"},
		OnDryRun:                 true,
		OnSuccessWithZeroDeletes: false,
		OnPartialErrors:          true,
	}

	cases := []struct {
		name string
		f    notify.Filter
		ev   notify.Event
		want bool
	}{
		{
			name: "disabled",
			f:    notify.Filter{Enabled: false, NotifyOn: []string{"started"}},
			ev:   notify.Event{Kind: notify.KindStarted},
			want: false,
		},
		{
			name: "started on",
			f:    base,
			ev:   notify.Event{Kind: notify.KindStarted},
			want: true,
		},
		{
			name: "started omitted from notifyOn",
			f: notify.Filter{
				Enabled:  true,
				NotifyOn: []string{"finished", "failed"},
			},
			ev:   notify.Event{Kind: notify.KindStarted},
			want: false,
		},
		{
			name: "dry-run finished when onDryRun",
			f:    base,
			ev:   notify.Event{Kind: notify.KindFinished, DryRun: true, Planned: 0},
			want: true,
		},
		{
			name: "dry-run finished suppressed",
			f: notify.Filter{
				Enabled:  true,
				NotifyOn: []string{"finished"},
				OnDryRun: false,
			},
			ev:   notify.Event{Kind: notify.KindFinished, DryRun: true, Planned: 3},
			want: false,
		},
		{
			name: "live zero deletes quiet",
			f:    base,
			ev:   notify.Event{Kind: notify.KindFinished, DryRun: false, Deleted: 0},
			want: false,
		},
		{
			name: "live deletes notify",
			f:    base,
			ev:   notify.Event{Kind: notify.KindFinished, DryRun: false, Deleted: 2},
			want: true,
		},
		{
			name: "partial errors",
			f:    base,
			ev:   notify.Event{Kind: notify.KindFinished, DryRun: false, Deleted: 1, ErrorCount: 1},
			want: true,
		},
		{
			name: "failed always",
			f:    base,
			ev:   notify.Event{Kind: notify.KindFailed},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.ShouldNotify(tc.ev); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
