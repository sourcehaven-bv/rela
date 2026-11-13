package flowcfg

import (
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/mermaid"
)

// HeadlessPrefix is the prefix `rela flow show` puts before the name of a
// headless flow. A flows.yaml flow name cannot contain a dot
// ([flow.ValidName]), so a prefixed name never names a run-based flow. It
// is presentation only: no run, run store or flow view sees it.
const HeadlessPrefix = "schema."

// Name returns the name of the custom type the flow was compiled from.
func (m *HeadlessFlow) Name() string { return m.name }

// States returns the state names in declaration order: the values of the
// enum type.
func (m *HeadlessFlow) States() []string { return slices.Clone(m.states) }

// Initial returns the entry value: the only value a create may set.
func (m *HeadlessFlow) Initial() string { return m.def.Initial() }

// Mermaid renders the flow as a Mermaid state diagram: an entry arrow to
// the entry value and one edge per transition, labeled with the move's
// label, its guard permission and its `when:` source. Package mermaid
// flattens the labels, so a `when:` cannot break the diagram.
func (m *HeadlessFlow) Mermaid() string {
	var ts []mermaid.Transition
	for _, st := range m.states {
		for _, b := range m.exits[st] {
			ts = append(ts, mermaid.Transition{From: st, To: b.to, Label: headlessLabel(b)})
		}
	}
	return mermaid.StateDiagramAt(m.def.Initial(), ts)
}

func headlessLabel(b headlessBranch) string {
	var parts []string
	if b.label != "" {
		parts = append(parts, b.label)
	}
	if b.guard != "" {
		parts = append(parts, "needs "+b.guard)
	}
	if b.whenSource != "" {
		parts = append(parts, "when "+b.whenSource)
	}
	return strings.Join(parts, " ")
}

// Headless returns every headless flow, sorted by name.
//
// Nil: accepted, returns nil.
func (s *Transitions) Headless() []*HeadlessFlow {
	if s == nil {
		return nil
	}
	out := make([]*HeadlessFlow, 0, len(s.machines))
	for _, name := range sortedKeys(s.machines) {
		out = append(out, s.machines[name])
	}
	return out
}

// HeadlessFlow returns the headless flow compiled from the custom type
// name, without [HeadlessPrefix].
//
// Nil: accepted, reports false.
func (s *Transitions) HeadlessFlow(name string) (*HeadlessFlow, bool) {
	if s == nil {
		return nil, false
	}
	m, ok := s.machines[name]
	return m, ok
}

// Uses returns the `entity.property` pairs whose property follows the
// headless flow name, sorted. A custom type may be the type of several
// properties, so one flow can have several uses.
//
// Nil: accepted, returns nil.
func (s *Transitions) Uses(name string) []string {
	if s == nil {
		return nil
	}
	var out []string
	for et, props := range s.propType {
		for prop, typ := range props {
			if typ == name {
				out = append(out, et+"."+prop)
			}
		}
	}
	slices.Sort(out)
	return out
}
