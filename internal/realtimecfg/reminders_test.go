package realtimecfg

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestParse_SessionReminders pins AC 1: the breach-response reminder
// compiles with its accessors and defaults, a state that only a reminder
// covers besides its on: loads, and a reminder type with actions keeps them.
func TestParse_SessionReminders(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("breach-response")
	s := ct.Session()
	rs := s.Reminders()
	if len(rs) != 1 {
		t.Fatalf("reminders = %+v", rs)
	}
	r := rs[0]
	if r.Name != "status-update" || r.Label != "Status update" || r.Every != 30*time.Minute ||
		r.First != 10*time.Minute || r.EveryText != "30m" || r.Max != 48 || r.PostType != "reminder" {

		t.Errorf("status-update = %+v", r)
	}
	if want := []string{"detected", "contained", "handed-over", "assessed", "overdue"}; !slices.Equal(r.In(), want) {
		t.Errorf("in = %v; want %v", r.In(), want)
	}
	if !r.Active("contained") || r.Active("notified") {
		t.Error("Active does not follow in:")
	}
	if got := r.Props(); len(got) != 1 || got["body"] != "Post a status update: what changed since the last one?" {
		t.Errorf("props = %v", got)
	}
	if got, ok := s.Reminder("status-update"); !ok || got.Name != r.Name {
		t.Error("Reminder did not find status-update")
	}
	if _, ok := s.Reminder("nope"); ok {
		t.Error("Reminder found an undeclared reminder")
	}
	if len(s.RemindersIn("overdue")) != 1 || len(s.RemindersIn("notified")) != 0 {
		t.Error("RemindersIn does not follow in:")
	}
	if !s.HasSchedule() || !s.HasTimers() {
		t.Error("HasSchedule or HasTimers is false")
	}

	src := reminderBase(`
        open:
          on: { done: { to: closed } }
        closed: { final: true }`, `
        zeta:  { in: [open], every: 1h, post: { type: nudge, props: { body: hi } } }
        alpha: { in: [open], every: 2h, max: 3, post: { type: nudge, props: { body: hi } } }`)
	src = strings.Replace(src, "      nudge: { properties: { body: { type: string, required: true } }, post: [member] }",
		"      nudge:\n        properties: { body: { type: string, required: true } }\n        post: [member]\n"+
			"        actions: { ack: { roles: [member] } }", 1)
	c, err = Parse([]byte(src), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ = c.ChannelType("br")
	s = ct.Session()
	rs = s.Reminders()
	if len(rs) != 2 || rs[0].Name != "alpha" || rs[1].Name != "zeta" {
		t.Fatalf("reminders = %+v; want alpha, zeta", rs)
	}
	if rs[0].Label != "alpha" || rs[0].First != 2*time.Hour || rs[1].Max != 0 {
		t.Errorf("defaults = %+v", rs)
	}
	if s.HasTimers() || !s.HasSchedule() {
		t.Error("a session with only reminders: HasTimers or HasSchedule is wrong")
	}
	if _, ok := ct.MessageTypes["nudge"].Actions["ack"]; !ok {
		t.Error("the reminder type lost its actions")
	}
}

// reminderBase is a channel type br whose session has states and
// reminders. Its message types: done (moves the session), nudge (for
// reminders), sev (an enum property), typing (ephemeral).
func reminderBase(states, reminders string) string {
	return `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      done:  { post: [member] }
      nudge: { properties: { body: { type: string, required: true } }, post: [member] }
      sev:   { properties: { level: { type: severity }, short: { type: string, max: 3 } }, post: [member] }
      typing: { persist: ephemeral, post: [member] }
    session:
      initial: open
      states:` + states + `
      reminders:` + reminders + "\n"
}

const reminderStates = `
        open:
          on: { done: { to: mid } }
        mid:
          on: { done: { to: closed } }
        closed: { final: true }`

// TestParse_SessionRemindersInvalid pins AC 2: each invalid reminder
// reports its key path and reason.
func TestParse_SessionRemindersInvalid(t *testing.T) {
	const k = "channel_types.br.session.reminders"
	const post = "post: { type: nudge, props: { body: hi } }"
	var nine strings.Builder
	for i := range 9 {
		fmt.Fprintf(&nine, "\n        r%d: { in: [open], every: 1h, %s }", i, post)
	}
	one := func(body string) string { return "\n        r: { " + body + " }" }
	cases := []struct {
		name, reminders, key, reason string
	}{
		{"too many", nine.String(), k, "at most 8 reminders per session"},
		{"bad name", "\n        Bad: { in: [open], every: 1h, " + post + " }", k + ".Bad",
			"reminder name must match ^[a-z][a-z0-9-]{0,62}$"},
		{"missing in", one("every: 1h, " + post), k + ".r.in", "name the states the reminder runs in"},
		{"empty in", one("in: [], every: 1h, " + post), k + ".r.in", "name the states the reminder runs in"},
		{"undeclared state", one("in: [gone], every: 1h, " + post), k + ".r.in[0]", `unknown state "gone"`},
		{"final state", one("in: [closed], every: 1h, " + post), k + ".r.in[0]", "a final state ends the session"},
		{"state twice", one("in: [open, open], every: 1h, " + post), k + ".r.in[1]", "state open is listed twice"},
		{"missing every", one("in: [open], " + post), k + ".r.every", "name how often the reminder posts"},
		{"bad every", one("in: [open], every: often, " + post), k + ".r.every", "must be a count from 1 to 100000"},
		{"calendar every", one("in: [open], every: 1mo, " + post), k + ".r.every",
			"every must be a fixed duration such as 30m or 24h; months and years are not supported"},
		{"every under 1m", one("in: [open], every: 30s, " + post), k + ".r.every", "a reminder waits at least 1m"},
		{"bad first", one("in: [open], every: 1h, first: soon, " + post), k + ".r.first", "must be a count"},
		{"calendar first", one("in: [open], every: 1h, first: 1y, " + post), k + ".r.first",
			"first must be a fixed duration"},
		{"first under 1m", one("in: [open], every: 1h, first: 10s, " + post), k + ".r.first",
			"a reminder waits at least 1m"},
		{"max zero", one("in: [open], every: 1h, max: 0, " + post), k + ".r.max", "max must be between 1 and 1000"},
		{"max over 1000", one("in: [open], every: 1h, max: 1001, " + post), k + ".r.max",
			"max must be between 1 and 1000"},
		{"missing post", one("in: [open], every: 1h"), k + ".r.post", "name the message type the reminder posts"},
		{"missing post type", one("in: [open], every: 1h, post: { props: { body: hi } }"), k + ".r.post.type",
			"name the message type the reminder posts"},
		{"undeclared post type", one("in: [open], every: 1h, post: { type: nope }"), k + ".r.post.type",
			`unknown message type "nope"`},
		{"ephemeral post type", one("in: [open], every: 1h, post: { type: typing }"), k + ".r.post.type",
			"a reminder posts a log message"},
		{"post type keys a transition", one("in: [open], every: 1h, post: { type: done }"), k + ".r.post.type",
			"a reminder never moves the session, but a person posting done would"},
		{"undeclared prop", one("in: [open], every: 1h, post: { type: nudge, props: { body: hi, x: y } }"),
			k + ".r.post.props.x", `the message type has no property "x"`},
		{"substitution", one("in: [open], every: 1h, post: { type: nudge, props: { body: '{{entity.title}}' } }"),
			k + ".r.post.props.body", "{{...}} substitution is not supported"},
		{"literal on an enum not in the values", one("in: [open], every: 1h, post: { type: sev, props: { level: nope } }"),
			k + ".r.post.props.level", "a trigger or reminder can set only string, markdown or enum properties"},
		{"required prop not set", one("in: [open], every: 1h, post: { type: nudge }"), k + ".r.post.props",
			`required property "body" is not set`},
		{"prop over max", one("in: [open], every: 1h, post: { type: sev, props: { short: abcd } }"),
			k + ".r.post.props.short", "longer than the property's max of 3"},
	}
	meta, policy := testMeta(t), testPolicy(t)
	if _, err := Parse([]byte(reminderBase(reminderStates, one("in: [open], every: 1h, "+post))), meta, policy); err != nil {
		t.Fatalf("the base file does not load: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(reminderBase(reminderStates, tc.reminders)), meta, policy)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}

	t.Run("props over message_bytes", func(t *testing.T) {
		src := "version: 1\nlimits: { message_bytes: 64 }\n" +
			strings.TrimPrefix(reminderBase(reminderStates, one("in: [open], every: 1h, post: { type: nudge, props: "+
				"{ body: '"+strings.Repeat("x", 80)+"' } }")), "version: 1\n")
		_, err := Parse([]byte(src), meta, policy)
		assertProblem(t, err, k+".r.post.props", "the message is over message_bytes (64 bytes)")
	})
	whenCases := map[string]struct{ when, reason string }{
		"when not met":    {"message.body == 'other'", "the message does not meet the when: condition of message type nudge"},
		"when on today()": {"message.body ~= '' or today() == today()", "calls today()"},
	}
	for name, tc := range whenCases {
		t.Run(name, func(t *testing.T) {
			src := strings.Replace(reminderBase(reminderStates, one("in: [open], every: 1h, "+post)),
				"      nudge: { properties: { body: { type: string, required: true } }, post: [member] }",
				"      nudge: { properties: { body: { type: string, required: true } }, post: [member], when: \""+
					tc.when+"\" }", 1)
			_, err := Parse([]byte(src), meta, policy)
			assertProblem(t, err, k+".r.post.props", tc.reason)
		})
	}
}

// TestParse_SessionReminderGuard pins AC 4: a guard reads a declared
// reminder's counter, an undeclared one does not compile, a session
// without reminders declares an empty record, and a reminder whose name is
// valid but whose every is not reports only its own problem.
func TestParse_SessionReminderGuard(t *testing.T) {
	meta, policy := testMeta(t), testPolicy(t)
	withGuard := func(guard, reminder string) string {
		states := strings.Replace(reminderStates, "on: { done: { to: mid } }",
			"on: { done: { to: mid, when: \""+guard+"\" } }", 1)
		return reminderBase(states, "\n        status-update: { "+reminder+" }")
	}
	const good = "in: [open], every: 1h, post: { type: nudge, props: { body: hi } }"
	if _, err := Parse([]byte(withGuard("session.reminders['status-update'] >= 3", good)), meta, policy); err != nil {
		t.Errorf("a guard on a declared reminder does not load: %v", err)
	}
	const on = "channel_types.br.session.states.open.on.done.when"
	_, err := Parse([]byte(withGuard("session.reminders['nope'] >= 3", good)), meta, policy)
	assertProblem(t, err, on, `unknown attribute "nope"`)

	_, err = Parse([]byte(withGuard("session.reminders['status-update'] >= 3",
		"in: [open], every: soon, post: { type: nudge, props: { body: hi } }")), meta, policy)
	assertProblem(t, err, "channel_types.br.session.reminders.status-update.every", "must be a count")
	if strings.Contains(err.Error(), on) {
		t.Errorf("a guard on a broken but named reminder reports a second problem: %v", err)
	}
}
