package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Session timers (`after:` on a session state, TKT-INM3AZ).
//
// The due time of a session is derived from the configuration and the
// channel row: the current state's timers, each counting from its anchor
// (see [timerDue]), and its reminders' next slots (see [nextSlot]). The
// row's Session.NextDueAt (see [nextDue]) is only an index hint for
// [channels.Store.ListSessionsDue]: every write that changes the session
// stores it, the leader's recompute repairs a stale one, and the wire and
// the timer round always derive the due time again.

// timerDue is when t comes due for ch, whose current session is cur. A
// timer counts from when the session entered its state (since: state; the
// channel's creation for a session that never moved) or from the channel's
// creation (since: session). A restart anchor later than that, stored when
// the timer came due while the channel was closed or tombstoned, wins.
func timerDue(t realtimecfg.SessionTimer, cur current, ch channels.Channel) time.Time {
	anchor := cur.EnteredAt
	if t.Since == realtimecfg.TimerSinceSession {
		anchor = ch.CreatedAt
	}
	if r, ok := ch.Session.Restarts[t.Name]; ok && r.After(anchor) {
		anchor = r
	}
	return channels.NormTime(t.Period.AddTo(anchor))
}

// nextTimer is the timer of ch's current state that comes due first, and
// when. A tie goes to the first name. ok is false for a final or stranded
// state, or a state without timers.
func nextTimer(s *realtimecfg.Session, ch channels.Channel) (t realtimecfg.SessionTimer, due time.Time, ok bool) {
	cur := currentSession(s, ch)
	st, declared := s.State(cur.State)
	if !declared || st.Final {
		return t, due, false
	}
	for _, tm := range st.Timers() {
		d := timerDue(tm, cur, ch)
		if !ok || d.Before(due) {
			t, due, ok = tm, d, true
		}
	}
	return t, due, ok
}

// sessionDue is the schedule of a create of a type with session s: the
// due time [nextDue] derives, or zero, and the stored restart anchors and
// reminder records kept.
func sessionDue(s *realtimecfg.Session) channels.ScheduleFunc {
	return func(next channels.Channel) channels.SessionSchedule {
		return channels.SessionSchedule{NextDueAt: nextDue(s, next)}
	}
}

// withSessionDue sets the schedule of step, a session step of a type with
// session s, once its decider is final. The schedule is [sessionDue], and
// on a move it carries the restart anchors over (see [stepSchedule]). To
// know the state the session left it wraps the decider, which every store
// calls before the schedule in the same attempt; the captured channel is
// per call, so the schedule stays free of I/O and locks.
func withSessionDue(step *channels.SessionStep, s *realtimecfg.Session) {
	decide := step.Decide
	var from channels.Channel
	step.Decide = func(cur channels.Channel) (channels.SessionMove, bool) {
		from = cur
		return decide(cur)
	}
	step.Due = func(next channels.Channel) channels.SessionSchedule { return stepSchedule(s, from, next) }
}

// stepSchedule is the schedule of a session step from channel from to
// next. Without a move it is [sessionDue]. A move clears the restart
// anchors and reminder records (see [channels.ApplySession]); the anchor of
// a since: session timer is put back when the target state has a since:
// session timer of the same name, because that timer keeps counting across
// the move. A since: state timer starts again from the move. The records of
// reminders active in both states are put back (see [carriedReminders]).
// Without a move, stored records are kept, pruned (see [pruneReminders]).
func stepSchedule(s *realtimecfg.Session, from, next channels.Channel) channels.SessionSchedule {
	if next.Session.Generation == from.Session.Generation {
		sch := channels.SessionSchedule{NextDueAt: nextDue(s, next)}
		if next.Session.Reminders != nil {
			sch.Reminders = pruneReminders(s, next.Session.Reminders)
		}
		return sch
	}
	carried := carriedRestarts(s, from, next.Session.State)
	reminders := carriedReminders(s, from, next.Session.State)
	next.Session.Restarts, next.Session.Reminders = carried, reminders
	return channels.SessionSchedule{NextDueAt: nextDue(s, next), Restarts: carried, Reminders: reminders}
}

