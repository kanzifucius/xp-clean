package discover_test

import (
	"testing"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestForegroundDeleteOptions(t *testing.T) {
	opts := discover.ForegroundDeleteOptions()
	if opts.PropagationPolicy == nil {
		t.Fatal("PropagationPolicy must be set")
	}
	if *opts.PropagationPolicy != metav1.DeletePropagationForeground {
		t.Fatalf("want Foreground, got %s", *opts.PropagationPolicy)
	}
}
