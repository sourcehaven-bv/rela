package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// SessionView is a channel's session as a reader sees it. Transitions are
// the `on:` entries of the current state, sorted by message type: config,
// the same for every reader. They are empty when the state is final or
// stranded.
type SessionView struct {
	State string `json:"state"`
	Label string `json:"label,omitempty"`
	// Status is what the state means to an agent, one of the
	// [realtimecfg.SessionStatus] values, read from the current
	// configuration. A stranded state reads [realtimecfg.StatusFailed], so
	// an agent stops waiting; Stranded says why.
	Status string `json:"status"`
	// WaitingFor are the state's waiting roles, in config order, while the
	// status is input_required. A named role carries the holders the
	// reader's redacted anchor view shows.
	WaitingFor []SessionWaitingView `json:"waiting_for,omitempty"`
	// WaitingOnMe is set when the reader holds a waiting role for update
	// on the anchor, the channel is stored and open, the status is
	// input_required and the configuration is not degraded. It is per
	// reader, like the affordances.
	WaitingOnMe bool `json:"waiting_on_me,omitempty"`
	// Generation counts the transitions; 0 means the session never moved.
	Generation int64 `json:"generation"`
	// EnteredAt is when the session entered its state: the time of the
	// causing message, or for a session that never moved, the channel's
	// created_at. It is zero, and left out, for a channel that is not
	// stored yet, so two reads of it render the same.
	EnteredAt time.Time `json:"entered_at,omitzero"`
	Final     bool      `json:"final,omitempty"`
	// Stranded is set when the stored state is one the configuration no
	// longer declares. Label is then the stored name, and nothing moves the
	// session until the state is declared again.
	Stranded    bool                    `json:"stranded,omitempty"`
	Transitions []SessionTransitionView `json:"transitions,omitempty"`
	// DueAt is when the first of Timers comes due. It is derived from the
	// configuration and the channel, and set only on a stored, open channel
	// under a configuration that is not degraded (see [SessionTimerView]).
	DueAt *time.Time `json:"due_at,omitempty"`
	// Timers are the `after:` entries of the current state, sorted by name:
	// config, the same for every reader. They are empty when the state is
	// final or stranded, or the channel is not stored yet.
	Timers []SessionTimerView `json:"timers,omitempty"`
	// Reminders are the reminders active in the current state, sorted by
	// name. They are empty when the state is final or stranded, or the
	// channel is not stored yet. DueAt above stays the next move; each
	// reminder carries its own due time.
	Reminders []SessionReminderView `json:"reminders,omitempty"`
}

// SessionReminderView is one reminder active in the current state. Count
// is the number of reminders posted in the current span. DueAt is the next
// slot, set on a stored, open channel under a configuration that is not
// degraded, and not once Count reaches Max.
type SessionReminderView struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	// Every is the interval as configured, such as "30m".
	Every string `json:"every"`
	Count int64  `json:"count"`
	// Max is the cap on Count; 0 means no cap.
	Max   int64      `json:"max,omitempty"`
	DueAt *time.Time `json:"due_at,omitempty"`
}

// SessionTimerView is one timer of the current state. DueAt is set on a
// stored, open channel under a configuration that is not degraded. A
// closed channel's timers are frozen, and under a degraded configuration
// the leader fires none, so neither shows a due time.
type SessionTimerView struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	To    string `json:"to"`
	// Since is what the timer counts from: state or session.
	Since string     `json:"since"`
	DueAt *time.Time `json:"due_at,omitempty"`
	// Outputs is the number of graph outputs the timer's move writes, as
	// the session's run_as. It is config, the same for every reader.
	Outputs int `json:"outputs,omitempty"`
}

// SessionTransitionView is one transition out of the current state: the
// message type that fires it, the state it moves to and, when only a
// trigger may fire it, By "trigger".
type SessionTransitionView struct {
	MessageType string `json:"message_type"`
	To          string `json:"to"`
	By          string `json:"by,omitempty"`
	// Guarded is set when the transition has a `when:`, so a post of the
	// message type moves the session only if the guard holds for it. It is
	// config, the same for every reader.
	Guarded bool `json:"guarded,omitempty"`
	// Outputs is the number of graph outputs the transition writes. It is
	// config, the same for every reader.
	Outputs int `json:"outputs,omitempty"`
	// Entry is the transition's place in its `on.<type>` list, from 0. An
	// [EntryRef] names the transition by it. Config, the same for every
	// reader.
	Entry int `json:"entry"`
	// Button is the entry's one-click button, or nil. Config, the same for
	// every reader.
	Button *SessionButtonView `json:"button,omitempty"`
}