// carriedRestarts are the restart anchors of from that a move to state to
// keeps: those of since: session timers declared, with since: session,
// in both the state the session left and to. Nil when there are none.
func carriedRestarts(s *realtimecfg.Session, from channels.Channel, to string) map[string]time.Time {
	left, ok := s.State(currentSession(s, from).State)
	target, ok2 := s.State(to)
	if !ok || !ok2 || len(from.Session.Restarts) == 0 {
		return nil
	}
	sessionScoped := make(map[string]bool)
	for _, t := range left.Timers() {
		sessionScoped[t.Name] = t.Since == realtimecfg.TimerSinceSession
	}
	var out map[string]time.Time
	for _, t := range target.Timers() {
		r, anchored := from.Session.Restarts[t.Name]
		if !anchored || t.Since != realtimecfg.TimerSinceSession || !sessionScoped[t.Name] {
			continue
		}
		if out == nil {
			out = make(map[string]time.Time)
		}
		out[t.Name] = r
	}
	return out
}

// ResumeSchedule is the schedule of a channel that resumes at at: a
// reopen, or the restore of an open channel from a tombstone. Each timer of
// the current state that came due at or before at restarts from at (its
// restart anchor becomes at); a timer not yet due keeps its deadline. The
// anchors are kept per timer, so repeated close and reopen cycles never
// stretch a deadline that had not come due. Reminders resume the same way
// (see [resumeReminders]).
//
// A channel of a type cfg does not declare, or without a session, keeps
// its schedule. Nil cfg: accepted, and every channel keeps its schedule.
func ResumeSchedule(cfg *realtimecfg.Compiled, at time.Time) channels.ScheduleFunc {
	return func(next channels.Channel) channels.SessionSchedule {
		keep := channels.SessionSchedule{NextDueAt: next.Session.NextDueAt}
		if cfg == nil {
			return keep
		}
		ct, ok := cfg.ChannelType(next.Key.Type)
		if !ok || ct.Session() == nil {
			return keep
		}
		return resumeSession(ct.Session(), next, at)
	}
}

// Resumption is the [ResumeSchedule] of one restore that also keeps the
// earliest due time it set, so the caller can note it on a [DueWake] once
// the restore committed. A store may call the schedule more than once, or
// for an attempt it then rolls back; a wake for such a due time only runs
// a round early.
type Resumption struct {
	f        channels.ScheduleFunc
	mu       sync.Mutex
	earliest time.Time
}

// NewResumption returns the resumption of a restore at at under cfg (see
// [ResumeSchedule]).
func NewResumption(cfg *realtimecfg.Compiled, at time.Time) *Resumption {
	return &Resumption{f: ResumeSchedule(cfg, at)}
}

// Schedule is the [channels.ScheduleFunc] to pass to Untombstone.
func (r *Resumption) Schedule(next channels.Channel) channels.SessionSchedule {
	sch := r.f(next)
	if due := sch.NextDueAt; !due.IsZero() {
		r.mu.Lock()
		if r.earliest.IsZero() || due.Before(r.earliest) {
			r.earliest = due
		}
		r.mu.Unlock()
	}
	return sch
}

// Note notes the earliest due time the restore set on w.
// Nil: w accepted, and nothing is noted.
func (r *Resumption) Note(w *DueWake) {
	r.mu.Lock()
	at := r.earliest
	r.mu.Unlock()
	if w != nil {
		w.Note(at)
	}
}

// resumeSession is [ResumeSchedule] for one channel of a type with
// session s.
func resumeSession(s *realtimecfg.Session, ch channels.Channel, at time.Time) channels.SessionSchedule {
	at = channels.NormTime(at)
	cur := currentSession(s, ch)
	st, declared := s.State(cur.State)
	if !declared || st.Final {
		return channels.SessionSchedule{}
	}
	timers := st.Timers()
	restarts := make(map[string]time.Time, len(timers))
	for _, t := range timers {
		if r, ok := ch.Session.Restarts[t.Name]; ok {
			restarts[t.Name] = r
		}
		if due := timerDue(t, cur, ch); !due.After(at) {
			restarts[t.Name] = at
		}
	}
	reminders := resumeReminders(s, cur, ch, at)
	resumed := ch
	resumed.Session.Restarts, resumed.Session.Reminders = restarts, reminders
	return channels.SessionSchedule{NextDueAt: nextDue(s, resumed), Restarts: restarts, Reminders: reminders}
}

