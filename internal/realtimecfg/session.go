package realtimecfg

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// SessionNoticeType is the message type of the notice a session transition
// appends. A channel type with a session may not declare a message type of
// that name.
const SessionNoticeType = "session"

// Session is the compiled `session:` of a channel type: a flat state
// machine whose events are the channel's typed messages. It is immutable:
// its fields are unexported and every accessor returns a value or a copy,
// so one *Session is shared by every copy of its channel type.
type Session struct {
	label     string
	initial   string
	states    []SessionState
	index     map[string]int
	reminders []SessionReminder
	// guardReminders names every reminder whose name matches the grammar,
	// valid or not; the guards declare a counter for each.
	guardReminders []string
	// paused names the reminders paused in this copy (see [Session.Pausing]).
	paused map[string]bool
	// onFinal is `on_final`: [OnFinalClose] or empty.
	onFinal string
}

// Label is the session's display name.
func (s *Session) Label() string { return s.label }

// Initial is the state an unset session reads as.
func (s *Session) Initial() string { return s.initial }

// States returns the states in declaration order.
func (s *Session) States() []SessionState { return slices.Clone(s.states) }

// State returns the state called name.
func (s *Session) State(name string) (SessionState, bool) {
	i, ok := s.index[name]
	if !ok {
		return SessionState{}, false
	}
	return s.states[i], true
}

// Transition returns the transition out of state that a message of type
// msgType fires.
func (s *Session) Transition(state, msgType string) (Transition, bool) {
	st, ok := s.State(state)
	if !ok {
		return Transition{}, false
	}
	t, ok := st.on[msgType]
	if !ok {
		return Transition{}, false
	}
	t.Roles = slices.Clone(t.Roles)
	return t, true
}

// Drives reports whether a message of type msgType fires a transition out
// of some state.
func (s *Session) Drives(msgType string) bool {
	for _, st := range s.states {
		if _, ok := st.on[msgType]; ok {
			return true
		}
	}
	return false
}

// HasTimers reports whether some state of the session declares a timer.
func (s *Session) HasTimers() bool {
	for _, st := range s.states {
		if len(st.timers) > 0 {
			return true
		}
	}
	return false
}

// SessionState is one entry of `session.states`.
type SessionState struct {
	Name  string
	Label string
	// Final marks a state the session ends in. It has no transitions and
	// no timers.
	Final  bool
	on     map[string]Transition
	timers []SessionTimer
}

// Timers returns the state's timers, sorted by name. The slice is a copy.
func (st SessionState) Timers() []SessionTimer { return slices.Clone(st.timers) }

// TimerSince names what a timer counts from.
type TimerSince string

const (
	// TimerSinceState counts from when the session entered the state. It
	// is the default.
	TimerSinceState TimerSince = "state"
	// TimerSinceSession counts from when the channel was created.
	TimerSinceSession TimerSince = "session"
)

// SessionTimer is one `after.<name>` entry of a session state: once
// Period has passed since its anchor (see [TimerSince]) while the session
// is still in the state and the channel is open, the session moves to To.
type SessionTimer struct {
	Name   string
	Label  string
	To     string
	Period Period
	Since  TimerSince
}

// Transitions returns the state's transitions, sorted by message type.
func (st SessionState) Transitions() []Transition {
	out := make([]Transition, 0, len(st.on))
	for _, mt := range sortedKeys(st.on) {
		t := st.on[mt]
		t.Roles = slices.Clone(t.Roles)
		out = append(out, t)
	}
	return out
}

// TransitionBy limits which kind of poster fires a transition.
type TransitionBy string

const (
	// TransitionByAny lets every poster the message type admits fire the
	// transition, unless Roles narrows it.
	TransitionByAny TransitionBy = ""
	// TransitionByTrigger lets only a trigger post fire the transition.
	TransitionByTrigger TransitionBy = "trigger"
)

// Transition is one `on.<message type>` entry: a message of MessageType
// moves the session to To when its poster passes Roles or By and When
// holds.
type Transition struct {
	MessageType string
	To          string
	// Roles narrows the posters who fire the transition to the holders
	// of one of these roles. A trigger holds no role.
	Roles []string
	By    TransitionBy
	// When is the compiled guard over `message`, `session` and the
	// standard library, or nil.
	When *predicate.Program
	// outputs are the compiled `outputs:`, in file order.
	outputs []Output
	// warned is shared by every copy of the transition, so a guard failure
	// is reported once per loaded configuration.
	warned *atomic.Bool
}

// WarnGuardOnce reports true the first time it is called for this
// transition of this loaded configuration, and false after. A reload
// compiles new transitions, so each reload reports once again.
func (t Transition) WarnGuardOnce() bool {
	return t.warned != nil && t.warned.CompareAndSwap(false, true)
}

