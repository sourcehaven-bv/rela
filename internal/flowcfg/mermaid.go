package flowcfg

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/mermaid"
)

// Mermaid renders f as a Mermaid state diagram: an entry arrow to the
// initial state and one edge per branch and timer, labeled with what moves
// the run. The labels are flattened by package mermaid, so a guard cannot
// break the diagram.
func (f *Flow) Mermaid() string { return f.MermaidAt("") }

// MermaidAt renders the diagram [Flow.Mermaid] renders with the state
// current marked; an empty or unknown current marks nothing.
func (f *Flow) MermaidAt(current string) string {
	var ts []mermaid.Transition
	for _, st := range f.states {
		for _, ev := range Events {
			for _, b := range st.on[ev] {
				ts = append(ts, mermaid.Transition{From: st.Name, To: b.To, Label: branchLabel(ev, b)})
			}
		}
		for _, t := range st.timers {
			ts = append(ts, mermaid.Transition{From: st.Name, To: t.To, Label: "after " + t.Period.String()})
		}
	}
	return mermaid.StateDiagramAt(f.initial, ts, current)
}

func branchLabel(ev Event, b Branch) string {
	if b.Label != "" {
		return b.Label
	}
	parts := []string{string(ev)}
	if len(b.Roles) > 0 {
		parts = append(parts, "by "+strings.Join(b.Roles, ", "))
	}
	if b.When != nil {
		parts = append(parts, "when "+b.When.Source())
	}
	return strings.Join(parts, " ")
}