// transitionSchedule is the schedule a lifecycle move of a channel of
// type ct to to stores: a reopen resumes the session at at; any other move
// keeps it.
func transitionSchedule(ct realtimecfg.ChannelType, to channels.State, at time.Time) channels.ScheduleFunc {
	s := ct.Session()
	if s == nil || to != channels.StateOpen {
		return nil
	}
	return func(next channels.Channel) channels.SessionSchedule { return resumeSession(s, next, at) }
}

// timerLabel is the audit TriggeredBy of a move made by timer name of
// state in channel type typ, in the style of [autoLabel].
func timerLabel(typ, state, name string) string {
	return "timer:" + typ + "/" + state + "/" + name
}

// timerStore is what the timer loop reads and writes.
type timerStore interface {
	ListSessionsDue(ctx context.Context, types []string, until time.Time, limit int) ([]channels.Channel, error)
	ListByLifecycle(ctx context.Context, typ string, state channels.State, before time.Time) ([]channels.Channel, error)
	SetSession(ctx context.Context, key channels.Key, at time.Time, step channels.SessionStep) (channels.Channel, error)
}

// TimerDeps are the collaborators of [Timers].
type TimerDeps struct {
	Store  timerStore
	Config ConfigSource
	Marker ChangeMarker
	Clock  Clock
}

// TimerReport is what one [Timers.RunRound] did.
type TimerReport struct {
	// Fired counts the sessions a timer moved; Reminded the reminder
	// messages posted; Rescheduled the due times rewritten without either.
	Fired, Reminded, Rescheduled int
	// Stranded counts the sessions seen in a state the configuration no
	// longer declares.
	Stranded int
	// Failed counts the writes that failed; their channels wait out
	// [realtimelimits.TimerBackoff].
	Failed int
	// Marks counts the timer moves that wrote an outputs mark, for the
	// outputs runner to run.
	Marks int
	// More is set when writes were left for the next round, or a write
	// left a due time that has already passed.
	More bool
	// Next is the earliest due time after now that the round saw, or zero.
	Next time.Time
}

// Timers fires session timers and posts session reminders: the leader runs
// [Timers.RunRound] at the due times it learns and at least every
// [realtimelimits.TimerCeiling].
//
// A fire is a compare-and-set on the session's (state, generation): the
// move happens once per (state, generation), and the attempt at least
// once. Two leaders overlapping, or a message that lands first, make the
// later attempt move nothing. A reminder post is a compare-and-set on the
// reminder's record (see [reminderStep]): a slot posts at most once.
//
// Timer fires, reminder posts and reschedules share one cap of
// [realtimelimits.MaxTimerWritesPerRound] writes per round, and the
// listing orders channels by their stored due time. When B due reminders
// sort ahead of a due timer, the timer fires at most ceil(B/64) rounds
// late, each [realtimelimits.TimerFloor] apart.
//
// A reminder's props come from the configuration alone, so the loop checks
// them once per reminder and configuration (see Timers.session). A
// reminder whose props fail is paused for every channel until the next
// configuration: the failure is logged once, and the channel's timers
// still fire and its due time skips the reminder. It is no channel's
// failure, so no channel goes into backoff for it.
//
// It holds the pending recompute, the channels in backoff and the
// stranded states already logged; [Timers.Reset] drops them all.
type Timers struct {
	store  timerStore
	cfg    ConfigSource
	marker ChangeMarker
	clock  Clock
	// jitter spreads the periodic recompute; tests replace it.
	jitter func(time.Duration) time.Duration
	// props checks a reminder's props ([reminderProps]); tests replace it
	// to break the check.
	props func(
		ctx context.Context, cfg *realtimecfg.Compiled, ct realtimecfg.ChannelType, r realtimecfg.SessionReminder,
		now time.Time,
	) (map[string]any, error)

	mu sync.Mutex
	// config is the configuration of the last round; a new pointer starts
	// a recompute.
	config *realtimecfg.Compiled
	// second is when the second recompute after a config change runs.
	second time.Time
	// recomputeAt is when the periodic recompute runs.
	recomputeAt time.Time
	// queue holds the writes a recompute decided and has not made yet.
	queue []channels.Key
	// backoff holds the channels whose write failed, until when; order
	// lists them oldest first, for the cap.
	backoff map[channels.Key]time.Time
	order   []channels.Key
	// warned holds the stranded (type, state) pairs logged under config.
	warned map[[2]string]bool
	// checked holds the outcome of props for each (type, reminder) under
	// config.
	checked map[[2]string]checkedProps
}