// Session returns the channel type's session, or nil when it has none.
func (ct *ChannelType) Session() *Session { return ct.session }

const stateRule = "state name must match ^[a-z][a-z0-9-]{0,62}$"

// session compiles `channel_types.<name>.session`. order is the key order
// of `session.states` in the file, or nil when it could not be read.
func (v *validator) session(
	key string, raw *rawSession, order []string, ct *ChannelType, rawCT *rawChannelType, limits Limits,
) *Session {
	s := &Session{label: orDefault(raw.Label, ct.Label), initial: raw.Initial, index: map[string]int{}}
	statesKey := key + ".states"
	switch {
	case len(raw.States) == 0:
		v.add(statesKey, "a session needs at least one state")
	case len(raw.States) > realtimelimits.MaxSessionStates:
		v.add(statesKey, fmt.Sprintf("at most %d states", realtimelimits.MaxSessionStates))
	}
	if len(order) != len(raw.States) {
		order = sortedKeys(raw.States)
	}
	if _, ok := ct.MessageTypes[SessionNoticeType]; ok {
		v.add(ct.messageTypesKey()+"."+SessionNoticeType, fmt.Sprintf("%q is reserved in a channel type with "+
			"a session: transitions post notices of that type; choose another message type name", SessionNoticeType))
	}
	for _, mn := range sortedKeys(ct.MessageTypes) {
		if _, ok := ct.MessageTypes[mn].Actions[OutputsActionName]; ok {
			v.add(ct.messageTypesKey()+"."+mn+".actions."+OutputsActionName, fmt.Sprintf("%q is reserved for "+
				"the session's output marks; choose another action name", OutputsActionName))
		}
	}
	switch raw.OnFinal {
	case "", OnFinalClose:
		s.onFinal = raw.OnFinal
	default:
		v.add(key+".on_final", "on_final may only be close")
	}
	for _, name := range order {
		if !namePattern.MatchString(name) {
			v.add(statesKey+"."+name, stateRule)
		}
		s.index[name] = len(s.states)
		s.states = append(s.states, SessionState{Name: name})
	}
	switch _, declared := s.index[raw.Initial]; {
	case raw.Initial == "":
		v.add(key+".initial", "name the state a new session starts in")
	case !declared:
		v.add(key+".initial", unknownState(raw.Initial))
	case raw.States[raw.Initial].Final:
		v.add(key+".initial", "the initial state cannot be final: an unset session reads as the "+
			"initial state, so no channel could ever move")
	}
	// Reminders come before the transitions: the guards declare their
	// counters.
	reminderCompiler{v: v, rawTypes: rawCT.MessageTypes, states: raw.States, limits: limits, s: s, ct: ct}.
		compile(key+".reminders", raw.Reminders)
	for i := range s.states {
		st := &s.states[i]
		rs := raw.States[st.Name]
		st.Label, st.Final, st.on = orDefault(rs.Label, st.Name), rs.Final, map[string]Transition{}
		sk := statesKey + "." + st.Name
		switch {
		case rs.Final && len(rs.On) > 0:
			v.add(sk+".on", "a final state has no transitions; drop on or remove final: true")
			continue
		case rs.Final && len(rs.After) > 0:
			v.add(sk+".after", "a final state has no timers; drop after or remove final: true")
			continue
		case !rs.Final && len(rs.On) == 0 && len(rs.After) == 0:
			v.add(sk, "a state that is not final needs a transition under on or a timer under after; "+
				"mark it final: true if the session ends there")
		}
		st.timers = compileTimers(v.add, sk+".after", rs.After, s)
		for _, mn := range sortedKeys(rs.On) {
			rt := rs.On[mn]
			if t, ok := v.transition(sk+".on."+mn, mn, &rt, s, ct); ok {
				st.on[mn] = t
			}
		}
	}
	timerLoops(v.add, statesKey, s)
	return s
}

// problemSink records a problem at a key; the validator's add is one. The
// timer checks take it rather than the validator, so they stay free
// functions over the session they check.
type problemSink func(key, reason string)

// compileTimers compiles a state's `after:`, sorted by timer name. A timer
// with a problem is left out.
func compileTimers(add problemSink, key string, raw map[string]rawSessionTimer, s *Session) []SessionTimer {
	if len(raw) > realtimelimits.MaxSessionTimers {
		add(key, fmt.Sprintf("at most %d timers per state", realtimelimits.MaxSessionTimers))
	}
	var out []SessionTimer
	for _, name := range sortedKeys(raw) {
		if t, ok := compileTimer(add, key+"."+name, name, raw[name], s); ok {
			out = append(out, t)
		}
	}
	return out
}

