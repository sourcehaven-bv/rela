package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// SessionView is a channel's session as a reader sees it. Transitions are
// the `on:` entries of the current state, sorted by message type: config,
// the same for every reader. They are empty when the state is final or
// stranded.
type SessionView struct {
	State string `json:"state"`
	Label string `json:"label,omitempty"`
	// Generation counts the transitions; 0 means the session never moved.
	Generation int64 `json:"generation"`
	// EnteredAt is when the session entered its state: the time of the
	// causing message, or for a session that never moved, the channel's
	// created_at.
	EnteredAt time.Time `json:"entered_at"`
	Final     bool      `json:"final,omitempty"`
	// Stranded is set when the stored state is one the configuration no
	// longer declares. Label is then the stored name, and nothing moves the
	// session until the state is declared again.
	Stranded    bool                    `json:"stranded,omitempty"`
	Transitions []SessionTransitionView `json:"transitions,omitempty"`
}

// SessionTransitionView is one transition out of the current state: the
// message type that fires it, the state it moves to and, when only a
// trigger may fire it, By "trigger".
type SessionTransitionView struct {
	MessageType string `json:"message_type"`
	To          string `json:"to"`
	By          string `json:"by,omitempty"`
}

// current is a channel's session read against the configuration.
type current struct {
	channels.Session
	stranded bool
}

// currentSession reads ch's session under s. An unset session reads as the
// configured initial state, entered when the channel was created. For a
// channel that is not stored yet, the caller passes a channel whose
// CreatedAt is the operation's clock time, the time the append will store.
func currentSession(s *realtimecfg.Session, ch channels.Channel) current {
	if ch.Session.IsZero() {
		return current{Session: channels.Session{State: s.Initial(), EnteredAt: ch.CreatedAt}}
	}
	_, declared := s.State(ch.Session.State)
	return current{Session: ch.Session, stranded: !declared}
}

