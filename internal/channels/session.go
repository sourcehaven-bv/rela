package channels

import (
	"fmt"
	"strings"
	"time"
)

// Session is a channel's session snapshot: the state it is in, how often it
// moved, and when it entered the state. The zero value is unset: the
// session never moved, and the caller reads it as its configured initial
// state. Only an append with a [SessionStep] writes it.
type Session struct {
	State      string    `json:"state,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	EnteredAt  time.Time `json:"entered_at,omitzero"`
}

// IsZero reports an unset session.
func (s Session) IsZero() bool { return s.Generation == 0 && s.State == "" }

// SessionMove is one session transition: the state the session leaves and
// the state it enters. From equals To for a re-entry.
type SessionMove struct{ From, To string }

// SessionMarker starts the TriggeredBy of every session notice. Only the
// store writes it: [PrepareMessage] refuses it on any other append, so a
// poster cannot forge a notice.
const SessionMarker = "session:"

// SessionStep lets an append move the channel's session in the same step.
//
// The store calls Decide with the channel as committed, under its lock or
// inside its transaction: for an implicit create, an open channel with an
// unset session. When Decide returns a move, the store appends the message
// at seq n and a notice at n+1 (see [SessionNotice]), sets the session (see
// [ApplySession]), and writes an [AuditSession] record (see [SessionAudit]).
// All of it is one step: a backend with transactions rolls back every part
// together.
//
// The store applies no policy of its own. It calls Decide, checks the state
// names with [CheckSessionMove], and writes.
type SessionStep struct {
	// Decide must be pure and fast: no I/O and no locks. A backend may call
	// it more than once if it retries a transaction.
	Decide func(cur Channel) (SessionMove, bool)
	// NoticeType is the message type of the notice.
	NoticeType string
	// NoticeBy is the author of the notice.
	NoticeBy Author
	// TriggeredBy is the notice's TriggeredBy. It must start with
	// [SessionMarker].
	TriggeredBy string
	// AuditTriggeredBy is the audit record's TriggeredBy: set when the
	// causing message came from a rule rather than a person.
	AuditTriggeredBy string
	// Outcome, when set, is filled by the store after a successful append.
	Outcome *SessionOutcome
}

// SessionOutcome is what an append with a [SessionStep] did to the session:
// the session after the append, and the move when there was one. CreatedAt
// is the channel's creation time, when an unset session entered its
// initial state.
type SessionOutcome struct {
	Session   Session
	CreatedAt time.Time
	Moved     bool
	Move      SessionMove
}

// ValidStateName reports whether s is a valid session state name: the
// name grammar of realtime.yaml. A state name is never a path segment, so
// unlike [ValidName] it allows Windows device names such as aux.
func ValidStateName(s string) bool { return namePattern.MatchString(s) }

// PrepareSessionStep validates a step before the append takes its lock.
// Nil: accepted and returned as nil, an append without a step.
func PrepareSessionStep(step *SessionStep) error {
	if step == nil {
		return nil
	}
	if step.Decide == nil {
		return fmt.Errorf("%w: session step has no decider", ErrInvalidMessage)
	}
	if !ValidName(step.NoticeType) {
		return fmt.Errorf("%w: notice type %q", ErrInvalidMessage, step.NoticeType)
	}
	if err := checkAuthor(step.NoticeBy); err != nil {
		return err
	}
	if !strings.HasPrefix(step.TriggeredBy, SessionMarker) {
		return fmt.Errorf("%w: notice triggered_by must start with %q", ErrInvalidMessage, SessionMarker)
	}
	if len(step.TriggeredBy) > MaxTextBytes || len(step.AuditTriggeredBy) > MaxTextBytes {
		return fmt.Errorf("%w: triggered_by over %d bytes", ErrTooLarge, MaxTextBytes)
	}
	return nil
}

// CheckSessionMove refuses a move whose state names break the grammar,
// with [ErrInvalidMessage]. Nothing is stored then.
func CheckSessionMove(move SessionMove) error {
	if !ValidStateName(move.From) || !ValidStateName(move.To) {
		return fmt.Errorf("%w: session move %q to %q", ErrInvalidMessage, move.From, move.To)
	}
	return nil
}

// DecideSession runs the step's decider against cur and checks the move.
// moved is false for an append without a step, or a decider that returns
// no move.
func DecideSession(step *SessionStep, cur Channel) (move SessionMove, moved bool, err error) {
	if step == nil {
		return SessionMove{}, false, nil
	}
	move, moved = step.Decide(cur)
	if !moved {
		return SessionMove{}, false, nil
	}
	if err := CheckSessionMove(move); err != nil {
		return SessionMove{}, false, err
	}
	return move, true, nil
}

// ApplySession moves c's session to move.To at at: the generation rises by
// one and the session enters the state at at.
func ApplySession(c *Channel, move SessionMove, at time.Time) {
	c.Session = Session{State: move.To, Generation: c.Session.Generation + 1, EnteredAt: NormTime(at)}
}

// SessionNotice returns the notice of a move caused by the stored message
// cause, which the store appends at cause.Seq + 1 with rev, the rev after
// every other change of the append (a reply also raises its parent). Its
// props are from, to, generation (the generation after the move) and
// cause (the causing seq), in the shapes a read returns. raw is their JSON
// encoding.
func SessionNotice(
	step *SessionStep, cause Message, move SessionMove, gen, rev int64,
) (m Message, raw []byte, err error) {
	props, raw, err := PrepareProps(map[string]any{
		"from": move.From, "to": move.To, "generation": gen, "cause": cause.Seq,
	})
	if err != nil { // coverage-ignore: invariant: four short values always encode under the caps
		return Message{}, nil, err
	}
	return Message{
		Seq: cause.Seq + 1, Rev: rev, Type: step.NoticeType, Author: step.NoticeBy, At: cause.At,
		Props: props, TriggeredBy: step.TriggeredBy,
	}, raw, nil
}

// SessionAudit returns the audit record of a move caused by cause: By is
// the poster, Seq the causing seq, Generation the generation after the
// move.
func SessionAudit(k Key, step *SessionStep, cause Message, move SessionMove, gen int64) AuditRecord {
	r := KeyAudit(k, AuditSession, cause.At)
	r.Seq, r.By, r.From, r.To, r.Generation = cause.Seq, cause.Author, move.From, move.To, gen
	r.TriggeredBy = step.AuditTriggeredBy
	return r
}

// FillSessionOutcome records the session after an append in the step's
// Outcome, when the caller asked for it.
func FillSessionOutcome(step *SessionStep, c Channel, move SessionMove, moved bool) {
	if step == nil || step.Outcome == nil {
		return
	}
	*step.Outcome = SessionOutcome{Session: c.Session, CreatedAt: c.CreatedAt, Moved: moved, Move: move}
}
