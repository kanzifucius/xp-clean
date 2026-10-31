package trace

import (
	"fmt"
	"strings"
)

// RenderText returns a simple ASCII tree for a Trace.
func RenderText(t Trace) string {
	children := map[string][]ObjectRef{}
	for _, e := range t.Edges {
		children[e.From.Key()] = append(children[e.From.Key()], e.To)
	}
	nodeByKey := map[string]Node{}
	for _, n := range t.Nodes {
		nodeByKey[n.Ref.Key()] = n
	}

	var b strings.Builder
	fmt.Fprintf(&b, "trace %s\n", t.ID)
	fmt.Fprintf(&b, "%s\n", formatNodeLine(t.Root, nodeByKey))
	writeChildren(&b, t.Root, nodeByKey, children, "")
	for _, w := range t.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", w)
	}
	return b.String()
}

func writeChildren(b *strings.Builder, ref ObjectRef, nodes map[string]Node, children map[string][]ObjectRef, prefix string) {
	kids := children[ref.Key()]
	for i, child := range kids {
		last := i == len(kids)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		fmt.Fprintf(b, "%s%s%s\n", prefix, connector, formatNodeLine(child, nodes))
		writeChildren(b, child, nodes, children, nextPrefix)
	}
}

func formatNodeLine(ref ObjectRef, nodes map[string]Node) string {
	n, ok := nodes[ref.Key()]
	kind := ref.Kind
	if ok {
		kind = string(n.Kind)
	}
	line := fmt.Sprintf("%s %s", kind, displayName(ref))
	if ok {
		if n.Keep {
			line += " [keep]"
		}
		if n.Paused {
			line += " [paused]"
		}
		if n.OrphanHint != "" {
			line += " [" + n.OrphanHint + "]"
		}
	}
	return line
}

func displayName(ref ObjectRef) string {
	if ref.Namespace == "" {
		return ref.Name
	}
	return ref.Namespace + "/" + ref.Name
}