// sessionView renders the session of h under ct, or of the empty channel
// created at now when h is nil. Nil when ct has no session.
func sessionView(ct realtimecfg.ChannelType, h *channels.Channel, now time.Time) *SessionView {
	s := ct.Session()
	if s == nil {
		return nil
	}
	ch := channels.Channel{CreatedAt: now}
	if h != nil {
		ch = *h
	}
	cur := currentSession(s, ch)
	v := &SessionView{
		State: cur.State, Label: cur.State, Generation: cur.Generation, EnteredAt: cur.EnteredAt,
		Stranded: cur.stranded,
	}
	st, ok := s.State(cur.State)
	if !ok {
		return v
	}
	v.Label, v.Final = st.Label, st.Final
	for _, tr := range st.Transitions() {
		v.Transitions = append(v.Transitions, SessionTransitionView{
			MessageType: tr.MessageType, To: tr.To, By: string(tr.By),
		})
	}
	return v
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

// admits reports whether p passes t's poster rule. With roles, a person
// or agent holding one passes and a trigger never does. With by: trigger,
// only a trigger passes. With neither, every poster passes: they were
// allowed to post the message type.
func (p poster) admits(t realtimecfg.Transition) bool {
	switch {
	case len(t.Roles) > 0:
		if p.trigger {
			return false
		}
		for _, r := range t.Roles {
			if p.roles[r] {
				return true
			}
		}
		return false
	case t.By == realtimecfg.TransitionByTrigger:
		return p.trigger
	default:
		return true
	}
}

// transitionRoles asks, once per role, which roles named by a transition
// that mt fires the caller holds for update, as [call.mayPost] asks, so the
// client ceiling applies. A message type that fires no transition with
// roles asks nothing.
func (c *call) transitionRoles(mt *realtimecfg.MessageType) map[string]bool {
	out := map[string]bool{}
	s := c.ct.Session()
	if s == nil {
		return out
	}
	for _, st := range s.States() {
		t, ok := s.Transition(st.Name, mt.Name)
		if !ok {
			continue
		}
		for _, name := range t.Roles {
			if _, asked := out[name]; asked {
				continue
			}
			r, found := c.ct.Roles[name]
			out[name] = found && c.can(r, realtimecfg.CapabilityUpdate)
		}
	}
	return out
}

// sessionPost is the session part of one log post: the step the store
// runs, and what its decider saw that the caller logs after the append.
type sessionPost struct {
	step   *channels.SessionStep
	ct     string
	mt     string
	failed *guardFailure
}

// guardFailure is a guard that failed to evaluate when the decider last
// ran. err is nil when it did not fail.
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
func newSessionPost(
	ctx context.Context, ct realtimecfg.ChannelType, mt *realtimecfg.MessageType, props map[string]any,
	now time.Time, p poster, triggeredBy string,
) sessionPost {
	s := ct.Session()
	sp := sessionPost{ct: ct.Name, mt: mt.Name, failed: &guardFailure{}}
	if s == nil {
		return sp
	}
	sp.step = &channels.SessionStep{
		Decide:     sessionDecider(ctx, s, mt, props, now, p, sp.failed),
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
		TriggeredBy: channels.SessionMarker + ct.Name, AuditTriggeredBy: triggeredBy,
		Outcome: &channels.SessionOutcome{},
	}
	return sp
}

// logGuardFailure logs, at warn and once per guard per loaded
// configuration, a guard that failed to evaluate in the append. It runs
// after the append, outside the store's lock. A guard that fails closed
// otherwise looks like one that does not hold.
func (sp sessionPost) logGuardFailure(ctx context.Context) {
	f := sp.failed
	if f == nil || f.err == nil || !f.t.WarnGuardOnce() {
		return
	}
	slog.WarnContext(ctx, "realtime: a session transition guard failed to evaluate; the session did not move",
		"channel_type", sp.ct, "state", f.state, "message_type", sp.mt, "error", f.err)
}

// sessionDecider returns the decider for a post of mt with props. It moves
// nothing when the channel is not open, the state has no transition for
// mt, the poster fails its rule, or its `when:` does not hold. A final
// state has no transitions and a stranded state is not declared, so for
// both Transition finds none. It reads only its arguments and the channel,
// so it is safe to run under the store's lock; a guard that fails to
// evaluate is recorded in failed for the caller to log.
func sessionDecider(
	ctx context.Context, s *realtimecfg.Session, mt *realtimecfg.MessageType, props map[string]any,
	now time.Time, p poster, failed *guardFailure,
) func(channels.Channel) (channels.SessionMove, bool) {
	fields := make(map[string]predicate.Value, len(mt.Properties))
	for name, prop := range mt.Properties {
		fields[name] = predicateValue(prop, props[name])
	}
	return func(ch channels.Channel) (channels.SessionMove, bool) {
		*failed = guardFailure{}
		if ch.State != channels.StateOpen {
			return channels.SessionMove{}, false
		}
		cur := currentSession(s, ch)
		t, ok := s.Transition(cur.State, mt.Name)
		if !ok || !p.admits(t) {
			return channels.SessionMove{}, false
		}
		holds, err := guardHolds(ctx, t.When, fields, cur, now)
		if err != nil {
			*failed = guardFailure{t: t, state: cur.State, err: err}
		}
		if !holds {
			return channels.SessionMove{}, false
		}
		return channels.SessionMove{From: cur.State, To: t.To}, true
	}
}

// guardHolds reports whether prog, a transition's `when:`, holds for the
// message fields and the session cur. No guard holds. An evaluation error,
// or a result that is not a boolean, does not hold: a guard fails closed.
// err reports such a failure. A nil operand, from an absent optional
// property, is no failure: the guard just does not hold.
func guardHolds(
	ctx context.Context, prog *predicate.Program, fields map[string]predicate.Value, cur current, now time.Time,
) (bool, error) {
	if prog == nil {
		return true, nil
	}
	b := predicate.NewBindings()
	err := b.SetVar("message", predicate.NewRecord(fields))
	if err == nil {
		err = b.SetVar("session", predicate.NewRecord(map[string]predicate.Value{
			"state":      predicate.NewString(cur.State),
			"generation": predicate.NewInt(cur.Generation),
			"entered_at": predicate.NewDate(cur.EnteredAt),
		}))
	}
	if err == nil {
		err = predicatefns.Bind(b, now)
	}
	if err != nil {
		return false, err
	}
	v, err := prog.Eval(ctx, b)
	if errors.Is(err, predicate.ErrNilOperand) {
		// An optional property the message leaves out: a valid config
		// whose guard does not hold, not a failure to report.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	got, ok := v.(predicate.Bool)
	if !ok {
		return false, fmt.Errorf("the guard returned %T, not a boolean", v)
	}
	return got.Bool(), nil
}

// postedSession is the session after a post to c's channel, as the store
// committed it under its lock. Nil when the channel type has no session.
func postedSession(c *call, step *channels.SessionStep) *SessionView {
	if step == nil || step.Outcome == nil {
		return nil
	}
	ch := channels.Channel{State: channels.StateOpen, CreatedAt: step.Outcome.CreatedAt, Session: step.Outcome.Session}
	return sessionView(c.ct, &ch, c.now)
}

// isNotice reports whether a message of msgType triggered by triggeredBy
// is a session notice. Only the store can write the
// [channels.SessionMarker], so a poster cannot fool the check.
func isNotice(msgType, triggeredBy string) bool {
	return msgType == realtimecfg.SessionNoticeType && strings.HasPrefix(triggeredBy, channels.SessionMarker)
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
