package trace

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// ObjectRef identifies a Kubernetes object in a trace.
type ObjectRef struct {
	Group     string    `json:"group"`
	Version   string    `json:"version"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace,omitempty"`
	Name      string    `json:"name"`
	UID       types.UID `json:"uid,omitempty"`
}

// Key returns a stable string key for maps and edges.
func (r ObjectRef) Key() string {
	if r.Namespace == "" {
		return r.Group + "/" + r.Version + "/" + r.Kind + "/" + r.Name
	}
	return r.Group + "/" + r.Version + "/" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

// NodeKind classifies a node in the composition tree.
type NodeKind string

const (
	NodeXRD                  NodeKind = "XRD"
	NodeComposition          NodeKind = "Composition"
	NodeCompositionRevision  NodeKind = "CompositionRevision"
	NodeXR                   NodeKind = "XR"
	NodeComposed             NodeKind = "Composed"
	NodeMR                   NodeKind = "MR"
)

// Node is one object in a trace DAG.
type Node struct {
	Ref        ObjectRef         `json:"ref"`
	Kind       NodeKind          `json:"kind"`
	Labels     map[string]string `json:"labels,omitempty"`
	Eligible   bool              `json:"eligible"`
	Keep       bool              `json:"keep"`
	ExpiresAt  *time.Time        `json:"expiresAt,omitempty"`
	Paused     bool              `json:"paused"`
	OrphanHint string            `json:"orphanHint,omitempty"`
}

// Edge links parent → child with a reason.
type Edge struct {
	From   ObjectRef `json:"from"`
	To     ObjectRef `json:"to"`
	Reason string    `json:"reason"`
}

// Trace is one cleanup unit (parent→child DAG).
type Trace struct {
	ID       string    `json:"id"`
	Root     ObjectRef `json:"root"`
	Nodes    []Node    `json:"nodes"`
	Edges    []Edge    `json:"edges"`
	BuiltAt  time.Time `json:"builtAt"`
	Warnings []string  `json:"warnings,omitempty"`
}

// HasKeep returns true if any node is keep-listed.
func (t Trace) HasKeep() bool {
	for _, n := range t.Nodes {
		if n.Keep {
			return true
		}
	}
	return false
}

// HasPaused returns true if any node is paused.
func (t Trace) HasPaused() bool {
	for _, n := range t.Nodes {
		if n.Paused {
			return true
		}
	}
	return false
}

// InstanceNodes returns XR / Composed / MR nodes.
func (t Trace) InstanceNodes() []Node {
	out := make([]Node, 0, len(t.Nodes))
	for _, n := range t.Nodes {
		switch n.Kind {
		case NodeXR, NodeComposed, NodeMR:
			out = append(out, n)
		}
	}
	return out
}

// DefinitionNodes returns XRD / Composition / CompositionRevision nodes.
func (t Trace) DefinitionNodes() []Node {
	out := make([]Node, 0)
	for _, n := range t.Nodes {
		switch n.Kind {
		case NodeXRD, NodeComposition, NodeCompositionRevision:
			out = append(out, n)
		}
	}
	return out
}