// SessionButtonView is a transition entry's one-click button: a click
// posts the entry's message type with Props, in stored form.
type SessionButtonView struct {
	Label string         `json:"label"`
	Props map[string]any `json:"props"`
	// ButtonOptionsView holds the button's options, with its type's
	// `decision_button:` already applied.
	ButtonOptionsView
}

// ButtonOptionsView is how the web app draws a decision button: the
// authored `confirm:` and `style:` (see [realtimecfg.ButtonOptions]).
// They do not change what a post does. Config, the same for every
// reader.
//
// Confirm is nil when the file leaves the question to the derived rule.
// ConfirmText is set only with Confirm true, so `confirm: false` never
// carries a text. Style is empty when the file sets none.
type ButtonOptionsView struct {
	Confirm     *bool  `json:"confirm,omitempty"`
	ConfirmText string `json:"confirm_text,omitempty"`
	Style       string `json:"style,omitempty"`
}

// buttonOptionsView serves o.
func buttonOptionsView(o realtimecfg.ButtonOptions) ButtonOptionsView {
	v := ButtonOptionsView{Style: string(o.Style)}
	switch o.Confirm {
	case realtimecfg.ConfirmAsk:
		ask := true
		v.Confirm, v.ConfirmText = &ask, o.ConfirmText
	case realtimecfg.ConfirmNever:
		never := false
		v.Confirm = &never
	case realtimecfg.ConfirmDerived:
		// Left out: the web app derives it.
	}
	return v
}

// decisionButtonView serves a type's `decision_button:`; nil for nil.
func decisionButtonView(o *realtimecfg.ButtonOptions) *ButtonOptionsView {
	if o == nil {
		return nil
	}
	v := buttonOptionsView(*o)
	return &v
}

// EntryRef names one transition entry of the current state: its message
// type and its place in the type's list.
type EntryRef struct {
	MessageType string `json:"message_type"`
	Entry       int    `json:"entry"`
}

// current is a channel's session read against the configuration.
type current struct {
	channels.Session
	stranded bool
}

// currentSession reads ch's session under s. An unset session reads as the
// configured initial state, entered when the channel was created. For a
// channel that is not stored yet, a decider passes a channel whose
// CreatedAt is the operation's clock time, the time the append will store;
// [sessionView] passes the zero time, so the view leaves entered_at out.
func currentSession(s *realtimecfg.Session, ch channels.Channel) current {
	if ch.Session.IsZero() {
		return current{Session: channels.Session{State: s.Initial(), EnteredAt: ch.CreatedAt}}
	}
	_, declared := s.State(ch.Session.State)
	return current{Session: ch.Session, stranded: !declared}
}

// sessionView renders the session of h under ct, or of the channel not
// stored yet when h is nil. Nil when ct has no session. degraded is the
// snapshot's flag: due times are left out under a degraded configuration.
// wf are the reader's facts for the waiting view (see [waitingView]).
func sessionView(ct realtimecfg.ChannelType, h *channels.Channel, degraded bool, wf waitingFacts) *SessionView {
	s := ct.Session()
	if s == nil {
		return nil
	}
	var ch channels.Channel
	if h != nil {
		ch = *h
	}
	cur := currentSession(s, ch)
	v := &SessionView{
		State: cur.State, Label: cur.State, Generation: cur.Generation, EnteredAt: cur.EnteredAt,
		Stranded: cur.stranded, Status: string(realtimecfg.StatusFailed),
	}
	st, ok := s.State(cur.State)
	if !ok {
		return v
	}
	v.Label, v.Final, v.Status = st.Label, st.Final, string(st.Status)
	v.WaitingFor, v.WaitingOnMe = waitingView(st, ct.Roles, wf)
	for _, tr := range st.Transitions() {
		tv := SessionTransitionView{
			MessageType: tr.MessageType, To: tr.To, By: string(tr.By), Guarded: tr.When != nil,
			Outputs: len(tr.Outputs()), Entry: tr.Index(),
		}
		if b, ok := tr.Button(); ok {
			tv.Button = &SessionButtonView{
				Label: b.Label, Props: b.Props(), ButtonOptionsView: buttonOptionsView(b.Options),
			}
		}
		v.Transitions = append(v.Transitions, tv)
	}
	if h != nil && !st.Final {
		showDue := h.State == channels.StateOpen && !degraded
		timerViews(v, cur, *h, st.Timers(), showDue)
		v.Reminders = reminderViews(s, cur, *h, showDue)
	}
	return v
}

