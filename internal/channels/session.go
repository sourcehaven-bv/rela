package channels

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"
)

// Session is a channel's session snapshot: the state it is in, how often it
// moved, and when it entered the state. The zero value is unset: the
// session never moved, and the caller reads it as its configured initial
// state. Only an append with a [SessionStep] writes it.
//
// NextDueAt, Restarts and Reminders are the session's timer schedule (see
// [ScheduleFunc]). The store keeps them, but never derives them.
type Session struct {
	State      string    `json:"state,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	EnteredAt  time.Time `json:"entered_at,omitzero"`
	// NextDueAt is the index hint for [Store.ListSessionsDue]: the earliest
	// due time of the current state's timers. Zero: no due time.
	NextDueAt time.Time `json:"next_due_at,omitzero"`
	// Restarts holds, per timer name, the restart anchor of a timer that
	// came due while the channel was closed or tombstoned. A move clears
	// it.
	Restarts map[string]time.Time `json:"restarts,omitempty"`
	// Reminders holds, per reminder name, the reminder's record in the
	// current span. A move clears it; the step's Due decides which records
	// carry over.
	Reminders map[string]ReminderRecord `json:"reminders,omitempty"`
}

// ReminderRecord is one reminder's state in a session span: the time its
// slots count from, the time of its last post and how many it posted. The
// slot grid itself is configuration, so only these three are stored.
type ReminderRecord struct {
	// Since is the start of the span, or the resume time after a restart.
	Since time.Time `json:"since"`
	// Last is the time of the last post in this span; zero for none.
	Last time.Time `json:"last,omitzero"`
	// Count is the number of reminders posted in this span.
	Count int64 `json:"count"`
}

// NormReminder returns r with its times normalized ([NormTime]), as every
// backend stores them, so a compare-and-set on a record read back compares
// equal values on every backend.
func NormReminder(r ReminderRecord) ReminderRecord {
	return ReminderRecord{Since: NormTime(r.Since), Last: NormTime(r.Last), Count: r.Count}
}

// CloneReminders returns a copy of rs, or nil for none.
func CloneReminders(rs map[string]ReminderRecord) map[string]ReminderRecord {
	if len(rs) == 0 {
		return nil
	}
	out := make(map[string]ReminderRecord, len(rs))
	maps.Copy(out, rs)
	return out
}

// MaxSessionReminders caps the reminder records one session stores: one per
// reminder, of which realtime.yaml allows at most this many per session.
const MaxSessionReminders = 8

// CheckReminder refuses a record with an invalid reminder name or a
// negative count, with [ErrInvalidMessage]. The stores run it on write and,
// through [CheckReminders], on read, where a failure marks a damaged row.
func CheckReminder(name string, r ReminderRecord) error {
	if !ValidStateName(name) {
		return fmt.Errorf("%w: reminder record %q", ErrInvalidMessage, name)
	}
	if r.Count < 0 {
		return fmt.Errorf("%w: reminder %q has a negative count", ErrInvalidMessage, name)
	}
	return nil
}

// CheckReminders refuses a record map over [MaxSessionReminders], or with
// a record [CheckReminder] refuses, with [ErrInvalidMessage].
func CheckReminders(rs map[string]ReminderRecord) error {
	if len(rs) > MaxSessionReminders {
		return fmt.Errorf("%w: %d reminder records, at most %d", ErrInvalidMessage, len(rs), MaxSessionReminders)
	}
	for name, r := range rs {
		if err := CheckReminder(name, r); err != nil {
			return err
		}
	}
	return nil
}

// IsZero reports an unset session.
func (s Session) IsZero() bool { return s.Generation == 0 && s.State == "" }

// SessionSchedule is what a [ScheduleFunc] returns: the next due time and,
// when non-nil, the restart anchors and reminder records to store. A nil
// Restarts keeps the stored anchors; a nil Reminders keeps the stored
// records, and an empty one stores none.
type SessionSchedule struct {
	NextDueAt time.Time
	Restarts  map[string]time.Time
	Reminders map[string]ReminderRecord
}

// ScheduleFunc computes a channel's session schedule. The store calls it
// under its lock or inside its transaction, with the channel as it will be
// committed. Like [SessionStep.Decide] it must be pure and fast: no I/O
// and no locks. A backend may call it more than once if it retries.
type ScheduleFunc func(next Channel) SessionSchedule

// MaxSessionRestarts caps the restart anchors one session stores: one per
// timer of the state, of which realtime.yaml allows at most this many.
const MaxSessionRestarts = 8

// ApplySchedule stores what f returns for c, the channel as it will be
// committed. With f nil it does nothing. Restart anchors over
// [MaxSessionRestarts] or with an invalid timer name, and reminder records
// over [MaxSessionReminders], with an invalid name or with a negative
// count, are [ErrInvalidMessage], and c is left as it was.
func ApplySchedule(c *Channel, f ScheduleFunc) error {
	if f == nil {
		return nil
	}
	sch := f(*c)
	var reminders map[string]ReminderRecord
	if sch.Reminders != nil {
		if err := CheckReminders(sch.Reminders); err != nil {
			return err
		}
		reminders = make(map[string]ReminderRecord, len(sch.Reminders))
		for name, r := range sch.Reminders {
			reminders[name] = NormReminder(r)
		}
	}
	if sch.Restarts != nil {
		if len(sch.Restarts) > MaxSessionRestarts {
			return fmt.Errorf("%w: %d restart anchors, at most %d", ErrInvalidMessage,
				len(sch.Restarts), MaxSessionRestarts)
		}
		rs := make(map[string]time.Time, len(sch.Restarts))
		for name, at := range sch.Restarts {
			if !ValidStateName(name) {
				return fmt.Errorf("%w: restart anchor of timer %q", ErrInvalidMessage, name)
			}
			rs[name] = NormTime(at)
		}
		if len(rs) == 0 {
			rs = nil
		}
		c.Session.Restarts = rs
	}
	if sch.Reminders != nil {
		c.Session.Reminders = CloneReminders(reminders)
	}
	c.Session.NextDueAt = NormTime(sch.NextDueAt)
	return nil
}

// ReminderMarker starts the TriggeredBy of every reminder message. Only the
// store's reminder step writes it: [PrepareMessage] refuses it on any other
// append, so a poster cannot forge a reminder.
const ReminderMarker = "reminder:"

// ReminderPost is what a [SessionStep.Remind] returns: the message to
// append and the reminder's record after the post.
type ReminderPost struct {
	// Name is the reminder's name.
	Name string
	// Record is stored under Name before the step's Due runs, so Due sees
	// the committed channel with the post applied.
	Record ReminderRecord
	// Type is the message type of the reminder message.
	Type string
	// Props are the reminder message's properties.
	Props map[string]any
	// TriggeredBy must start with [ReminderMarker].
	TriggeredBy string
}

// PrepareReminderPost refuses a post whose TriggeredBy does not start with
// [ReminderMarker] or is too long, whose type is not a valid name, or
// whose record is invalid for at (see [ApplyReminder]).
func PrepareReminderPost(post ReminderPost, at time.Time) error {
	if !strings.HasPrefix(post.TriggeredBy, ReminderMarker) {
		return fmt.Errorf("%w: reminder triggered_by must start with %q", ErrInvalidMessage, ReminderMarker)
	}
	if len(post.TriggeredBy) > MaxTextBytes {
		return fmt.Errorf("%w: triggered_by over %d bytes", ErrTooLarge, MaxTextBytes)
	}
	if !ValidName(post.Type) {
		return fmt.Errorf("%w: reminder message type %q", ErrInvalidMessage, post.Type)
	}
	if err := CheckReminder(post.Name, post.Record); err != nil {
		return err
	}
	if NormTime(post.Record.Last).After(NormTime(at)) {
		return fmt.Errorf("%w: reminder %q last posted after now", ErrInvalidMessage, post.Name)
	}
	return nil
}

// ApplyReminder stores post's normalized record under its name in c, which
// must have passed [PrepareReminderPost]. The record map is copied, so a
// channel c was copied from keeps its own.
func ApplyReminder(c *Channel, post ReminderPost) {
	rs := CloneReminders(c.Session.Reminders)
	if rs == nil {
		rs = map[string]ReminderRecord{}
	}
	rs[post.Name] = NormReminder(post.Record)
	c.Session.Reminders = rs
}

// DecideReminder calls step's Remind against cur, when it is set, and
// checks the post for at. ok is false for a step without Remind or a
// Remind that posts nothing.
func DecideReminder(step *SessionStep, cur Channel, at time.Time) (post ReminderPost, ok bool, err error) {
	if step.Remind == nil {
		return ReminderPost{}, false, nil
	}
	post, ok = step.Remind(cur)
	if !ok {
		return ReminderPost{}, false, nil
	}
	if err := PrepareReminderPost(post, at); err != nil {
		return ReminderPost{}, false, err
	}
	return post, true, nil
}

// ReminderMessage returns the message of post, which the store appends at
// seq with rev at at: the post's type, props and TriggeredBy, with the
// step's NoticeBy as author. Props over the caps are [ErrTooLarge]. raw is
// the props' JSON encoding.
func ReminderMessage(
	step *SessionStep, post ReminderPost, seq int64, at time.Time, rev int64,
) (m Message, raw []byte, err error) {
	props, raw, err := PrepareProps(post.Props)
	if err != nil {
		return Message{}, nil, err
	}
	return Message{
		Seq: seq, Rev: rev, Type: post.Type, Author: step.NoticeBy, At: NormTime(at),
		Props: props, TriggeredBy: post.TriggeredBy,
	}, raw, nil
}

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
	// Due computes the session's schedule after the step, moved or not.
	// It is required: a move without it would leave no due time, and the
	// next timer would never fire.
	Due ScheduleFunc
	// Timer names the timer behind a [Store.SetSession] move. [Store.Append]
	// refuses a step that sets it, so a post cannot pose as a timer fire.
	Timer string
	// Remind is called by [Store.SetSession] under the lock when Decide
	// returns no move. When it returns a post, the store applies the post's
	// record (see [ApplyReminder]), appends the reminder message at
	// last_seq+1 (see [ReminderMessage]) and then applies Due. Pure and
	// fast, like Decide. [Store.Append] refuses a step that sets it.
	Remind func(cur Channel) (ReminderPost, bool)
	// Mark, when set, is called by [Store.Append] or [Store.SetSession] for
	// a move the decider returned. When it returns an action, the notice
	// carries it as its pending [OutputsAction] mark: the store sets the
	// name, the pending status, the time, and the mark's Gen, From, To and
	// Cause (on a timer move: Timer and Job instead of Cause), and keeps
	// the returned PendingAt as the time the mark becomes eligible for a
	// retry. Pure and fast, like Decide. Nil: no mark.
	Mark func(move SessionMove) *Action
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
	// Reminded is true when a [Store.SetSession] step appended a reminder
	// message.
	Reminded bool
	// Notice is the seq of the notice a move appended; zero without a
	// move.
	Notice int64
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
	if step.Due == nil {
		return fmt.Errorf("%w: session step has no schedule", ErrInvalidMessage)
	}
	if step.Timer != "" && !ValidStateName(step.Timer) {
		return fmt.Errorf("%w: timer %q", ErrInvalidMessage, step.Timer)
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
// one and the session enters the state at at. It clears the due time and
// the restart anchors; the step's [SessionStep.Due] sets the new ones and
// decides which anchors carry over.
func ApplySession(c *Channel, move SessionMove, at time.Time) {
	c.Session = Session{State: move.To, Generation: c.Session.Generation + 1, EnteredAt: NormTime(at)}
}

// PrepareAppendStep is [PrepareSessionStep] for [Store.Append], which also
// refuses a step that names a timer or sets Remind. Nil: accepted, as for
// [PrepareSessionStep].
func PrepareAppendStep(step *SessionStep) error {
	if step != nil && step.Timer != "" {
		return fmt.Errorf("%w: an append cannot name a timer", ErrInvalidMessage)
	}
	if step != nil && step.Remind != nil {
		return fmt.Errorf("%w: an append cannot post a reminder", ErrInvalidMessage)
	}
	return PrepareSessionStep(step)
}

// PrepareTimerStep validates the step of a [Store.SetSession] call.
func PrepareTimerStep(step SessionStep) error {
	return PrepareSessionStep(&step)
}

// DecideTimer runs the step's decider against cur for [Store.SetSession]
// and checks the move: a move needs a timer name and valid state names.
func DecideTimer(step *SessionStep, cur Channel) (move SessionMove, moved bool, err error) {
	move, moved, err = DecideSession(step, cur)
	if err == nil && moved && step.Timer == "" {
		return SessionMove{}, false, fmt.Errorf("%w: a timer move must name its timer", ErrInvalidMessage)
	}
	return move, moved, err
}

// TimerNotice returns the notice of a move made by step's timer, which the
// store appends at seq with rev at at. Its props are from, to, generation
// (the generation after the move) and timer. The notice carries the step's
// outputs mark, when its Mark gives one (see [SessionStep.Mark]), as a job
// mark naming the timer; an invalid mark is an error and nothing is stored.
func TimerNotice(
	step *SessionStep, seq int64, at time.Time, move SessionMove, gen, rev int64,
) (m Message, raw []byte, err error) {
	props, raw, err := PrepareProps(map[string]any{
		"from": move.From, "to": move.To, "generation": gen, "timer": step.Timer,
	})
	if err != nil { // coverage-ignore: invariant: four short values always encode under the caps
		return Message{}, nil, err
	}
	at = NormTime(at)
	mark, err := timerMark(step, at, move, gen)
	if err != nil {
		return Message{}, nil, err
	}
	return Message{
		Seq: seq, Rev: rev, Type: step.NoticeType, Author: step.NoticeBy, At: at,
		Props: props, TriggeredBy: step.TriggeredBy, Action: mark,
	}, raw, nil
}

// TimerAudit returns the audit record of a move made by step's timer: By
// is the notice's author, Seq the notice's seq, Generation the generation
// after the move.
func TimerAudit(k Key, step *SessionStep, notice Message, move SessionMove, gen int64) AuditRecord {
	r := KeyAudit(k, AuditSession, notice.At)
	r.Seq, r.By, r.From, r.To, r.Generation = notice.Seq, notice.Author, move.From, move.To, gen
	r.TriggeredBy = step.AuditTriggeredBy
	return r
}

// SessionNotice returns the notice of a move caused by the stored message
// cause, which the store appends at cause.Seq + 1 with rev, the rev after
// every other change of the append (a reply also raises its parent). Its
// props are from, to, generation (the generation after the move) and
// cause (the causing seq), in the shapes a read returns. raw is their JSON
// encoding. The notice carries the step's outputs mark, when its Mark
// gives one (see [SessionStep.Mark]); an invalid mark is an error and
// nothing is stored.
func SessionNotice(
	step *SessionStep, cause Message, move SessionMove, gen, rev int64,
) (m Message, raw []byte, err error) {
	props, raw, err := PrepareProps(map[string]any{
		"from": move.From, "to": move.To, "generation": gen, "cause": cause.Seq,
	})
	if err != nil { // coverage-ignore: invariant: four short values always encode under the caps
		return Message{}, nil, err
	}
	mark, err := sessionMark(step, cause, move, gen)
	if err != nil {
		return Message{}, nil, err
	}
	return Message{
		Seq: cause.Seq + 1, Rev: rev, Type: step.NoticeType, Author: step.NoticeBy, At: cause.At,
		Props: props, TriggeredBy: step.TriggeredBy, Action: mark,
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
	if moved {
		step.Outcome.Notice = c.LastSeq
	}
}

// FillReminderOutcome is [FillSessionOutcome] for a [Store.SetSession] step
// that appended a reminder message.
func FillReminderOutcome(step *SessionStep, c Channel) {
	FillSessionOutcome(step, c, SessionMove{}, false)
	if step.Outcome != nil {
		step.Outcome.Reminded = true
	}
}

// CheckDueQuery validates the arguments of [Store.ListSessionsDue]: at
// least one type, every type valid ([ErrInvalidKey] otherwise), and a
// positive limit ([ErrInvalidMessage] otherwise).
func CheckDueQuery(types []string, limit int) error {
	if len(types) == 0 {
		return fmt.Errorf("%w: no channel type to list due sessions of", ErrInvalidKey)
	}
	for _, t := range types {
		if !ValidName(t) {
			return fmt.Errorf("%w: channel type %q", ErrInvalidKey, t)
		}
	}
	if limit <= 0 {
		return fmt.Errorf("%w: limit %d", ErrInvalidMessage, limit)
	}
	return nil
}

// SortDue orders cs as [Store.ListSessionsDue] returns them: by
// Session.NextDueAt and then by key.
func SortDue(cs []Channel) {
	sort.SliceStable(cs, func(i, j int) bool {
		if c := cs[i].Session.NextDueAt.Compare(cs[j].Session.NextDueAt); c != 0 {
			return c < 0
		}
		return KeyLess(cs[i].Key, cs[j].Key)
	})
}
