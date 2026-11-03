package cleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kanzifucius/xp-clean/pkg/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const defaultPollInterval = 2 * time.Second

// ObjectGetter fetches an object for wait-after-delete polling.
type ObjectGetter interface {
	Get(ctx context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error)
}

// ParseWaitTimeout parses deletion.waitTimeout; empty defaults to 30m.
func ParseWaitTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 30 * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid waitTimeout %q: %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid waitTimeout %q: must be non-negative", s)
	}
	return d, nil
}

// WaitResult is the outcome of polling until an object is gone.
type WaitResult struct {
	Gone       bool
	TimedOut   bool
	Finalizers []string
	Err        error
}

// WaitUntilGone polls getter until ref is NotFound or timeout elapses.
// It never strips finalizers — on timeout it surfaces the finalizer list for audit.
func WaitUntilGone(ctx context.Context, getter ObjectGetter, ref trace.ObjectRef, timeout, pollInterval time.Duration) WaitResult {
	if getter == nil || timeout == 0 {
		return WaitResult{Gone: true}
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	deadline := time.Now().Add(timeout)
	var lastFinalizers []string

	for {
		if err := ctx.Err(); err != nil {
			return WaitResult{Finalizers: lastFinalizers, Err: err}
		}

		obj, err := getter.Get(ctx, ref)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return WaitResult{Gone: true}
			}
			return WaitResult{Finalizers: lastFinalizers, Err: err}
		}
		lastFinalizers = append([]string(nil), obj.GetFinalizers()...)

		if !time.Now().Before(deadline) {
			return WaitResult{
				TimedOut:   true,
				Finalizers: lastFinalizers,
			}
		}

		sleep := pollInterval
		if rem := time.Until(deadline); rem > 0 && rem < sleep {
			sleep = rem
		}

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return WaitResult{Finalizers: lastFinalizers, Err: ctx.Err()}
		case <-timer.C:
		}
	}
}

// FormatStuckFinalizers builds an audit reason for a wait timeout.
func FormatStuckFinalizers(finalizers []string) string {
	if len(finalizers) == 0 {
		return "wait_timeout: object still present (no finalizers listed)"
	}
	return "wait_timeout: stuck finalizers: " + strings.Join(finalizers, ",")
}
