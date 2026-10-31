package notify_test

import (
	"strings"
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/notify"
)

func TestExpandLogURL(t *testing.T) {
	got := notify.ExpandLogURL("https://logs.example/search?q={{runId}}", "01JABC")
	if got != "https://logs.example/search?q=01JABC" {
		t.Fatalf("got %q", got)
	}
	if notify.ExpandLogURL("", "x") != "" {
		t.Fatal("empty template")
	}
}

func TestRedactWebhookURL(t *testing.T) {
	got := notify.RedactWebhookURL("https://prod-00.westus.logic.azure.com:443/workflows/abcdef123456/triggers/manual/paths/invoke?api-version=1")
	if got == "" || !strings.Contains(got, "logic.azure.com") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "abcdef123456") {
		t.Fatalf("should redact path secret: %q", got)
	}
}

func TestTruncateErrors(t *testing.T) {
	errs := notify.TruncateErrors([]string{"abcdefghij"}, 1, 5)
	if len(errs) != 1 || errs[0] != "abcde…" {
		t.Fatalf("%#v", errs)
	}
}