// reminderViews are the views of the reminders active in ch's current
// state cur, with showDue their next slots.
func reminderViews(s *realtimecfg.Session, cur current, ch channels.Channel, showDue bool) []SessionReminderView {
	active := s.RemindersIn(cur.State)
	out := make([]SessionReminderView, 0, len(active))
	for _, r := range active {
		rec, _ := reminderRecord(r, cur, ch)
		rv := SessionReminderView{Name: r.Name, Label: r.Label, Every: r.EveryText, Count: rec.Count, Max: r.Max}
		if slot, ok := nextSlot(r, cur, ch); ok && showDue {
			rv.DueAt = &slot
		}
		out = append(out, rv)
	}
	return out
}

// timerViews fills v's timers for ch, whose current session is cur, and
// with showDue their due times and the session's.
func timerViews(v *SessionView, cur current, ch channels.Channel, timers []realtimecfg.SessionTimer, showDue bool) {
	for _, t := range timers {
		tv := SessionTimerView{
			Name: t.Name, Label: t.Label, To: t.To, Since: string(t.Since), Outputs: len(t.Outputs()),
		}
		if showDue {
			due := timerDue(t, cur, ch)
			tv.DueAt = &due
			if v.DueAt == nil || due.Before(*v.DueAt) {
				v.DueAt = &due
			}
		}
		v.Timers = append(v.Timers, tv)
	}
}

// poster is who posts a message that may move a session, as a decider
// sees it. The answers are computed before the append, because a role
// check reads ACL state and the decider runs under the store's lock.
type poster struct {
	// trigger is set for a trigger's post.
	trigger bool
	// roles holds, for a person or agent, the transition roles they hold.
	roles map[string]bool
}

// admits reports whether p passes t's poster rule,
// [realtimecfg.Transition.Admits], the rule the load checks judge entries
// by.
func (p poster) admits(t realtimecfg.Transition) bool {
	if p.trigger {
		return t.Admits(nil)
	}
	return t.Admits(func(role string) bool { return p.roles[role] })
}

// transitionRoles reports which roles named by a transition that mt fires
// the caller holds, through [call.heldRoles], so the client ceiling
// applies. A message type that fires no transition with roles asks
// nothing.
func (c *call) transitionRoles(mt *realtimecfg.MessageType) map[string]bool {
	s := c.ct.Session()
	if s == nil {
		return map[string]bool{}
	}
	var names []string
	for _, st := range s.States() {
		for _, t := range s.Branches(st.Name, mt.Name) {
			names = append(names, t.Roles...)
		}
	}
	return c.heldRoles(names)
}

// heldRoles reports [call.heldRole] for each name.
func (c *call) heldRoles(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = c.heldRole(name)
	}
	return out
}

// heldRole reports whether the caller holds the role name for update on
// the anchor, as [call.mayPost] asks, so the client ceiling applies. Each
// distinct role is asked once per call; the post path and [call.sessionAffordances]
// share the answers, so they cannot drift. An undeclared name is not held.
//
// A creator role read while the call holds no stored channel is answered
// but not kept: the creator is a fact of the stored channel, so an answer
// from before it exists must not outlive it within the call.
func (c *call) heldRole(name string) bool {
	if held, asked := c.ops.roles[name]; asked {
		return held
	}
	r, found := c.ct.Roles[name]
	held := found && c.can(r, realtimecfg.CapabilityUpdate)
	if found && r.Source.Kind == realtimecfg.SourceCreator && c.head == nil {
		return held
	}
	if c.ops.roles == nil {
		c.ops.roles = map[string]bool{}
	}
	c.ops.roles[name] = held
	return held
}

