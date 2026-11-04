package options

import (
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/pkg/notify"
)

// LogNotifyResult reports one notify delivery attempt.
//
// Failures always go to stderr (visible under quiet CLI) and logr Error.
// Successes print a short stderr line and logr Info (Info needs -v / manager).
// Callers must not invoke this for ErrSkipped (AroundRun already filters those).
func LogNotifyResult(log logr.Logger, w io.Writer, policy string, kind notify.EventKind, err error) {
	if err != nil {
		if log.GetSink() != nil {
			log.Error(err, "notify failed", "kind", string(kind), "policy", policy)
		}
		if w != nil {
			_, _ = fmt.Fprintf(w, "notify %s: failed: %v\n", kind, err)
		}
		return
	}
	if log.GetSink() != nil {
		log.Info("notify ok", "kind", string(kind), "policy", policy)
	}
	if w != nil {
		_, _ = fmt.Fprintf(w, "notify %s: ok\n", kind)
	}
}