// compileTimer compiles one `after.<name>` entry. ok is false when it has
// a problem.
func compileTimer(add problemSink, key, name string, raw rawSessionTimer, s *Session) (SessionTimer, bool) {
	t := SessionTimer{Name: name, Label: orDefault(raw.Label, name), To: raw.To, Since: TimerSince(raw.Since)}
	ok := true
	bad := func(k, reason string) {
		add(k, reason)
		ok = false
	}
	if !namePattern.MatchString(name) {
		bad(key, "timer name must match ^[a-z][a-z0-9-]{0,62}$")
	}
	switch p, err := ParsePeriod(raw.Period); {
	case raw.Period == "":
		bad(key+".period", "name how long the timer waits, such as 72h")
	case err != nil:
		bad(key+".period", err.Error())
	default:
		if d, fixed := p.Fixed(); fixed && d < realtimelimits.MinSessionTimer {
			bad(key+".period", "a timer waits at least 1m")
		}
		t.Period = p
	}
	switch t.Since {
	case "":
		t.Since = TimerSinceState
	case TimerSinceState, TimerSinceSession:
	default:
		bad(key+".since", "since must be state or session")
	}
	if raw.To == "" {
		bad(key+".to", "name the state this timer moves to")
	} else if _, declared := s.index[raw.To]; !declared {
		bad(key+".to", unknownState(raw.To))
	}
	return t, ok
}

// timerLoops refuses every cycle made only of timer edges, self-loops
// included, at the timer that closes it: such a loop would move the session
// forever without a message. One depth-first search over the states.
func timerLoops(add problemSink, statesKey string, s *Session) {
	const (
		unseen = iota
		onPath
		done
	)
	mark := make([]int, len(s.states))
	var path []int
	var visit func(i int)
	visit = func(i int) {
		mark[i] = onPath
		path = append(path, i)
		st := s.states[i]
		for _, t := range st.timers {
			j, ok := s.index[t.To]
			if !ok {
				continue
			}
			switch mark[j] {
			case onPath:
				start := slices.Index(path, j)
				names := make([]string, 0, len(path)-start+1)
				for _, k := range path[start:] {
					names = append(names, s.states[k].Name)
				}
				names = append(names, s.states[j].Name)
				add(statesKey+"."+st.Name+".after."+t.Name, fmt.Sprintf("timers form a loop (%s) that "+
					"would move the session forever without a message; break it with a transition under on",
					strings.Join(names, " -> ")))
			case unseen:
				visit(j)
			}
		}
		path = path[:len(path)-1]
		mark[i] = done
	}
	for i := range s.states {
		if mark[i] == unseen {
			visit(i)
		}
	}
}

// messageTypesKey is the key path of ct's message types.
func (ct *ChannelType) messageTypesKey() string { return "channel_types." + ct.Name + ".message_types" }

func unknownState(name string) string {
	return fmt.Sprintf("unknown state %q; declare it under states", name)
}

// transition compiles one `on.<message type>` entry. ok is false when the
// message type is unusable, so the entry is left out.
func (v *validator) transition(
	key, msgType string, raw *rawTransition, s *Session, ct *ChannelType,
) (Transition, bool) {
	t := Transition{MessageType: msgType, To: raw.To, By: TransitionBy(raw.By)}
	mt, ok := ct.MessageTypes[msgType]
	switch {
	case !ok:
		v.add(key, fmt.Sprintf("unknown message type %q; a transition is keyed by a message type of "+
			"this channel type", msgType))
	case mt.Persist == PersistEphemeral:
		v.add(key, "an ephemeral message is never stored, so it cannot move a session")
		ok = false
	}
	if raw.To == "" {
		v.add(key+".to", "name the state this transition moves to")
	} else if _, declared := s.index[raw.To]; !declared {
		v.add(key+".to", unknownState(raw.To))
	}
	t.Roles = v.roleRefs(key+".roles", raw.Roles, ct)
	if ok {
		for i, r := range raw.Roles {
			if _, declared := ct.Roles[r]; declared && !slices.Contains(mt.Post, r) {
				v.add(fmt.Sprintf("%s.roles[%d]", key, i), fmt.Sprintf("role %q may not post %s, so it could "+
					"never fire this transition; add it to post or drop it here", r, msgType))
			}
		}
	}
	switch {
	case raw.By != "" && t.By != TransitionByTrigger:
		v.add(key+".by", "by must be trigger; agents are ordinary principals, so limit people and agents with roles")
	case t.By == TransitionByTrigger && len(raw.Roles) > 0:
		v.add(key+".by", "name roles or by: trigger, not both: a trigger holds no role")
	case t.By == TransitionByTrigger && ct.Cardinality == CardinalityMany:
		v.add(key+".by", "a trigger posts only into cardinality one channel types, so by: trigger never fires here")
	}
	if raw.When != "" && ok {
		t.When = v.guard(key+".when", raw.When, mt, ct, s.guardReminders)
		t.warned = &atomic.Bool{}
	}
	if len(raw.Outputs) > 0 && len(raw.Roles) == 0 && t.By == TransitionByAny {
		if tr := triggerReachable(v.file, ct.Name, msgType); tr != "" {
			v.add(key+".outputs", fmt.Sprintf("trigger %s posts %s into this channel type, so it can fire this "+
				"transition; outputs on a transition a trigger can fire need %s; add roles: so only people "+
				"fire it, or remove the outputs", tr, msgType, runAsLater))
		}
	}
	var cause *MessageType
	if ok {
		cause = mt
	}
	t.outputs = writeChecker{v}.outputs(key+".outputs", raw.Outputs, t, cause, ct)
	return t, ok
}