// sessionAffordances answers, over the post set post, which session moves
// the caller's post could make out of the current state. It reads the
// session as [sessionView] does, and only while the channel is open and
// the current state is declared and not final; otherwise both are empty.
// The caller's roles are looked up once, so a caller with two roles is
// judged on both. Neither is ever nil.
//
// types lists, sorted and once each, the message types in post with an
// entry whose poster rule ([poster.admits]) the caller passes, so a type
// with only by: trigger entries is never listed. A guard is not evaluated:
// it reads the message, which does not exist yet.
//
// entries lists, sorted by message type and entry, each entry of a type
// in post that the state offers the caller
// ([realtimecfg.SessionState.Offers]): the entries their post could take.
func (c *call) sessionAffordances(post map[string]bool) (types []string, entries []EntryRef) {
	types, entries = []string{}, []EntryRef{}
	s := c.ct.Session()
	if s == nil || c.state() != channels.StateOpen {
		return types, entries
	}
	cur := currentSession(s, sessionChannel(c.head, c.now))
	st, ok := s.State(cur.State)
	if !ok || st.Final {
		return types, entries
	}
	trs := st.Transitions()
	var names []string
	for _, t := range trs {
		if post[t.MessageType] {
			names = append(names, t.Roles...)
		}
	}
	p := poster{roles: c.heldRoles(names)}
	holds := func(role string) bool { return p.roles[role] }
	for _, t := range trs {
		if !post[t.MessageType] {
			continue
		}
		if p.admits(t) {
			types = append(types, t.MessageType)
		}
		if st.Offers(t, holds) {
			entries = append(entries, EntryRef{MessageType: t.MessageType, Entry: t.Index()})
		}
	}
	slices.Sort(types)
	return slices.Compact(types), entries
}

// sessionChannel is the channel a call reads the session of: its head, or
// for a channel not stored yet (nil head) one created at now, as an append
// would create it.
func sessionChannel(head *channels.Channel, now time.Time) channels.Channel {
	if head != nil {
		return *head
	}
	return channels.Channel{CreatedAt: now}
}

// sessionRefusal is the `in:` decision for a post of mt under session s
// on ch: a [*channels.SessionStateError] when the session is in a state
// mt's `in:` does not list, or is stranded; nil when it admits the post,
// there is no session, or mt has no `in:`. The pre-check in [Service.Post]
// calls it on the channel the call read (see [sessionChannel]), and the
// store calls it through [admitPost], so the two cannot drift. It reads only
// its arguments, so it is safe under the store's lock.
func sessionRefusal(
	s *realtimecfg.Session, mt *realtimecfg.MessageType, ch channels.Channel,
) *channels.SessionStateError {
	if s == nil || mt.In == nil {
		return nil
	}
	cur := currentSession(s, ch)
	if cur.stranded || !mt.PostableIn(cur.State) {
		return &channels.SessionStateError{State: cur.State, Stranded: cur.stranded}
	}
	return nil
}

// admitPost is the [channels.SessionStep.Admit] of a person's post of mt:
// the `in:` refusal of [sessionRefusal], then, when expect is set, a
// [*channels.SessionGenerationError] when the session is not at generation
// *expect. Nil when there is no session or neither check applies, so the
// store skips the call.
func admitPost(s *realtimecfg.Session, mt *realtimecfg.MessageType, expect *int64) func(channels.Channel) error {
	if s == nil || (mt.In == nil && expect == nil) {
		return nil
	}
	return func(ch channels.Channel) error {
		// Not `return sessionRefusal(...)`: a nil *SessionStateError in an
		// error is not a nil error.
		if e := sessionRefusal(s, mt, ch); e != nil {
			return e
		}
		e := generationRefusal(s, ch, expect)
		if e == nil {
			return nil
		}
		return e
	}
}

