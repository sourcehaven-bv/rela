package realtimecfg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// SessionStatus is what a session state means to an agent that waits on
// the session. The values are those of MCP tasks.
type SessionStatus string

// Session statuses. A final state reads [StatusCompleted] by default; a
// state that is not final reads [StatusInputRequired] when it has
// `waiting_for:`, else [StatusWorking].
const (
	// StatusWorking: the session moves on without a person named by the
	// state, through other roles, triggers or timers.
	StatusWorking SessionStatus = "working"
	// StatusInputRequired: the session waits for a holder of one of the
	// state's waiting roles to answer.
	StatusInputRequired SessionStatus = "input_required"
	// StatusCompleted: the session ended as intended.
	StatusCompleted SessionStatus = "completed"
	// StatusCancelled: the session ended because someone stopped it.
	StatusCancelled SessionStatus = "cancelled"
	// StatusFailed: the session ended in failure. A stranded session reads
	// as failed too, so an agent stops waiting.
	StatusFailed SessionStatus = "failed"
)

// sessionStatuses lists every status, in the order a problem names them.
var sessionStatuses = []SessionStatus{
	StatusWorking, StatusInputRequired, StatusCompleted, StatusCancelled, StatusFailed,
}

// Terminal reports whether s is a status no session leaves.
func (s SessionStatus) Terminal() bool {
	return s == StatusCompleted || s == StatusCancelled || s == StatusFailed
}

// WaitingFor returns the state's waiting roles, in file order: the roles
// whose holders the session waits for while it is in the state. Each is a
// bounded role (a from: entity.<property> role or a creator role) with an
// answer out of the state. The slice is a copy.
func (st SessionState) WaitingFor() []string { return slices.Clone(st.waitingFor) }

// StatusDeclared reports whether the file writes the state's `status:`.
// When it does not, [SessionState.Status] is the default.
func (st SessionState) StatusDeclared() bool { return st.statusDeclared }

// waitChecker checks the `status:` and `waiting_for:` of every session
// state, after the transitions and timers compiled, and gives the handoff
// and answer grant warnings. It keeps these checks off the validator,
// which is at its method budget.
type waitChecker struct{ *validator }

const (
	finalNoAnswer = "a final state has no transitions, so nothing can answer; use completed, cancelled or failed"
	boundedRule   = "waiting_for names a role with a known holder: a from: entity.<property> role or a creator role"
	deadlineReset = "a waiting role can fire moves that bring the session back into this state, which " +
		"restarts its since: state timers (and a reminder, when the moves leave its in: states), so a holder " +
		"could push back the deadline forever; use since: session for a deadline that survives handoffs"
)

// check sets each state's status and waiting roles and checks them.
func (w waitChecker) check(statesKey string, raw map[string]rawSessionState, s *Session, ct *ChannelType) {
	for i := range s.states {
		st := &s.states[i]
		rs := raw[st.Name]
		sk := statesKey + "." + st.Name
		st.waitingFor = w.waitingFor(sk, rs, st, ct)
		st.Status = w.status(sk, rs, st)
		st.statusDeclared = rs.Status != nil
		if len(st.waitingFor) > 0 && !st.Final {
			w.answers(sk, st, ct)
			w.deadlineReset(st, s, ct)
			w.answerGrants(st, ct)
		}
	}
	w.handoffs(s, ct)
}

// status parses a state's `status:` and applies the default and the rules
// that tie it to final and waiting_for. It reports one problem at most.
func (w waitChecker) status(sk string, rs rawSessionState, st *SessionState) SessionStatus {
	def := StatusWorking
	switch {
	case st.Final:
		def = StatusCompleted
	case len(rs.WaitingFor) > 0:
		def = StatusInputRequired
	}
	if rs.Status == nil {
		return def
	}
	got := SessionStatus(*rs.Status)
	key := sk + ".status"
	switch {
	case !slices.Contains(sessionStatuses, got):
		names := make([]string, len(sessionStatuses))
		for i, s := range sessionStatuses {
			names[i] = string(s)
		}
		w.add(key, fmt.Sprintf("status must be one of %s; got %q", strings.Join(names, ", "), got))
		return def
	case st.Final && !got.Terminal():
		w.add(key, finalNoAnswer)
	case !st.Final && got.Terminal():
		w.add(key, fmt.Sprintf("status %s ends the session; mark the state final: true", got))
	case got == StatusInputRequired && len(rs.WaitingFor) == 0:
		w.add(key, "name whom the session waits for under waiting_for")
	case got != StatusInputRequired && len(rs.WaitingFor) > 0 && !st.Final:
		w.add(key, fmt.Sprintf("a state with waiting_for has status input_required; drop status %s or "+
			"waiting_for", got))
	}
	return got
}