// checkedProps is the outcome of Timers.props for one reminder.
type checkedProps struct {
	props map[string]any
	err   error
}

// NewTimers returns the timer loop over d.
//
// Nil: every field of d is rejected.
func NewTimers(d TimerDeps) (*Timers, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Marker == nil:
		return nil, errNil("Marker")
	case d.Clock == nil:
		return nil, errNil("Clock")
	}
	t := &Timers{store: d.Store, cfg: d.Config, marker: d.Marker, clock: d.Clock, jitter: tenthJitter,
		props: reminderProps,
	}
	t.Reset()
	return t, nil
}

// tenthJitter returns d moved by up to a tenth either way.
func tenthJitter(d time.Duration) time.Duration {
	tenth := d / 10
	return d - tenth + rand.N(2*tenth) //nolint:gosec // jitter, not a secret
}

// Reset drops everything the loop remembers. The leader calls it when it
// starts and stops leading, so the first round after it recomputes every
// session.
func (t *Timers) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config, t.second, t.recomputeAt, t.queue = nil, time.Time{}, time.Time{}, nil
	t.backoff, t.order, t.warned = map[channels.Key]time.Time{}, nil, map[[2]string]bool{}
	t.checked = map[[2]string]checkedProps{}
}

// timerRound is the state of one [Timers.RunRound].
type timerRound struct {
	cfg    *realtimecfg.Compiled
	now    time.Time
	writes int
	rep    TimerReport
	errs   []error
	// stranded holds the stranded sessions counted in rep, so a session
	// the recompute and the due listing both see counts once.
	stranded map[channels.Key]bool
}

// strand counts k as a stranded session of the round, once.
func (r *timerRound) strand(k channels.Key) {
	if r.stranded[k] {
		return
	}
	if r.stranded == nil {
		r.stranded = map[channels.Key]bool{}
	}
	r.stranded[k] = true
	r.rep.Stranded++
}

// RunRound recomputes when one is due, then fires the sessions whose
// timer has come due and rewrites the due times that differ from the
// derived ones, at most [realtimelimits.MaxTimerWritesPerRound] writes in
// all. It does nothing when realtime is off or the configuration is
// degraded: the configuration it would follow is no longer the
// operator's. A failing channel costs only that channel; the round goes
// on and the errors are joined into the returned error.
func (t *Timers) RunRound(ctx context.Context) (TimerReport, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	snap := t.cfg.Load()
	if snap.Config == nil || snap.Degraded {
		return TimerReport{}, nil
	}
	r := &timerRound{cfg: snap.Config, now: t.clock()}
	if t.recomputeDue(r) {
		t.recompute(ctx, r)
	}
	t.drain(ctx, r)
	t.fireDue(ctx, r)
	return r.rep, errors.Join(r.errs...)
}

