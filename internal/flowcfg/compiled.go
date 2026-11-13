package flowcfg

import (
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// FileName is the file the flows live in, at the project root.
const FileName = "flows.yaml"

// Compiled is a loaded flows.yaml. It is immutable; one value is shared by
// every reader.
type Compiled struct {
	flows    map[string]*Flow
	warnings []Problem
}

// Flow returns the flow called name.
func (c *Compiled) Flow(name string) (*Flow, bool) {
	if c == nil {
		return nil, false
	}
	f, ok := c.flows[name]
	return f, ok
}

// Flows returns every flow, sorted by name.
func (c *Compiled) Flows() []*Flow {
	if c == nil {
		return nil
	}
	out := make([]*Flow, 0, len(c.flows))
	for _, name := range slices.Sorted(maps.Keys(c.flows)) {
		out = append(out, c.flows[name])
	}
	return out
}

// Warnings returns what the load noticed but accepted, in key order.
func (c *Compiled) Warnings() []Problem {
	if c == nil {
		return nil
	}
	return slices.Clone(c.warnings)
}

// PerSubject is `per_subject:`: how many unfinished runs of a flow one
// subject may carry at once.
type PerSubject string

const (
	// PerSubjectOne allows one unfinished run per subject. It is the
	// default: a second start while one runs is refused.
	PerSubjectOne PerSubject = "one"
	// PerSubjectMany allows any number of runs per subject.
	PerSubjectMany PerSubject = "many"
)

// RetentionAction is what happens to a finished run's header once its
// retention period has passed.
type RetentionAction string

const (
	// RetentionArchive keeps the header, marked archived. It is the default.
	RetentionArchive RetentionAction = "archive"
	// RetentionDelete deletes the header and its relations.
	RetentionDelete RetentionAction = "delete"
)

// Retention is `retention:`. A run's runtime state is always dropped when
// it finishes; Retention governs the header that remains.
type Retention struct {
	// After is how long after it finished a run's header is kept as is.
	// Zero: kept for ever.
	After  flow.Period
	Action RetentionAction
}

// StartOn names what starts a run.
type StartOn string

const (
	// StartCreate starts a run when a subject is created and, with a
	// `when:`, the new subject satisfies it.
	StartCreate StartOn = "create"
	// StartPredicate starts a run when a subject comes to satisfy `when:`.
	StartPredicate StartOn = "predicate"
	// StartManual starts a run on request of a person who holds the
	// trigger's permission.
	StartManual StartOn = "manual"
	// StartSchedule starts a run for every subject that satisfies `when:`
	// (or every subject, without one), once per `every:`.
	StartSchedule StartOn = "schedule"
)

// Start is one entry of `start:`.
type Start struct {
	On StartOn
	// When is the subject condition, or nil for none.
	When *Guard
	// Permission is the permission a manual start requires.
	Permission string
	// Every is the period of a scheduled start.
	Every flow.Period
}

// Guard is a compiled `when:` over the subject, bound as `entity`.
type Guard struct {
	program *predicate.Program
	source  string
	fields  []string
}

// Program is the compiled condition. It is shared and read-only.
func (g *Guard) Program() *predicate.Program { return g.program }

// Source is the condition as written.
func (g *Guard) Source() string { return g.source }

// Fields returns the subject properties the condition reads.
func (g *Guard) Fields() []string { return slices.Clone(g.fields) }

// Flow is one compiled entry of `flows:`. It is immutable.
type Flow struct {
	name       string
	label      string
	subject    string
	runAs      string
	perSubject PerSubject
	deadline   flow.Period
	retention  Retention
	starts     []Start
	initial    string
	states     []State
	index      map[string]int
	version    string
	source     []byte
	def        *flow.Definition
	// outsideEdits is nil when the flow declares no `outside_edits:`.
	outsideEdits *OutsideEdits
	// forks are the flow's forks; region maps each state of a fork's
	// branch to its fork, by index, and its branch: a fork target.
	forks  []Fork
	region map[string]regionOf
}

// regionOf places a state in a fork's branch.
type regionOf struct {
	fork   int
	branch string
}

// Forks returns the flow's forks, in file order of their states.
func (f *Flow) Forks() []Fork {
	out := slices.Clone(f.forks)
	for i := range out {
		out[i].Targets = slices.Clone(out[i].Targets)
	}
	return out
}

// Region returns the fork and the branch, by its first state, a state
// belongs to; ok is false for a state outside every branch.
func (f *Flow) Region(state string) (fork Fork, branch string, ok bool) {
	r, ok := f.region[state]
	if !ok {
		return Fork{}, "", false
	}
	return f.Forks()[r.fork], r.branch, true
}

// Name is the flow's key in flows.yaml.
func (f *Flow) Name() string { return f.name }

// Label is the flow's display name: `label:`, or its name.
func (f *Flow) Label() string { return f.label }

// Subject is the entity type a run carries.
func (f *Flow) Subject() string { return f.subject }

// RunAs is the job identity the flow's outputs write as, or empty for a
// flow without outputs.
func (f *Flow) RunAs() string { return f.runAs }

// PerSubject is how many unfinished runs one subject may carry.
func (f *Flow) PerSubject() PerSubject { return f.perSubject }

// Deadline is how long a run may take before it reads overdue. Zero: no
// deadline.
func (f *Flow) Deadline() flow.Period { return f.deadline }

// Retention is what happens to a finished run's header.
func (f *Flow) Retention() Retention { return f.retention }

// Starts returns the start triggers in file order. The slice is a copy.
func (f *Flow) Starts() []Start { return slices.Clone(f.starts) }

// Initial is the state a run starts in.
func (f *Flow) Initial() string { return f.initial }

// States returns the states in file order. The slice is a copy.
func (f *Flow) States() []State {
	out := slices.Clone(f.states)
	for i := range out {
		out[i] = out[i].clone()
	}
	return out
}

// State returns the state called name.
func (f *Flow) State(name string) (State, bool) {
	i, ok := f.index[name]
	if !ok {
		return State{}, false
	}
	return f.states[i].clone(), true
}

// Version is the flow's definition version: a hash of its own
// declaration, without comments. Editing another flow leaves it alone.
func (f *Flow) Version() string { return f.version }

// Source is the flow's declaration as the hash reads it: the snapshot a
// run store keeps of a version at first use. The slice is a copy.
func (f *Flow) Source() []byte { return slices.Clone(f.source) }

// Definition is the engine's view of the flow: its states and their
// timers. Flows declare no reminders.
func (f *Flow) Definition() *flow.Definition { return f.def }

// OutsideEdits is the flow's `outside_edits:`, or nil when it declares
// none.
func (f *Flow) OutsideEdits() *OutsideEdits { return f.outsideEdits }

// Branches returns the entries of `on.<event>` out of state, in the order
// the engine tries them. Nil when the state is not declared or the event
// moves nothing out of it.
func (f *Flow) Branches(state string, event Event) []Branch {
	st, ok := f.State(state)
	if !ok {
		return nil
	}
	return st.on[event]
}

// Event names what moves a run out of a state, besides its timers.
type Event string

const (
	// EventChange is a change to the subject: an update by a person or by
	// another write. A branch's roles name who may cause it; its `when:`
	// reads the subject after the change.
	EventChange Event = "change"
	// EventDone is the completion of the state's step: the state's outputs
	// were written. A state without outputs completes on entry.
	EventDone Event = "done"
)

// Events are the event names `on:` accepts, in the order [State.Events]
// lists them.
var Events = []Event{EventChange, EventDone}

// State is one compiled entry of `states:`.
type State struct {
	Name  string
	Label string
	Final bool
	// Status is what the state means to someone who waits on the run:
	// `status:`, or the default (see [flow.Status]).
	Status     flow.Status
	waitingFor []string
	writes     []string
	outputs    []Output
	on         map[Event][]Branch
	timers     []Timer
	// join is nil for a state that closes no fork.
	join *Join
	// job is nil for a state that runs no job.
	job *Job
}

// Job returns the state's `job:`; ok is false for a state that runs none.
func (st State) Job() (Job, bool) {
	if st.job == nil {
		return Job{}, false
	}
	return *st.job, true
}

// Join returns the state's `join:`; ok is false for a state that closes
// no fork.
func (st State) Join() (Join, bool) {
	if st.join == nil {
		return Join{}, false
	}
	return *st.join, true
}

func (st State) clone() State {
	st.waitingFor = slices.Clone(st.waitingFor)
	st.writes = slices.Clone(st.writes)
	st.outputs = slices.Clone(st.outputs)
	st.timers = slices.Clone(st.timers)
	if st.job != nil {
		j := *st.job
		j.Capabilities.Secrets = slices.Clone(j.Capabilities.Secrets)
		j.Capabilities.Tokens = slices.Clone(j.Capabilities.Tokens)
		st.job = &j
	}
	on := make(map[Event][]Branch, len(st.on))
	for e, bs := range st.on {
		bs = slices.Clone(bs)
		for i := range bs {
			bs[i].Roles = slices.Clone(bs[i].Roles)
			bs[i].Fork = slices.Clone(bs[i].Fork)
		}
		on[e] = bs
	}
	st.on = on
	return st
}

// WaitingFor returns the acl.yaml roles the state waits on, in file order.
func (st State) WaitingFor() []string { return slices.Clone(st.waitingFor) }

// Writes returns the subject properties a person who does the state's step
// fills in (`writes:`), in file order. Empty: the step has no form.
func (st State) Writes() []string { return slices.Clone(st.writes) }

// Outputs returns what a run writes when it enters the state, in file
// order.
func (st State) Outputs() []Output { return slices.Clone(st.outputs) }

// Timers returns the state's timers, sorted by name.
func (st State) Timers() []Timer { return slices.Clone(st.timers) }

// Events returns the events that move a run out of the state, in the
// order of [Events].
func (st State) Events() []Event {
	var out []Event
	for _, e := range Events {
		if len(st.on[e]) > 0 {
			out = append(out, e)
		}
	}
	return out
}

// Branches returns the entries of `on.<e>`, in file order.
func (st State) Branches(e Event) []Branch { return slices.Clone(st.on[e]) }

// Branch is one entry of `on.<event>`.
type Branch struct {
	// To is the state the branch moves to; empty for a fork.
	To string
	// Fork are the states a fork moves to, in file order: 2 to
	// [MaxForkTargets] of them. Nil for an ordinary branch.
	Fork  []string
	Label string
	// Roles are the acl.yaml roles, one of which the author of a change
	// must hold on the subject. Empty: anyone whose write reaches the
	// subject. A `done` branch has none.
	Roles []string
	// When is the guard over the subject, or nil for none.
	When *Guard
}

// Targets returns the states the branch moves to: To, or the fork's
// targets.
func (b Branch) Targets() []string {
	if b.Fork != nil {
		return slices.Clone(b.Fork)
	}
	return []string{b.To}
}

// MaxForkTargets caps the targets of one fork. It equals
// flowrun.MaxBranches, which caps the branches a run stores.
const MaxForkTargets = 8

// Join is a state's `join:`: when the branches of the fork it closes
// continue as one run.
type Join struct {
	// Any continues with the first branch that arrives (`join: any`).
	Any bool
	// When continues early, once it holds after an arrival
	// (`join: {when:}`). Nil: wait for every branch, unless Any.
	When *Guard
}

// Fork is one fork of a flow: a branch with several targets and the join
// that closes it.
type Fork struct {
	// State is the state the fork leaves.
	State string
	// Targets are the states the branches start in, in file order. Each
	// names its branch.
	Targets []string
	// Join is the state that closes the fork.
	Join string
	// Waits reports that a state of one of the fork's branches waits for
	// a role.
	Waits bool
}

// Timer is one entry of a state's `after:`.
type Timer struct {
	flow.Timer
	Label string
}

// OutputKind is what an output writes.
type OutputKind string

const (
	// OutputSet patches properties of the subject.
	OutputSet OutputKind = "set"
	// OutputCreate creates an entity, optionally related to the subject.
	OutputCreate OutputKind = "create"
)

// Output is one entry of a state's `outputs:`.
type Output struct {
	Kind OutputKind
	// Type is the entity type a create makes; the subject type for a set.
	Type string
	// Relate is the relation type a create links the new entity to the
	// subject with, or empty.
	Relate string
	props  map[string]Template
}

// Properties returns the property templates by property name. The map is
// a copy.
func (o Output) Properties() map[string]Template { return maps.Clone(o.props) }

// PropertyNames returns the names of the output's properties, sorted.
func (o Output) PropertyNames() []string { return slices.Sorted(maps.Keys(o.props)) }