// waitingFor checks a state's `waiting_for:` names: declared, listed
// once, and bounded. It returns the names as given.
func (w waitChecker) waitingFor(sk string, rs rawSessionState, st *SessionState, ct *ChannelType) []string {
	key := sk + ".waiting_for"
	switch {
	case rs.WaitingFor == nil:
		return nil
	case len(rs.WaitingFor) == 0:
		w.add(key, "name at least one role, or drop waiting_for")
		return nil
	case st.Final:
		w.add(key, finalNoAnswer)
		return nil
	}
	for i, name := range rs.WaitingFor {
		ik := fmt.Sprintf("%s[%d]", key, i)
		r, declared := ct.Roles[name]
		switch {
		case !declared && name == "read":
			w.add(ik, `unknown role "read"; the read role is not under roles, so declare a role there`)
		case !declared:
			w.add(ik, fmt.Sprintf("unknown role %q; declare it under roles", name))
		case slices.Index(rs.WaitingFor, name) < i:
			w.add(ik, fmt.Sprintf("role %q is listed twice", name))
		default:
			if reason := unboundedRole(r); reason != "" {
				w.add(ik, reason+"; "+boundedRule)
			}
		}
	}
	return slices.Clone(rs.WaitingFor)
}

// unboundedRole says who holds r when it is not a bounded role, or ""
// when it is one.
func unboundedRole(r Role) string {
	switch r.Source.Kind {
	case SourceEntityProperty, SourceCreator:
		return ""
	case SourcePermission:
		return fmt.Sprintf("role %s is held by everyone with permission %s", r.Name, r.Source.Permission)
	case SourceAttached:
		return fmt.Sprintf("role %s is held by everyone who can read the entity", r.Name)
	case SourcePrincipalType:
		return fmt.Sprintf("role %s is held by every principal of type %s", r.Name, r.Source.PrincipalType)
	}
	return fmt.Sprintf("role %s has no valid source", r.Name)
}

// fireableBy reports whether a holder of role can post a message that
// fires entry t of st: the role may post the message type, t admits the
// role (it is not by: trigger, and its roles, when set, include the role),
// and no earlier entry of its list without `when:` admits the role first
// (see SessionState.firesFor). A guard is not evaluated.
func fireableBy(st SessionState, t Transition, role string, mt *MessageType) bool {
	return mt != nil && slices.Contains(mt.Post, role) && st.firesFor(t, posterClass{role: role})
}

// isAnswer reports whether a holder of role can fire t to leave st.
func isAnswer(st *SessionState, t Transition, role string, ct *ChannelType) bool {
	return t.To != st.Name && fireableBy(*st, t, role, ct.MessageTypes[t.MessageType])
}

// entries returns every transition entry of st, sorted by message type and
// then in file order. Unlike [SessionState.Transitions] it does not copy.
func (st SessionState) entries() []Transition {
	var out []Transition
	for _, mn := range sortedKeys(st.on) {
		out = append(out, st.on[mn]...)
	}
	return out
}

// answers refuses a waiting role that cannot post a message that moves
// the session out of st: the session would wait for it forever.
func (w waitChecker) answers(sk string, st *SessionState, ct *ChannelType) {
	for i, role := range st.waitingFor {
		if _, ok := ct.Roles[role]; !ok {
			continue
		}
		answered := false
		for _, t := range st.entries() {
			answered = answered || isAnswer(st, t, role, ct)
		}
		if !answered {
			w.add(fmt.Sprintf("%s.waiting_for[%d]", sk, i), fmt.Sprintf("role %s cannot post a message that "+
				"moves the session out of %s, so the session would wait for it forever", role, st.Name))
		}
	}
}

// deadlineReset refuses, in a state st with since: state timers or
// reminders, every transition out of st that starts a chain of person
// moves back into st that st's waiting roles can fire (each hop by the
// fireableBy rule). Coming back restarts st's since: state timers. A
// reminder's span goes on across moves inside its in: states, so the
// chain restarts a reminder only when it passes a state outside them.
// Timers and trigger-only moves are not chains a holder controls.
func (w waitChecker) deadlineReset(st *SessionState, s *Session, ct *ChannelType) {
	stateTimers := slices.ContainsFunc(st.timers, func(t SessionTimer) bool { return t.Since == TimerSinceState })
	reminders := s.RemindersIn(st.Name)
	if !stateTimers && len(reminders) == 0 {
		return
	}
	holder := func(from SessionState, t Transition) bool {
		return slices.ContainsFunc(st.waitingFor, func(role string) bool {
			return fireableBy(from, t, role, ct.MessageTypes[t.MessageType])
		})
	}
	leaves := func(state string) bool {
		return slices.ContainsFunc(reminders, func(r SessionReminder) bool { return !r.Active(state) })
	}
	for _, t := range st.entries() {
		if holder(*st, t) && returnsTo(st.Name, t.To, s, holder, leaves, stateTimers) {
			w.add(t.key, deadlineReset)
		}
	}
}