// recomputeDue reports whether this round starts a recompute: the first
// round, a new configuration and one ceiling after it, and every
// [realtimelimits.TimerRecompute].
func (t *Timers) recomputeDue(r *timerRound) bool {
	start := false
	if r.cfg != t.config {
		if t.config != nil {
			t.second = r.now.Add(realtimelimits.TimerCeiling)
		}
		t.config, t.queue, t.warned = r.cfg, nil, map[[2]string]bool{}
		t.checked = map[[2]string]checkedProps{}
		start = true
	}
	if !t.second.IsZero() && !r.now.Before(t.second) {
		t.second = time.Time{}
		start = true
	}
	if !r.now.Before(t.recomputeAt) {
		start = true
	}
	if start {
		t.recomputeAt = r.now.Add(t.jitter(realtimelimits.TimerRecompute))
	}
	return start
}

// recompute lists the open channels of every session type with timers or
// reminders once and queues those whose stored due time differs from the
// derived one. It logs, once per configuration, the states sessions are
// stranded in. A type without either is skipped, as fireDue skips it: a
// stale due time on it is never listed, and the wire derives its own.
func (t *Timers) recompute(ctx context.Context, r *timerRound) {
	t.queue = nil
	for _, ct := range r.cfg.ChannelTypes() {
		if s := ct.Session(); s == nil || !s.HasSchedule() {
			continue
		}
		s := t.session(ctx, r, ct)
		list, err := t.store.ListByLifecycle(ctx, ct.Name, channels.StateOpen, r.now.Add(time.Microsecond))
		if err != nil {
			r.errs = append(r.errs, err)
			continue
		}
		stranded := map[string]int{}
		for _, c := range list {
			if cur := currentSession(s, c); cur.stranded {
				stranded[cur.State]++
				r.strand(c.Key)
			}
			if !nextDue(s, c).Equal(channels.NormTime(c.Session.NextDueAt)) {
				t.queue = append(t.queue, c.Key)
			}
		}
		t.warnStranded(ctx, ct.Name, stranded)
	}
}

// warnStranded logs each stranded state of typ once per configuration.
func (t *Timers) warnStranded(ctx context.Context, typ string, stranded map[string]int) {
	states := make([]string, 0, len(stranded))
	for st := range stranded {
		states = append(states, st)
	}
	sort.Strings(states)
	for _, st := range states {
		k := [2]string{typ, st}
		if t.warned[k] {
			continue
		}
		t.warned[k] = true
		slog.WarnContext(ctx, "realtime: sessions are in a state realtime.yaml no longer declares; "+
			"declare it again with a transition out", "channel_type", typ, "state", st, "sessions", stranded[st])
	}
}

// drain makes the queued recompute writes, within the round's cap.
func (t *Timers) drain(ctx context.Context, r *timerRound) {
	for len(t.queue) > 0 {
		if r.writes >= realtimelimits.MaxTimerWritesPerRound {
			r.rep.More = true
			return
		}
		k := t.queue[0]
		t.queue = t.queue[1:]
		if t.inBackoff(k, r.now) {
			continue
		}
		ct, ok := r.cfg.ChannelType(k.Type)
		if !ok || ct.Session() == nil {
			continue
		}
		s := t.session(ctx, r, ct)
		t.write(ctx, r, k, s, timerStep(ct, s, nil, "", 0, r.now))
	}
}