// generationRefusal is the generation precondition of a post that expects
// the session on ch at generation *expect: nil when expect is nil or the
// session is at it. The pre-check in [Service.Post] and the store (through
// [admitPost]) both call it.
func generationRefusal(s *realtimecfg.Session, ch channels.Channel, expect *int64) *channels.SessionGenerationError {
	if s == nil || expect == nil {
		return nil
	}
	cur := currentSession(s, ch)
	if cur.Generation == *expect {
		return nil
	}
	return &channels.SessionGenerationError{Want: *expect, Got: cur.Generation, State: cur.State}
}

// sessionMoved is the 409 for a post that expected the session at another
// generation than e saw.
func sessionMoved(e *channels.SessionGenerationError) *DenialError {
	return deny(http.StatusConflict, TypeSessionMoved,
		fmt.Sprintf("The session moved on: it is at generation %d (%s); this post expected generation %d.",
			e.Got, e.State, e.Want))
}

// sessionWrongState is the 409 for a post of mt that e refused: the
// session is in a state mt's `in:` does not list. The title is built from
// the state the refusal saw, so the pre-check and the store's refusal read
// the same.
func sessionWrongState(
	ct realtimecfg.ChannelType, mt *realtimecfg.MessageType, e *channels.SessionStateError,
) *DenialError {
	where := "it is in " + e.State
	if e.Stranded {
		where = "the session is stranded in " + e.State
	}
	return &DenialError{
		Status: http.StatusConflict, Type: TypeSessionWrongState,
		Title: fmt.Sprintf("%s may be posted only while the session is in %s; %s",
			mt.Name, strings.Join(mt.In, ", "), where),
		Rule: ct.InKey(mt),
	}
}

// sessionPost is the session part of one log post: the step the store
// runs, and what its decider saw that the caller logs after the append.
type sessionPost struct {
	step   *channels.SessionStep
	ct     string
	mt     string
	failed *guardFailure
	// gate is the decider's outputs backstop; see [outputGate].
	gate *outputGate
}

// guardFailure is the guard that failed to evaluate when the decider last
// ran; the decider stopped there and moved nothing. err is nil when none
// failed.
type guardFailure struct {
	t     realtimecfg.Transition
	state string
	err   error
}

// newSessionPost returns the session part of a post of mt with the
// validated props to a channel of ct. Its step is nil when ct has no
// session. A message type that fires no transition still gets a step that
// never moves, so the store reports the session it committed. triggeredBy
// names the trigger for a trigger's post and is empty for a person's.
// expect is the generation a person's post was decided against; nil for no
// precondition, and always nil for a trigger or a reminder.
func newSessionPost(
	ctx context.Context, ct realtimecfg.ChannelType, mt *realtimecfg.MessageType, props map[string]any,
	now time.Time, p poster, triggeredBy string, expect *int64,
) sessionPost {
	s := ct.Session()
	sp := sessionPost{ct: ct.Name, mt: mt.Name, failed: &guardFailure{}, gate: &outputGate{}}
	if s == nil {
		return sp
	}
	sp.step = &channels.SessionStep{
		Admit:      admitPost(s, mt, expect),
		Decide:     sessionDecider(ctx, s, mt, props, now, p, sp.failed, sp.gate),
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
		TriggeredBy: channels.SessionMarker + ct.Name, AuditTriggeredBy: triggeredBy,
		Outcome: &channels.SessionOutcome{},
	}
	withSessionDue(sp.step, s)
	return sp
}

// logGuardFailure logs, at warn and once per entry per loaded
// configuration, a guard that failed to evaluate in the append. It runs
// after the append, outside the store's lock. A guard that fails closed
// otherwise looks like one that does not hold; the decider stopped there
// and moved nothing.
func (sp sessionPost) logGuardFailure(ctx context.Context) {
	f := sp.failed
	if f == nil || f.err == nil || !f.t.WarnGuardOnce() {
		return
	}
	slog.WarnContext(ctx, "realtime: a session transition guard failed to evaluate; the session did not move",
		"channel_type", sp.ct, "state", f.state, "message_type", sp.mt, "to", f.t.To, "entry", f.t.Key(),
		"error", f.err)
}

