// Package mermaid renders mermaid diagram source from neutral, primitive inputs.
//
// It is deliberately decoupled from any domain type: callers map their own
// shapes (a metamodel CustomType, a data-entry EnumHelp DTO, a tracer result)
// into the small structs defined here. That keeps the injection-safe rendering
// logic in one place while letting both the data-entry help modal and the
// rela-docs generator share it without either importing the other.
//
// Injection safety is the reason this code exists as a shared unit. Raw enum
// values and node names are NOT safe to splice into mermaid syntax — a value
// with a space, arrow, or colon breaks parsing, and a newline in a label can
// inject a spurious statement. Every state/node is mapped to a synthetic id
// (s0, s1, … / n0, n1, …) and aliased so the diagram DISPLAYS the real text
// while edges reference the safe id; labels are flattened so they can never
// spill onto a new diagram line.
package mermaid

import (
	"fmt"
	"strings"
)

// Transition is one edge of a state machine: a move from one state value to
// another, labeled with a verb (Label). Label may be empty.
type Transition struct {
	From  string
	To    string
	Label string
}

// StateDiagram renders a mermaid stateDiagram-v2 for a state machine: an entry
// arrow to initial (when non-empty) plus one edge per transition. States are
// emitted in first-seen order (initial first, then each transition's From/To)
// so the alias declarations are deterministic.
//
// A transition whose Label is empty, or equals its To value, renders as a bare
// edge with no label (matching the convention that a move named after its
// target adds no information).
func StateDiagram(initial string, transitions []Transition) string {
	ids := map[string]string{}
	idFor := func(value string) string {
		if id, ok := ids[value]; ok {
			return id
		}
		id := fmt.Sprintf("s%d", len(ids))
		ids[value] = id
		return id
	}

	order := []string{}
	seen := map[string]bool{}
	note := func(v string) {
		if v != "" && !seen[v] {
			seen[v] = true
			order = append(order, v)
			idFor(v)
		}
	}
	note(initial)
	for _, tr := range transitions {
		note(tr.From)
		note(tr.To)
	}

	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	for _, v := range order {
		fmt.Fprintf(&b, "    state %s as %s\n", quoted(v), ids[v])
	}
	if initial != "" {
		fmt.Fprintf(&b, "    [*] --> %s\n", ids[initial])
	}
	for _, tr := range transitions {
		// Skip a transition referencing a state we never registered (From/To
		// empty): it cannot be drawn safely.
		if tr.From == "" || tr.To == "" {
			continue
		}
		label := Label(tr.Label)
		if label != "" && tr.Label != tr.To {
			fmt.Fprintf(&b, "    %s --> %s: %s\n", ids[tr.From], ids[tr.To], label)
		} else {
			fmt.Fprintf(&b, "    %s --> %s\n", ids[tr.From], ids[tr.To])
		}
	}
	return b.String()
}

// Node is one vertex of a flow graph: a stable Key (used for dedupe and edge
// references) and the Text shown in the box.
type Node struct {
	Key  string
	Text string
}

// Edge is one directed edge of a flow graph between two node keys, optionally
// labeled with the relation name.
type Edge struct {
	FromKey string
	ToKey   string
	Label   string
}

// Graph renders a mermaid `graph LR` (left-to-right flow). Nodes are declared
// once each in the given order with synthetic ids; edges reference those ids.
// An edge referencing a key with no matching node is skipped (it cannot be
// drawn). Duplicate nodes (same Key) and duplicate edges (same From/To/Label)
// are collapsed so a caller can pass a raw, possibly-redundant traversal.
func Graph(nodes []Node, edges []Edge) string {
	ids := map[string]string{}
	var b strings.Builder
	b.WriteString("graph LR\n")

	for _, n := range nodes {
		if _, ok := ids[n.Key]; ok {
			continue // already declared
		}
		id := fmt.Sprintf("n%d", len(ids))
		ids[n.Key] = id
		fmt.Fprintf(&b, "    %s[%s]\n", id, quoted(n.Text))
	}

	emitted := map[string]bool{}
	for _, e := range edges {
		from, ok1 := ids[e.FromKey]
		to, ok2 := ids[e.ToKey]
		if !ok1 || !ok2 {
			continue
		}
		dedupe := from + "\x00" + to + "\x00" + e.Label
		if emitted[dedupe] {
			continue
		}
		emitted[dedupe] = true
		if lbl := Label(e.Label); lbl != "" {
			fmt.Fprintf(&b, "    %s -->|%s| %s\n", from, quoted(e.Label), to)
		} else {
			fmt.Fprintf(&b, "    %s --> %s\n", from, to)
		}
	}
	return b.String()
}

// Label makes a string safe as the text of a mermaid quoted label. It flattens
// newlines (a raw newline ends the diagram line and lets the remainder inject a
// statement) and replaces the characters that break out of a quoted label —
// double quotes and angle brackets — with mermaid's HTML-entity forms. Mermaid
// does NOT honor backslash escaping (so fmt %q is unsafe); it decodes numeric
// and named entities inside quotes, so `#quot;` renders as a literal quote
// without terminating the label.
func Label(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, `"`, "#quot;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return strings.TrimSpace(s)
}

// quoted wraps an already-escaped Label in mermaid's double-quote delimiters.
func quoted(s string) string { return `"` + Label(s) + `"` }