// guard compiles a transition's `when:` in [sessionGuardEnv]. A guard that
// reads the entity is refused with its own reason: this slice gives guards
// no entity, and the operator's way out is a trigger.
func (v *validator) guard(key, src string, mt *MessageType, ct *ChannelType, reminders []string) *predicate.Program {
	env, err := sessionGuardEnv(mt.Properties, reminders, nil)
	if err != nil {
		v.add(key, err.Error())
		return nil
	}
	prog, err := predicate.Compile(env, src)
	if err == nil {
		return prog
	}
	if v.readsEntity(src, mt, ct, reminders) {
		v.add(key, "transition guards cannot read the entity yet; use a trigger that posts a message")
		return nil
	}
	v.add(key, "does not compile: "+err.Error())
	return nil
}

// readsEntity reports whether src names `entity`: it compiles once `entity`
// is declared as the record of one of ct's attach_to types, or it fails on
// `entity` as an unknown identifier.
func (v *validator) readsEntity(src string, mt *MessageType, ct *ChannelType, reminders []string) bool {
	for _, t := range ct.AttachTo {
		def, ok := v.meta.GetEntityDef(t)
		if !ok {
			continue
		}
		env, err := sessionGuardEnv(mt.Properties, reminders, predicatefns.EntityRecordType(v.meta, def))
		if err != nil {
			continue
		}
		if prog, err := predicate.Compile(env, src); err == nil && prog.References("entity") {
			return true
		}
	}
	env, err := sessionGuardEnv(mt.Properties, reminders, nil)
	if err != nil {
		return false
	}
	_, err = predicate.Compile(env, src)
	var ce *predicate.CompileError
	return errors.As(err, &ce) && ce.Reason == `unknown identifier "entity"`
}

// sessionGuardEnv declares what a transition guard reads: `message`, typed
// as for the message type's own `when:`, the `session` record (state,
// generation, entered_at, and `reminders` with one counter per name in
// reminders) and the standard library. A non-nil entity is declared as
// `entity`; only the load check's probe for a guard that reads the entity
// passes one.
func sessionGuardEnv(props map[string]Property, reminders []string, entity predicate.Type) (*predicate.Env, error) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("message", messageRecordType(props)); err != nil {
		return nil, err
	}
	counters := make(predicate.RecordType, len(reminders))
	for _, name := range reminders {
		counters[name] = predicate.IntType
	}
	if err := env.DeclareVar("session", predicate.RecordType{
		"state":      predicate.StringType,
		"generation": predicate.IntType,
		"entered_at": predicate.DateTypeWithLayout(metamodel.DefaultDatetimeFormat),
		"reminders":  counters,
	}); err != nil {
		return nil, err
	}
	if entity != nil {
		if err := env.DeclareVar("entity", entity); err != nil {
			return nil, err
		}
	}
	if err := predicatefns.Declare(env); err != nil {
		return nil, err
	}
	return env, nil
}

// mappingKeys returns the keys of the mapping at path under root, in file
// order. It follows aliases. A merge key (<<) is refused: the decoded map
// would hold the merged keys while the walk saw only the alias, and the
// two must agree. found is false when there is no mapping at path.
func mappingKeys(root *yaml.Node, path ...string) (keys []string, found bool, err error) {
	n := resolveNode(root)
	for _, seg := range path {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil, false, nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == seg {
				next = resolveNode(n.Content[i+1])
			}
		}
		n = next
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, false, nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Tag == "!!merge" || k.Value == "<<" {
			return nil, true, errors.New("merge keys are not supported under session.states; write the states out")
		}
		keys = append(keys, k.Value)
	}
	return keys, true, nil
}

// resolveNode skips document nodes and follows aliases. A chain of aliases
// is bounded: yaml.v3 refuses an alias to a node that is not yet defined,
// so no chain loops.
func resolveNode(n *yaml.Node) *yaml.Node {
	for n != nil {
		switch {
		case n.Kind == yaml.DocumentNode && len(n.Content) > 0:
			n = n.Content[0]
		case n.Kind == yaml.AliasNode:
			n = n.Alias
		default:
			return n
		}
	}
	return nil
}
