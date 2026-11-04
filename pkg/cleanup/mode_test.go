package cleanup_test

import (
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/cleanup"
)

func TestParseMode(t *testing.T) {
	m, err := cleanup.ParseMode("")
	if err != nil || m != cleanup.ModeRoots {
		t.Fatalf("empty default: got %q %v", m, err)
	}
	m, err = cleanup.ParseMode("roots")
	if err != nil || m != cleanup.ModeRoots {
		t.Fatalf("roots: got %q %v", m, err)
	}
	m, err = cleanup.ParseMode("trace")
	if err != nil || m != cleanup.ModeTrace {
		t.Fatalf("trace: got %q %v", m, err)
	}
	if _, err := cleanup.ParseMode("all"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestDefaultDeletionConfigModeAndWait(t *testing.T) {
	cfg := cleanup.DefaultDeletionConfig()
	if cfg.Mode != cleanup.ModeRoots {
		t.Fatalf("default mode=%q", cfg.Mode)
	}
	if cfg.Wait {
		t.Fatal("default wait must be false")
	}
}
