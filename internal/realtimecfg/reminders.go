package realtimecfg

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// SessionReminder is one `session.reminders.<name>` entry: while the
// session is in one of its In states and the channel is open, it posts a
// message of type PostType every Every, the first one First after the
// session entered those states. It never moves the session.
//
// Its slice and map fields are unexported and their accessors return
// copies, so a copy of the value shares nothing a caller can change.
type SessionReminder struct {
	Name  string
	Label string
	// Every is the time between two reminders.
	Every time.Duration
	// First is the time from the start of the span to the first reminder.
	// It defaults to Every.
	First time.Duration
	// EveryText is Every as written in the file, for the wire.
	EveryText string
	// Max caps the reminders of one span; 0 means no limit.
	Max int64
	// PostType is the message type the reminder posts.
	PostType string
	in       []string
	inSet    map[string]bool
	props    map[string]string
}

// In returns the states the reminder runs in, in file order. The slice is
// a copy.
func (r SessionReminder) In() []string { return slices.Clone(r.in) }

// Active reports whether the reminder runs in state.
func (r SessionReminder) Active(state string) bool { return r.inSet[state] }

// Props returns the literal property values of the reminder message. The
// map is a copy.
func (r SessionReminder) Props() map[string]string {
	out := make(map[string]string, len(r.props))
	maps.Copy(out, r.props)
	return out
}

// Reminders returns the session's reminders, sorted by name. The slice is a
// copy.
func (s *Session) Reminders() []SessionReminder { return slices.Clone(s.reminders) }

// Reminder returns the reminder called name.
func (s *Session) Reminder(name string) (SessionReminder, bool) {
	for _, r := range s.reminders {
		if r.Name == name {
			return r, true
		}
	}
	return SessionReminder{}, false
}

// RemindersIn returns the reminders that run in state, sorted by name.
func (s *Session) RemindersIn(state string) []SessionReminder {
	var out []SessionReminder
	for _, r := range s.reminders {
		if r.Active(state) {
			out = append(out, r)
		}
	}
	return out
}

// Pausing returns a copy of s in which the named reminders are paused. A
// paused reminder stays declared, active in its states and counted in
// guards, and its records are kept; only [Session.Paused] reports it, so
// the timer loop schedules no slot for it. The loop pauses a reminder whose
// props no longer pass the checks of a post. s is not changed.
func (s *Session) Pausing(names []string) *Session {
	c := *s
	c.paused = make(map[string]bool, len(s.paused)+len(names))
	maps.Copy(c.paused, s.paused)
	for _, n := range names {
		c.paused[n] = true
	}
	return &c
}

// Paused reports whether reminder name is paused in this copy of the
// session (see [Session.Pausing]).
func (s *Session) Paused(name string) bool { return s.paused[name] }

// HasSchedule reports whether the session declares a timer or a reminder,
// so the timer loop has work for it.
func (s *Session) HasSchedule() bool { return s.HasTimers() || len(s.reminders) > 0 }

// reminderCompiler compiles the `session.reminders` of one channel type
// into s, reporting problems to v. Besides the entries it reads the raw
// message types of the channel type, the compiled ones in ct, the raw
// states and the effective limits. It keeps these methods off validator.
type reminderCompiler struct {
	v        *validator
	rawTypes map[string]rawMessageType
	states   map[string]rawSessionState
	limits   Limits
	s        *Session
	ct       *ChannelType
}

// compile compiles `session.reminders` into c.s, sorted by name. A
// reminder with a problem is left out, but every reminder whose name
// matches the grammar is still declared to the guards (s.guardReminders),
// so a guard naming it reports no second error.
func (c reminderCompiler) compile(key string, raw map[string]rawReminder) {
	v, s := c.v, c.s
	if len(raw) > realtimelimits.MaxSessionReminders {
		v.add(key, fmt.Sprintf("at most %d reminders per session", realtimelimits.MaxSessionReminders))
	}
	for _, name := range sortedKeys(raw) {
		rk := key + "." + name
		if !namePattern.MatchString(name) {
			v.add(rk, "reminder name must match ^[a-z][a-z0-9-]{0,62}$")
			continue
		}
		s.guardReminders = append(s.guardReminders, name)
		if r, ok := c.reminder(rk, name, raw[name]); ok {
			s.reminders = append(s.reminders, r)
		}
	}
}

