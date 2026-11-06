package flow

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Definition is the compiled machine the engine runs: its states, their
// timers and the reminders. It is immutable: its fields are unexported and
// every accessor returns a value or a copy, so one *Definition is shared by
// every reader.
type Definition struct {
	initial   string
	states    []State
	index     map[string]int
	reminders []Reminder
	// paused names the reminders paused in this copy (see
	// [Definition.Pausing]).
	paused map[string]bool
}

// NewDefinition returns the definition with the given initial state,
// states in declaration order, and reminders, which it sorts by name. The
// loader validated them; a state named twice keeps its first entry.
func NewDefinition(initial string, states []State, reminders []Reminder) *Definition {
	d := &Definition{
		initial: initial, states: slices.Clone(states), index: make(map[string]int, len(states)),
		reminders: slices.Clone(reminders),
	}
	slices.SortStableFunc(d.reminders, func(a, b Reminder) int { return strings.Compare(a.Name, b.Name) })
	for i, st := range d.states {
		if _, dup := d.index[st.name]; !dup {
			d.index[st.name] = i
		}
	}
	return d
}

// Initial is the state an unset instance reads as.
func (d *Definition) Initial() string { return d.initial }

// State returns the state called name.
func (d *Definition) State(name string) (State, bool) {
	i, ok := d.index[name]
	if !ok {
		return State{}, false
	}
	return d.states[i], true
}

// Reminders returns the reminders, sorted by name. The slice is a copy.
func (d *Definition) Reminders() []Reminder { return slices.Clone(d.reminders) }

// Reminder returns the reminder called name.
func (d *Definition) Reminder(name string) (Reminder, bool) {
	for _, r := range d.reminders {
		if r.Name == name {
			return r, true
		}
	}
	return Reminder{}, false
}

// RemindersIn returns the reminders that run in state, sorted by name.
func (d *Definition) RemindersIn(state string) []Reminder {
	var out []Reminder
	for _, r := range d.reminders {
		if r.Active(state) {
			out = append(out, r)
		}
	}
	return out
}

// Pausing returns a copy of d in which the named reminders are paused. A
// paused reminder stays declared, active in its states and counted, and
// its records are kept; only [Definition.Paused] reports it, so
// [NextReminder] gives it no slot. d is not changed.
func (d *Definition) Pausing(names []string) *Definition {
	c := *d
	c.paused = make(map[string]bool, len(d.paused)+len(names))
	maps.Copy(c.paused, d.paused)
	for _, n := range names {
		c.paused[n] = true
	}
	return &c
}

// Paused reports whether reminder name is paused in this copy (see
// [Definition.Pausing]).
func (d *Definition) Paused(name string) bool { return d.paused[name] }

// State is one state of a definition: its name, whether the instance ends
// there, and its timers sorted by name.
type State struct {
	name   string
	final  bool
	timers []Timer
}

// NewState returns the state name with timers, which it sorts by name. A
// final state has no timers; the loader refuses timers on one.
func NewState(name string, final bool, timers []Timer) State {
	st := State{name: name, final: final, timers: slices.Clone(timers)}
	slices.SortStableFunc(st.timers, func(a, b Timer) int { return strings.Compare(a.Name, b.Name) })
	return st
}

// Name is the state's name.
func (st State) Name() string { return st.name }

// Final reports whether the instance ends in the state.
func (st State) Final() bool { return st.final }

// Timers returns the state's timers, sorted by name. The slice is a copy.
func (st State) Timers() []Timer { return slices.Clone(st.timers) }

// Since names what a timer counts from.
type Since string

const (
	// SinceState counts from when the instance entered the state. It is
	// the default.
	SinceState Since = "state"
	// SinceSession counts from when the instance was created.
	SinceSession Since = "session"
)

// Timer is one timer of a state: once Period has passed since its anchor
// (see [Since]) while the instance is still in the state and not paused,
// the instance moves to To.
type Timer struct {
	Name   string
	To     string
	Period Period
	Since  Since
}

// Reminder posts while an instance stays inside its states (its span):
// every Every, the first one First after the span started, at most Max
// per span (0: no limit). It never moves the instance.
//
// Its state set is unexported and [Reminder.In] returns a copy, so a copy
// of the value shares nothing a caller can change.
type Reminder struct {
	Name string
	// Every is the time between two reminders.
	Every time.Duration
	// First is the time from the start of the span to the first reminder.
	First time.Duration
	// Max caps the reminders of one span; 0 means no limit.
	Max   int64
	in    []string
	inSet map[string]bool
}

// NewReminder returns the reminder name that runs in the states in, in
// file order. Set Every, First and Max on the result.
func NewReminder(name string, in []string) Reminder {
	r := Reminder{Name: name, in: slices.Clone(in), inSet: make(map[string]bool, len(in))}
	for _, st := range in {
		r.inSet[st] = true
	}
	return r
}

// In returns the states the reminder runs in, in file order. The slice is
// a copy.
func (r Reminder) In() []string { return slices.Clone(r.in) }

// Active reports whether the reminder runs in state.
func (r Reminder) Active(state string) bool { return r.inSet[state] }