// fireDue lists the sessions due within one ceiling, in one call for
// every type with timers or reminders, and for each one fires its timer,
// posts its reminder or reschedules it. A timer due at or before now wins
// over a reminder: the reminder is judged against the new state in the
// next round. One step per channel per round: a second due reminder of the
// same channel posts in the next round, which More brings at the floor. A
// reminder fires by its slot, not by the stored hint (slot plus
// [realtimelimits.ReminderSlack]), so a round run for another channel posts
// every reminder whose slot has passed.
func (t *Timers) fireDue(ctx context.Context, r *timerRound) {
	var types []string
	sessions := map[string]*realtimecfg.Session{}
	for _, ct := range r.cfg.ChannelTypes() {
		if s := ct.Session(); s != nil && s.HasSchedule() {
			types = append(types, ct.Name)
			sessions[ct.Name] = t.session(ctx, r, ct)
		}
	}
	if len(types) == 0 {
		return
	}
	limit := min(realtimelimits.MaxTimerWritesPerRound+1+len(t.backoff), realtimelimits.MaxTimerListPerRound)
	list, err := t.store.ListSessionsDue(ctx, types, r.now.Add(realtimelimits.TimerCeiling), limit)
	if err != nil {
		r.errs = append(r.errs, err)
		return
	}
	for _, c := range list {
		if t.inBackoff(c.Key, r.now) {
			continue
		}
		ct, _ := r.cfg.ChannelType(c.Key.Type) // listed only for the types above
		s := sessions[ct.Name]
		tm, due, ok := nextTimer(s, c)
		var fire *realtimecfg.SessionTimer
		if ok && !due.After(r.now) {
			fire = &tm
		}
		rm, slot, remind := nextReminder(s, c)
		remind = remind && fire == nil && !slot.After(r.now)
		hint := nextDue(s, c)
		if fire == nil && !remind && hint.Equal(channels.NormTime(c.Session.NextDueAt)) {
			r.fold(hint)
			continue
		}
		if r.writes >= realtimelimits.MaxTimerWritesPerRound {
			r.rep.More = true
			return
		}
		cur := currentSession(s, c)
		if cur.stranded {
			r.strand(c.Key)
		}
		if remind {
			t.remind(ctx, r, c, ct, s, rm, cur)
			continue
		}
		t.write(ctx, r, c.Key, s, timerStep(ct, s, fire, cur.State, cur.Generation, r.now))
	}
	if n := len(list); n == limit && !list[n-1].Session.NextDueAt.After(r.now) {
		r.rep.More = true
	}
}

// fold keeps due in the report's Next when it is the earliest after now.
func (r *timerRound) fold(due time.Time) {
	if due.IsZero() || !due.After(r.now) {
		return
	}
	if r.rep.Next.IsZero() || due.Before(r.rep.Next) {
		r.rep.Next = due
	}
}

// timerStep is the SetSession step for a channel of type ct with session
// s. With fire set it moves the session out of (state, gen) through fire,
// if the channel is still open, still in that state and generation, and
// fire is still the timer that comes due first at or before now. Without
// it, or when that check fails, it only rewrites the due time. A move
// whose timer has outputs, or that closes the channel, carries an
// unclaimed job mark (see [jobMark]).
func timerStep(
	ct realtimecfg.ChannelType, s *realtimecfg.Session, fire *realtimecfg.SessionTimer, state string, gen int64,
	now time.Time,
) channels.SessionStep {
	step := channels.SessionStep{
		Decide:     func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
		NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
		TriggeredBy: channels.SessionMarker + ct.Name, Outcome: &channels.SessionOutcome{},
	}
	if fire == nil {
		withSessionDue(&step, s)
		return step
	}
	tm := *fire
	step.Timer, step.AuditTriggeredBy = tm.Name, timerLabel(ct.Name, state, tm.Name)
	step.Decide = func(c channels.Channel) (channels.SessionMove, bool) {
		if c.State != channels.StateOpen {
			return channels.SessionMove{}, false
		}
		cur := currentSession(s, c)
		if cur.State != state || cur.Generation != gen {
			return channels.SessionMove{}, false
		}
		next, due, ok := nextTimer(s, c)
		if !ok || next.Name != tm.Name || due.After(now) {
			return channels.SessionMove{}, false
		}
		return channels.SessionMove{From: cur.State, To: tm.To}, true
	}
	if a := jobMark(ct, len(tm.Outputs()) > 0, tm.To, now); a != nil {
		step.Mark = func(channels.SessionMove) *channels.Action {
			c := *a
			c.Outputs = a.Outputs.Clone()
			return &c
		}
	}
	withSessionDue(&step, s)
	return step
}