// returnsTo reports whether a chain of moves holder admits leads from
// from back into home and restarts something there: since: state timers
// (stateTimers) or a reminder, which needs the chain to pass a state where
// leaves holds. It walks each (state, left) pair once.
func returnsTo(
	home, from string, s *Session, holder func(SessionState, Transition) bool, leaves func(string) bool,
	stateTimers bool,
) bool {
	type node struct {
		state string
		left  bool
	}
	start := node{from, leaves(from)}
	seen := map[node]bool{start: true}
	queue := []node{start}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.state == home {
			if stateTimers || n.left {
				return true
			}
			continue
		}
		st, ok := s.State(n.state)
		if !ok {
			continue
		}
		for _, t := range st.entries() {
			if !holder(st, t) {
				continue
			}
			next := node{t.To, n.left || leaves(t.To)}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// answerGrants warns, once per output, when no role of acl.yaml grants a
// write an answer's output needs: no holder could post the answer without
// session_output_denied. The check is a floor: a local or asserted role
// may still grant the write at request time, so it is a warning.
func (w waitChecker) answerGrants(st *SessionState, ct *ChannelType) {
	if w.policy == nil {
		return
	}
	for _, t := range st.entries() {
		if !slices.ContainsFunc(st.waitingFor, func(role string) bool { return isAnswer(st, t, role, ct) }) {
			continue
		}
		for _, o := range t.outputs {
			if what := w.ungranted(o, ct); what != "" {
				w.warn(o.key, fmt.Sprintf("no role in acl.yaml grants %s, so no holder could post this answer "+
					"without session_output_denied; a local or asserted role may still grant it", what))
			}
		}
	}
}

// ungranted names the first write of o that no role grants, or "".
func (w waitChecker) ungranted(o Output, ct *ChannelType) string {
	if o.Kind == OutputCreate {
		if !acl.AnyRoleGrants(w.policy, acl.OpCreate, o.Type, entity.Face(o.Face), "") {
			return "create of " + faceAddr(o.Type, entity.Face(o.Face))
		}
		return ""
	}
	for _, typ := range ct.AttachTo {
		if _, ok := w.meta.GetEntityDef(typ); !ok {
			continue
		}
		faces := []entity.Face{entity.Face(o.Face)}
		if ct.Anchor == AnchorFace || len(metamodel.FaceOrderOf(w.meta, typ)) == 0 {
			faces = declaredFaces(w.meta, typ)
		}
		for _, face := range faces {
			for _, name := range o.PropertyNames() {
				if !acl.AnyRoleGrants(w.policy, acl.OpUpdate, typ, face, name) {
					return fmt.Sprintf("update of %s.%s on %s", typ, name, faceAddr(typ, face))
				}
			}
		}
	}
	return ""
}

// handoffs warns, when acl.yaml names a user entity type, at every set
// output property that writes a named role's property from a value that
// need not name a user: free text, or a reference other than
// message.author or a message ref to the user type. Such a value may match
// nobody, or the wrong principal.
func (w waitChecker) handoffs(s *Session, ct *ChannelType) {
	if w.policy == nil || strings.TrimSpace(w.policy.UserEntityType) == "" {
		return
	}
	roles := map[string]string{}
	for _, name := range sortedKeys(ct.Roles) {
		if r := ct.Roles[name]; r.Source.Kind == SourceEntityProperty {
			if _, seen := roles[r.Source.Property]; !seen {
				roles[r.Source.Property] = name
			}
		}
	}
	if ct.Read.Source.Kind == SourceEntityProperty {
		if _, seen := roles[ct.Read.Source.Property]; !seen {
			roles[ct.Read.Source.Property] = "read"
		}
	}
	if len(roles) == 0 {
		return
	}
	for _, st := range s.states {
		for _, t := range st.timers {
			w.handoffOutputs(t.outputs, nil, roles)
		}
		for _, t := range st.entries() {
			w.handoffOutputs(t.outputs, ct.MessageTypes[t.MessageType], roles)
		}
	}
}

// handoffOutputs is [waitChecker.handoffs] for the outputs of one move
// whose cause message type is mt, nil for a timer.
func (w waitChecker) handoffOutputs(outs []Output, mt *MessageType, roles map[string]string) {
	user := strings.TrimSpace(w.policy.UserEntityType)
	for _, o := range outs {
		if o.Kind != OutputSet {
			continue
		}
		for _, name := range o.PropertyNames() {
			role, ok := roles[name]
			if !ok || namesUser(o.props[name], mt, user) {
				continue
			}
			w.warn(o.key+".properties."+name, fmt.Sprintf("this writes the property of role %s from a value "+
				"that need not name a user; write a literal, {{message.author}} or a message ref to %s, or the "+
				"role may match nobody or the wrong principal", role, user))
		}
	}
}

// namesUser reports whether t writes a value that names a user of type
// user: a literal, the message author, or a whole message ref property
// that names only that type.
func namesUser(t Template, mt *MessageType, user string) bool {
	switch {
	case t.isLit, t.ref == refAuthor:
		return true
	case mt == nil:
		return false
	}
	name := t.messageRef()
	if name == "" {
		return false
	}
	p, ok := mt.Properties[name]
	return ok && p.Type == PropertyRef && len(p.To) > 0 &&
		!slices.ContainsFunc(p.To, func(to string) bool { return to != user })
}
