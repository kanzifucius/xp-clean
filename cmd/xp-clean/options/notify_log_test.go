package options

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kanzifucius/xp-clean/pkg/notify"
)

func TestLogNotifyResultOK(t *testing.T) {
	var buf bytes.Buffer
	LogNotifyResult(logr.Discard(), &buf, "p", notify.KindFinished, nil)
	if got := buf.String(); got != "notify finished: ok\n" {
		t.Fatalf("got %q", got)
	}
}

func TestLogNotifyResultFailed(t *testing.T) {
	var buf bytes.Buffer
	LogNotifyResult(logr.Discard(), &buf, "p", notify.KindStarted, errors.New("webhook down"))
	if got := buf.String(); got != "notify started: failed: webhook down\n" {
		t.Fatalf("got %q", got)
	}
}
