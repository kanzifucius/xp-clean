package trace

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RootOnlyFromXR builds a single-node Trace for the XR root without walking
// resourceRefs. Used by deletion.mode=roots (happy path).
func RootOnlyFromXR(root *unstructured.Unstructured, now time.Time) Trace {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rootRef := RefFromUnstructured(root)
	return Trace{
		ID:      fmt.Sprintf("xr/%s", rootRef.Key()),
		Root:    rootRef,
		Nodes:   []Node{nodeFromUnstructured(root, NodeXR)},
		BuiltAt: now,
	}
}