// reminder compiles one `session.reminders.<name>` entry. ok is false when
// it has a problem.
func (c reminderCompiler) reminder(key, name string, raw rawReminder) (SessionReminder, bool) {
	v, s := c.v, c.s
	r := SessionReminder{Name: name, Label: orDefault(raw.Label, name), inSet: map[string]bool{},
		props: map[string]string{}}
	ok := true
	bad := func(k, reason string) {
		v.add(k, reason)
		ok = false
	}
	if len(raw.In) == 0 {
		bad(key+".in", "name the states the reminder runs in")
	}
	for i, st := range raw.In {
		ik := fmt.Sprintf("%s.in[%d]", key, i)
		_, declared := s.index[st]
		switch {
		case !declared:
			bad(ik, unknownState(st))
		case c.states[st].Final:
			bad(ik, "a final state ends the session; a reminder there would post until the channel closes")
		case r.inSet[st]:
			bad(ik, fmt.Sprintf("state %s is listed twice", st))
		default:
			r.inSet[st] = true
			r.in = append(r.in, st)
		}
	}
	if raw.Every == "" {
		bad(key+".every", "name how often the reminder posts, such as 30m")
	} else if d, reason := reminderDuration(raw.Every, "every"); reason != "" {
		bad(key+".every", reason)
	} else {
		r.Every, r.EveryText = d, raw.Every
	}
	r.First = r.Every
	if raw.First != "" {
		if d, reason := reminderDuration(raw.First, "first"); reason != "" {
			bad(key+".first", reason)
		} else {
			r.First = d
		}
	}
	if raw.Max != nil {
		if *raw.Max < 1 || *raw.Max > realtimelimits.MaxReminderCount {
			bad(key+".max", fmt.Sprintf("max must be between 1 and %d", realtimelimits.MaxReminderCount))
		} else {
			r.Max = int64(*raw.Max)
		}
	}
	if !c.post(key+".post", raw.Post, &r) {
		ok = false
	}
	return r, ok
}

// reminderDuration parses a reminder's every or first: a fixed period of
// at least [realtimelimits.MinSessionTimer]. reason is empty when it is
// valid.
func reminderDuration(s, field string) (d time.Duration, reason string) {
	p, err := ParsePeriod(s)
	if err != nil {
		return 0, err.Error()
	}
	d, fixed := p.Fixed()
	switch {
	case !fixed:
		return 0, field + " must be a fixed duration such as 30m or 24h; months and years are not supported " +
			"for reminders"
	case d < realtimelimits.MinSessionTimer:
		return 0, "a reminder waits at least 1m"
	}
	return d, ""
}

// post checks a reminder's `post:` and stores its type and literal props in
// r. ok is false when it has a problem.
func (c reminderCompiler) post(key string, raw *rawTriggerPost, r *SessionReminder) bool {
	v := c.v
	if raw == nil {
		v.add(key, "name the message type the reminder posts under post.type")
		return false
	}
	tk := key + ".type"
	rmt, declared := c.rawTypes[raw.Type]
	switch {
	case raw.Type == "":
		v.add(tk, "name the message type the reminder posts")
		return false
	case !declared:
		v.add(tk, fmt.Sprintf("unknown message type %q; a reminder posts a message type of this channel type",
			raw.Type))
		return false
	case Persist(rmt.Persist) == PersistEphemeral:
		v.add(tk, "a reminder posts a log message; an ephemeral message would reach only the readers "+
			"connected to the leading process")
		return false
	}
	for _, sn := range sortedKeys(c.states) {
		if _, keyed := c.states[sn].On[raw.Type]; keyed {
			v.add(tk, fmt.Sprintf("a reminder never moves the session, but a person posting %s would; "+
				"use a message type no transition is keyed by", raw.Type))
			return false
		}
	}
	r.PostType = raw.Type
	before := len(v.problems)
	v.literalProps(key+".props", raw.Props, &rmt, r.props)
	if mt := c.ct.MessageTypes[raw.Type]; mt != nil {
		v.literalMessage(key+".props", mt, r.props, c.limits)
	}
	return len(v.problems) == before
}
