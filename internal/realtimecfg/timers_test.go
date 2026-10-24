package realtimecfg

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestParse_SessionTimers pins AC 1 and AC 3: the breach-response timers
// compile sorted by name with their defaults, a state with on: and no
// after: and a state with only after: load, and so do a cycle through a
// message transition and a calendar period.
func TestParse_SessionTimers(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("breach-response")
	s := ct.Session()
	if !s.HasTimers() {
		t.Error("HasTimers = false")
	}
	detected, _ := s.State("detected")
	got := detected.Timers()
	if len(got) != 1 {
		t.Fatalf("detected timers = %+v", got)
	}
	nc := got[0]
	if nc.Name != "no-containment" || nc.Label != "Not contained within a day" || nc.To != "overdue" ||
		nc.Since != TimerSinceState || nc.Period.String() != "24h" {

		t.Errorf("no-containment = %+v", nc)
	}
	if d, fixed := nc.Period.Fixed(); !fixed || d != 24*time.Hour {
		t.Errorf("period = %v, %v", d, fixed)
	}
	contained, _ := s.State("contained")
	if tm := contained.Timers(); len(tm) != 1 || tm[0].Since != TimerSinceSession {
		t.Errorf("contained timers = %+v", tm)
	}
	overdue, ok := s.State("overdue")
	if !ok || len(overdue.Timers()) != 0 || len(overdue.Transitions()) != 1 {
		t.Errorf("overdue = %+v", overdue)
	}
	room, _ := c.ChannelType("incident-room")
	if room.Session() != nil {
		t.Error("incident-room has a session")
	}

	cases := map[string]string{
		"only after, sorted, label defaults": `
        open:
          after:
            zeta:  { period: 2h, to: done }
            alpha: { period: 1h, to: done }
        done: { final: true }`,
		"cycle through a message transition": `
        open:
          after:
            late: { period: 1h, to: waiting }
        waiting:
          on:
            contained: { to: open }
          after:
            gone: { period: 1mo, to: done }
        done: { final: true }`,
	}
	for name, states := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := Parse([]byte(timerBase(states)), testMeta(t), testPolicy(t))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("br")
			open, _ := ct.Session().State("open")
			tm := open.Timers()
			if len(tm) == 0 || !ct.Session().HasTimers() {
				t.Fatalf("open timers = %+v", tm)
			}
			if name == "only after, sorted, label defaults" &&
				(tm[0].Name != "alpha" || tm[1].Name != "zeta" || tm[0].Label != "alpha") {

				t.Errorf("timers = %+v; want alpha, zeta with the name as label", tm)
			}
		})
	}
}

// timerBase is a channel type br whose session states are states.
func timerBase(states string) string {
	return `version: 1
channel_types:
  br:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      contained: { post: [member] }
    session:
      initial: open
      states:` + states + "\n"
}

// TestParse_SessionTimersInvalid pins AC 2: each invalid timer reports its
// key path and reason.
func TestParse_SessionTimersInvalid(t *testing.T) {
	const k = "channel_types.br.session.states"
	var nine strings.Builder
	for i := range 9 {
		fmt.Fprintf(&nine, "            t%d: { period: 1h, to: done }\n", i)
	}
	cases := []struct {
		name, states, key, reason string
	}{
		{"after on a final state", `
        open: { on: { contained: { to: done } } }
        done: { final: true, after: { x: { period: 1h, to: open } } }`,
			k + ".done.after", "a final state has no timers; drop after or remove final: true"},
		{"neither on nor after", `
        open: { label: Open }
        done: { final: true }`,
			k + ".open", "a state that is not final needs a transition under on or a timer under after"},
		{"too many timers", "\n        open:\n          after:\n" + nine.String() + "        done: { final: true }",
			k + ".open.after", "at most 8 timers per state"},
		{"bad timer name", `
        open: { after: { Late: { period: 1h, to: done } } }
        done: { final: true }`,
			k + ".open.after.Late", "timer name must match ^[a-z][a-z0-9-]{0,62}$"},
		{"missing period", `
        open: { after: { late: { to: done } } }
        done: { final: true }`,
			k + ".open.after.late.period", "name how long the timer waits, such as 72h"},
		{"bad period", `
        open: { after: { late: { period: soon, to: done } } }
        done: { final: true }`,
			k + ".open.after.late.period", "must be a count from 1 to 100000 and a unit"},
		{"period under a minute", `
        open: { after: { late: { period: 30s, to: done } } }
        done: { final: true }`,
			k + ".open.after.late.period", "a timer waits at least 1m"},
		{"bad since", `
        open: { after: { late: { period: 1h, since: entity, to: done } } }
        done: { final: true }`,
			k + ".open.after.late.since", "since must be state or session"},
		{"from key", `
        open: { after: { late: { period: 1h, from: state, to: done } } }
        done: { final: true }`,
			k + ".open.after.late.from", "use since: state or since: session"},
		{"missing to", `
        open: { after: { late: { period: 1h } } }
        done: { final: true }`,
			k + ".open.after.late.to", "name the state this timer moves to"},
		{"undeclared to", `
        open: { after: { late: { period: 1h, to: gone } } }
        done: { final: true }`,
			k + ".open.after.late.to", `unknown state "gone"; declare it under states`},
		{"self-loop since state", `
        open:
          on: { contained: { to: done } }
          after: { again: { period: 1h, to: open } }
        done: { final: true }`,
			k + ".open.after.again", "timers form a loop (open -> open)"},
		{"self-loop since session", `
        open:
          on: { contained: { to: done } }
          after: { again: { period: 1h, since: session, to: open } }
        done: { final: true }`,
			k + ".open.after.again", "timers form a loop (open -> open)"},
		{"mixed two-state cycle", `
        open:
          on: { contained: { to: done } }
          after: { late: { period: 1h, since: session, to: overdue } }
        overdue:
          after: { back: { period: 1m, to: open } }
        done: { final: true }`,
			k + ".overdue.after.back", "timers form a loop (open -> overdue -> open) that would move the session " +
				"forever without a message; break it with a transition under on"},
	}
	meta, policy := testMeta(t), testPolicy(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(timerBase(tc.states)), meta, policy)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestParse_SessionTimersAllOneLoad pins the "all in one load" half of
// AC 2: two broken timers are both reported.
func TestParse_SessionTimersAllOneLoad(t *testing.T) {
	_, err := Parse([]byte(timerBase(`
        open:
          after:
            a: { period: 1h }
            b: { period: 5s, to: done }
        done: { final: true }`)), testMeta(t), testPolicy(t))
	assertProblem(t, err, "channel_types.br.session.states.open.after.a.to", "name the state")
	assertProblem(t, err, "channel_types.br.session.states.open.after.b.period", "at least 1m")
}