// session is ct's session for this round: a copy in which every reminder
// whose props fail the checks of a post (see [reminderProps]) is paused.
// The check runs once per reminder and configuration; a failure is logged
// once, through the round's errors, and counted in Failed once.
func (t *Timers) session(ctx context.Context, r *timerRound, ct realtimecfg.ChannelType) *realtimecfg.Session {
	s := ct.Session()
	var paused []string
	for _, rm := range s.Reminders() {
		k := [2]string{ct.Name, rm.Name}
		c, ok := t.checked[k]
		if !ok {
			c.props, c.err = t.props(ctx, r.cfg, ct, rm, r.now)
			t.checked[k] = c
			if c.err != nil {
				r.rep.Failed++
				r.errs = append(r.errs, fmt.Errorf("reminder %s/%s is paused until realtime.yaml or the "+
					"schema changes: %w", ct.Name, rm.Name, c.err))
			}
		}
		if c.err != nil {
			paused = append(paused, rm.Name)
		}
	}
	if len(paused) == 0 {
		return s
	}
	return s.Pausing(paused)
}

// remind posts rm's reminder for c, of type ct with session s and current
// session cur. s comes from Timers.session, so rm is not paused and its
// props passed the checks.
func (t *Timers) remind(
	ctx context.Context, r *timerRound, c channels.Channel, ct realtimecfg.ChannelType, s *realtimecfg.Session,
	rm realtimecfg.SessionReminder, cur current,
) {
	props := maps.Clone(t.checked[[2]string{ct.Name, rm.Name}].props)
	listed, _ := reminderRecord(rm, cur, c)
	t.write(ctx, r, c.Key, s, reminderStep(ct, s, rm, listed, props, r.now))
}

// write runs step on k, of a type with session s, and records the outcome
// in r.
func (t *Timers) write(
	ctx context.Context, r *timerRound, k channels.Key, s *realtimecfg.Session, step channels.SessionStep,
) {
	r.writes++
	_, err := t.store.SetSession(ctx, k, r.now, step)
	switch {
	case err == nil:
	case errors.Is(err, channels.ErrConflict), errors.Is(err, channels.ErrNotFound):
		return
	default:
		// The caller logs the joined errors once per round.
		r.rep.Failed++
		t.holdBack(k, r.now)
		r.errs = append(r.errs, fmt.Errorf("channel %s waits %s before the next try: %w",
			k, realtimelimits.TimerBackoff, err))
		return
	}
	t.marker.Mark(k)
	switch {
	case step.Outcome.Moved:
		r.rep.Fired++
		if step.Mark != nil {
			r.rep.Marks++
		}
	case step.Outcome.Reminded:
		r.rep.Reminded++
	default:
		r.rep.Rescheduled++
	}
	// The hint of a reminder is its slot plus the slack, so a reminder
	// whose slot has passed is due work even while its hint is ahead.
	after := channels.Channel{
		State: channels.StateOpen, CreatedAt: step.Outcome.CreatedAt, Session: step.Outcome.Session,
	}
	_, due, timerOK := nextTimer(s, after)
	_, slot, remindOK := nextReminder(s, after)
	next := step.Outcome.Session.NextDueAt
	switch {
	case timerOK && !due.After(r.now), remindOK && !slot.After(r.now), !next.IsZero() && !next.After(r.now):
		r.rep.More = true
	default:
		r.fold(next)
	}
}

// inBackoff reports whether k waits out a backoff at now, and drops a
// backoff that has ended.
func (t *Timers) inBackoff(k channels.Key, now time.Time) bool {
	until, ok := t.backoff[k]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(t.backoff, k)
	return false
}

// holdBack puts k in backoff for [realtimelimits.TimerBackoff], dropping
// the oldest entry past [realtimelimits.MaxTimerBackoffKeys].
func (t *Timers) holdBack(k channels.Key, now time.Time) {
	kept := t.order[:0]
	for _, o := range t.order {
		if _, ok := t.backoff[o]; ok && o != k {
			kept = append(kept, o)
		}
	}
	kept = append(kept, k)
	t.order = kept
	t.backoff[k] = now.Add(realtimelimits.TimerBackoff)
	for len(t.order) > realtimelimits.MaxTimerBackoffKeys {
		delete(t.backoff, t.order[0])
		t.order = t.order[1:]
	}
}