// sessionDecider returns the decider for a post of mt with props. It tries
// the entries of `on.<mt>` out of the current state in file order and
// takes the first whose poster rule passes ([poster.admits]) and whose
// `when:` holds. It moves nothing when the channel is not open, no entry
// applies, or the chosen entry has outputs the pre-check did not check
// (see [outputGate]); after a gate refusal it does not try later entries,
// so the pre-check and the store follow one rule. A final state has no
// transitions and a stranded state is not declared, so for both
// [realtimecfg.Session.Branches] finds none. It reads only its arguments
// and the channel, so it is safe to run under the store's lock.
//
// A guard that fails to evaluate stops the decision: nothing moves, and
// later entries are not tried. The failure is recorded in failed for the
// caller to log. Falling through would let a message that makes a guard
// fail, rather than not hold, reach a later entry such as a catch-all
// with outputs (fail closed, as for a single entry).
func sessionDecider(
	ctx context.Context, s *realtimecfg.Session, mt *realtimecfg.MessageType, props map[string]any,
	now time.Time, p poster, failed *guardFailure, gate *outputGate,
) func(channels.Channel) (channels.SessionMove, bool) {
	fields := mt.PredicateFields(props)
	return func(ch channels.Channel) (channels.SessionMove, bool) {
		*failed = guardFailure{}
		if ch.State != channels.StateOpen {
			return channels.SessionMove{}, false
		}
		cur := currentSession(s, ch)
		branches := s.Branches(cur.State, mt.Name)
		if len(branches) == 0 {
			return channels.SessionMove{}, false
		}
		facts := realtimecfg.GuardFacts{
			State: cur.State, Generation: cur.Generation, EnteredAt: cur.EnteredAt,
			Reminders: reminderCounters(s, cur, ch),
		}
		for _, t := range branches {
			if !p.admits(t) {
				continue
			}
			holds, err := realtimecfg.GuardHolds(ctx, t.When, fields, facts, now)
			if err != nil {
				*failed = guardFailure{t: t, state: cur.State, err: err}
				return channels.SessionMove{}, false
			}
			if !holds {
				continue
			}
			if !gate.admits(t, cur.State) {
				return channels.SessionMove{}, false
			}
			return channels.SessionMove{From: cur.State, To: t.To}, true
		}
		return channels.SessionMove{}, false
	}
}

// postedSession is the session after a post to c's channel, as the store
// committed it under its lock. Nil when the channel type has no session.
func postedSession(c *call, step *channels.SessionStep) *SessionView {
	if step == nil || step.Outcome == nil {
		return nil
	}
	ch := channels.Channel{State: channels.StateOpen, CreatedAt: step.Outcome.CreatedAt, Session: step.Outcome.Session}
	if c.head != nil {
		ch.CreatedBy = c.head.CreatedBy
	}
	return readerSession(c, &ch)
}

// isNotice reports whether a message of msgType triggered by triggeredBy
// is a session notice. Only the store can write the
// [channels.SessionMarker], so a poster cannot fool the check.
func isNotice(msgType, triggeredBy string) bool {
	return msgType == realtimecfg.SessionNoticeType && strings.HasPrefix(triggeredBy, channels.SessionMarker)
}

// isReminder reports whether m was posted by a session reminder. Only the
// store can write the [channels.ReminderMarker], so a person's post of the
// same type is not a reminder.
func isReminder(m channels.Message) bool {
	return strings.HasPrefix(m.TriggeredBy, channels.ReminderMarker)
}

// reminderMessage is the 409 for an edit or author delete of a reminder.
func reminderMessage() *DenialError {
	return deny(http.StatusConflict, TypeReminderMessage,
		"a reminder is posted by the session and cannot be edited or deleted; a moderator may still remove it")
}

// sessionNotice is the 409 for an edit or removal of a session notice.
func sessionNotice() *DenialError {
	return deny(http.StatusConflict, TypeSessionNotice, "a session notice records a transition and cannot be changed")
}

// sessionMessage is the 409 for an author's edit or delete of a message
// that moved the session: the guard judged its properties as posted, and
// the notice and audit record name it as the cause.
func sessionMessage() *DenialError {
	return deny(http.StatusConflict, TypeSessionMessage,
		"this message moved the session, so it cannot be edited or deleted; a moderator may still remove it")
}
