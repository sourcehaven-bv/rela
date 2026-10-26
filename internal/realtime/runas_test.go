package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// watchRealtime declares watch, a session type on tickets whose timer and
// trigger moves write as run_as: detected times out into overdue with a
// set, a trigger's escalated moves it to escalated with a set, and from
// there a trigger's wrapup or the give-up timer ends it in closed, which
// on_final closes. escalated's stale timer has no outputs and is not
// final. The triggers post into watch when a ticket's status changes.
func watchRealtime(runAs string) string {
	return `
version: 1
channel_types:
  watch:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
      wrapup:    { properties: { note: { type: string } }, post: [lead] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      initial: detected
      on_final: close
      run_as: ` + runAs + `
      states:
        detected:
          on:
            escalated:
              to: escalated
              by: trigger
              outputs:
                - set: { properties: { status: "{{message.note}}" } }
          after:
            no-containment:
              period: 24h
              to: overdue
              outputs:
                - set: { properties: { status: overdue, due: "{{today}}" } }
        escalated:
          on:
            wrapup: { to: closed, by: trigger }
          after:
            stale: { period: 1h, to: overdue }
        overdue:
          after:
            give-up: { period: 24h, to: closed }
        closed: { final: true }
triggers:
  escalate:
    entity_type: ticket
    on: enter
    when: "entity.status == 'escalated'"
    channel: watch
    post: { type: escalated, props: { note: "escalated" } }
  wrap:
    entity_type: ticket
    on: enter
    when: "entity.status == 'done'"
    channel: watch
    post: { type: wrapup, props: { note: "done" } }
  chat:
    entity_type: ticket
    on: enter
    when: "entity.status == 'chatty'"
    channel: watch
    post: { type: text, props: { body: "hello" } }
`
}

const breachBot = "system:breach-bot"

var (
	watchAddr = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "watch"}
	watchKey  = channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "watch"}
)

// newWatchHarness is a timer harness over watchRealtime with run_as
// system:breach-bot and its watch channel on TKT-1 open since t0.
func newWatchHarness(t *testing.T) *timerHarness {
	t.Helper()
	h := newHarness(t, withYAML(watchRealtime(breachBot)))
	th := &timerHarness{harness: h, tm: h.timers(t)}
	th.postTo(t, user("alice"), watchAddr, "text", props("body", "hi"))
	return th
}

// lastMark returns the outputs mark of the last notice of watchKey.
func (h *harness) lastMark(t *testing.T) (channels.Message, *channels.OutputMark) {
	t.Helper()
	msgs, err := h.store.Store.Messages(context.Background(), watchKey, channels.Page{})
	require.NoError(t, err)
	for i := len(msgs) - 1; i >= 0; i-- {
		if m := msgs[i]; m.Type == realtimecfg.SessionNoticeType {
			require.True(t, isOutputs(m.Action), "the notice at seq %d carries a mark", m.Seq)
			return m, m.Action.Outputs
		}
	}
	t.Fatal("no session notice")
	return channels.Message{}, nil
}

// TestTimers_ReportsMarks pins the timer side: a timer with outputs and a
// timer into a final state write an unclaimed job mark, counted in Marks;
// a timer with neither writes none.
func TestTimers_ReportsMarks(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	rep := th.round(t)
	require.Equal(t, 1, rep.Fired)
	require.Equal(t, 1, rep.Marks)
	n, mark := th.lastMark(t)
	require.Equal(t, channels.ActionPending, n.Action.Status)
	require.True(t, mark.Job)
	require.Equal(t, "no-containment", mark.Timer)
	require.Zero(t, mark.Cause)
	require.Equal(t, breachBot, mark.Writer.User)
	require.Equal(t, principal.ToolRealtime, mark.Writer.Tool)
	require.Equal(t, "ticket", mark.AnchorType)
	require.Empty(t, mark.Claim, "unclaimed")
	// A job mark is due from the move: the runner's next clock reading.
	th.clock.Add(time.Microsecond)
	pending, err := th.store.StalePending(context.Background(), th.clock.Now())
	require.NoError(t, err)
	require.Len(t, pending, 1, "the runner sees it at once")
	require.Equal(t, channels.OutputsAction, pending[0].Name)

	th.at(48 * time.Hour)
	rep = th.round(t)
	require.Equal(t, 1, rep.Marks, "a move into a final state carries the close")
	_, mark = th.lastMark(t)
	require.True(t, mark.Job)
	require.Equal(t, "give-up", mark.Timer)
	require.Equal(t, systemAuthor().User, mark.Writer.User, "a close-only mark writes as the leader")
}

func TestTimers_NoOutputsNoMark(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.svcPostAsTrigger(t, "escalated", props("note", "x"))
	th.at(2 * time.Hour)
	rep := th.round(t)
	require.Equal(t, 1, rep.Fired, "stale fires")
	require.Zero(t, rep.Marks, "stale has no outputs and overdue is not final")
}

// svcPostAsTrigger posts typ into the watch channel as a trigger would.
func (th *timerHarness) svcPostAsTrigger(t *testing.T, typ string, p map[string]any) {
	t.Helper()
	snap := th.cfg.Load()
	ct, ok := snap.Config.ChannelType("watch")
	require.True(t, ok)
	w := &systemWriter{store: th.store, local: th.local, cfg: th.cfg, dueWake: th.wake}
	_, err := w.post(context.Background(), snap, th.clock.Now(), watchKey, ct, ct.MessageTypes[typ], p,
		"trigger:test", true)
	require.NoError(t, err)
}

// TestTriggers_ReportsMarks pins the trigger side: a post that moves the
// session through a transition with outputs writes a job mark as run_as;
// a move into a final state writes a close-only one; a post that moves
// nothing writes none.
func TestTriggers_ReportsMarks(t *testing.T) {
	t.Parallel()
	th := newTriggerHarness(t, watchRealtime(breachBot))
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)

	th.ticket(1, map[string]any{"status": "chatty"})
	rep := th.round(t)
	require.Equal(t, 1, rep.Posted)
	require.Zero(t, rep.Marks, "a text moves nothing")

	th.ticket(1, map[string]any{"status": "escalated"})
	rep = th.round(t)
	require.Equal(t, 1, rep.Posted)
	require.Equal(t, 1, rep.Marks)
	n, mark := th.lastMark(t)
	require.True(t, mark.Job)
	require.Empty(t, mark.Timer)
	require.Equal(t, n.Seq-1, mark.Cause)
	require.Equal(t, breachBot, mark.Writer.User)
	require.Equal(t, "escalated", mark.To)

	th.ticket(1, map[string]any{"status": "done"})
	rep = th.round(t)
	require.Equal(t, 1, rep.Marks, "the final move carries the close")
	_, mark = th.lastMark(t)
	require.True(t, mark.Job)
	require.Equal(t, systemAuthor().User, mark.Writer.User)
}

// TestOutputs_TimerRunsAsRunAs runs a timer mark: the writer is asked to
// write as run_as, never as a poster, and the result is done.
func TestOutputs_TimerRunsAsRunAs(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	th.round(t)
	th.clock.Add(time.Second)
	w := &fakeOutputs{}
	require.Equal(t, 1, th.runOutputs(t, w).Ran)
	reqs := w.requests()
	require.Len(t, reqs, 1)
	require.Equal(t, breachBot, reqs[0].RunAs)
	require.True(t, reqs[0].Job)
	require.Nil(t, reqs[0].Retry)
	require.Equal(t, "ticket", reqs[0].Target.AnchorType)
	require.Equal(t, map[string]any{"status": "overdue", "due": "2026-10-13"}, reqs[0].Props)
	require.Equal(t, "session:TKT-1/watch/detected->overdue#1/0", reqs[0].Origin)
	n, mark := th.lastMark(t)
	require.Equal(t, channels.ActionDone, n.Action.Status)
	require.Equal(t, channels.OutputDone, mark.Results[0].Status)
}

// TestOutputs_TimerFinalClosesUnit runs the close-only mark of a timer
// move into a final state: the channel closes and the result is a close.
func TestOutputs_TimerFinalClosesUnit(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	th.round(t)
	th.clock.Add(time.Second)
	w := &fakeOutputs{}
	th.runOutputs(t, w)
	th.at(48 * time.Hour)
	th.round(t)
	th.clock.Add(time.Second)
	require.Equal(t, 1, th.runOutputs(t, w).Ran)
	_, mark := th.lastMark(t)
	require.Len(t, mark.Results, 1)
	require.Equal(t, "close", mark.Results[0].Kind)
	require.Equal(t, channels.OutputDone, mark.Results[0].Status)
	require.Equal(t, channels.StateClosed, th.head(t, watchKey).State)
	require.Len(t, w.requests(), 1, "the close writes no graph output")
}

// TestOutputs_RunAsChangedUnit changes run_as between the move and the
// run: every output fails run_as_changed and nothing is written.
func TestOutputs_RunAsChangedUnit(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	th.round(t)
	th.clock.Add(time.Second)
	th.cfg.set(Snapshot{Config: testCompiled(t, watchRealtime("system:breach-bot-2"))})
	w := &fakeOutputs{}
	require.Equal(t, 1, th.runOutputs(t, w).Ran)
	require.Empty(t, w.requests())
	n, mark := th.lastMark(t)
	require.Equal(t, channels.ActionDone, n.Action.Status)
	require.Len(t, mark.Results, 1)
	require.Equal(t, channels.OutputFailed, mark.Results[0].Status)
	require.Equal(t, OutputCodeRunAsChanged, mark.Results[0].Code)
}

// TestOutputs_OneSnapshotPerMark pins one config load per mark, and that a
// reload between the claim and the run is read whole: the new run_as
// against the mark's writer, never the old outputs with the new run_as.
func TestOutputs_OneSnapshotPerMark(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	th.round(t)
	th.clock.Add(time.Second)
	cc := &countingConfig{ConfigSource: th.cfg}
	o, err := NewOutputs(OutputsDeps{Store: th.store, Config: cc, Writer: &fakeOutputs{}, Local: th.local,
		Clock: th.clock.Now})
	require.NoError(t, err)
	th.store.onSetAction = func() {
		th.store.onSetAction = nil
		th.cfg.set(Snapshot{Config: testCompiled(t, watchRealtime("system:breach-bot-2"))})
	}
	rep, err := o.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, rep.Ran)
	require.Equal(t, int64(2), cc.n.Load(), "one load for the round, one for the mark")
	_, mark := th.lastMark(t)
	require.Equal(t, OutputCodeRunAsChanged, mark.Results[0].Code)
}

// TestOutputs_TimerMarkReadBudget pins the reads: a timer round with
// outputs reads the store as often as one without, and the runner reads a
// timer mark's notice in one Messages page.
func TestOutputs_TimerMarkReadBudget(t *testing.T) {
	t.Parallel()
	count := func(th *timerHarness) (int64, int64) {
		return th.store.messages.Load(), th.store.heads.Load()
	}
	plain := newHarness(t, withYAML(timerRealtime))
	pth := &timerHarness{harness: plain, tm: plain.timers(t)}
	pth.open(t, "TKT-1")
	pth.at(24 * time.Hour)
	m0, h0 := count(pth)
	require.Equal(t, 1, pth.round(t).Fired)
	m1, h1 := count(pth)

	th := newWatchHarness(t)
	th.at(24 * time.Hour)
	w0, x0 := count(th)
	require.Equal(t, 1, th.round(t).Marks)
	w1, x1 := count(th)
	require.Equal(t, m1-m0, w1-w0, "no Messages read per mark")
	require.Equal(t, h1-h0, x1-x0, "no Head read per mark")

	th.clock.Add(time.Second)
	before := th.store.messages.Load()
	require.Equal(t, 1, th.runOutputs(t, &fakeOutputs{}).Ran)
	require.Equal(t, int64(1), th.store.messages.Load()-before, "one Messages page")
}

// TestSessionView_TimerOutputs pins the timer's output count on the head
// answer: config, the same for every reader.
func TestSessionView_TimerOutputs(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	h := th.head(t, watchKey)
	ct, ok := th.cfg.Load().Config.ChannelType("watch")
	require.True(t, ok)
	v := sessionView(ct, &h, th.clock.Now(), false)
	require.Len(t, v.Timers, 1)
	require.Equal(t, "no-containment", v.Timers[0].Name)
	require.Equal(t, 1, v.Timers[0].Outputs)
}

// flagRealtime is watchRealtime plus flag, a transition with no roles that
// a trigger posts, so a person and a trigger may both fire it.
func flagRealtime(t *testing.T) string {
	t.Helper()
	y := watchRealtime(breachBot)
	y = replaceOnce(t, y, "      wrapup:    {", "      flag:      { properties: { note: { type: string } }, post: [member] }\n      wrapup:    {")
	y = replaceOnce(t, y, "          after:\n            no-containment:", `            flag:
              to: escalated
              outputs:
                - set: { properties: { status: flagged } }
          after:
            no-containment:`)
	return y + `  flagger:
    entity_type: ticket
    on: enter
    when: "entity.status == 'flag'"
    channel: watch
    post: { type: flag, props: { note: "flag" } }
`
}

// TestOutputs_PersonOnTriggerReachableRetryWritesAsPoster pins that a
// person's post on a transition a trigger can also fire writes as the
// poster, and so does the runner's retry: never as run_as, never failed
// with run_as_changed.
func TestOutputs_PersonOnTriggerReachableRetryWritesAsPoster(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{errTransient}}
	h := newHarness(t, withYAML(flagRealtime(t)), withOutputs(w))
	res := h.postTo(t, user("alice"), watchAddr, "flag", props("note", "x"))
	require.Equal(t, OutputsPending, res.Outputs.Status)
	_, mark := h.lastMark(t)
	require.False(t, mark.Job)
	require.Equal(t, "alice", mark.Writer.User)

	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	require.Equal(t, 1, h.runOutputs(t, w).Ran)
	reqs := w.requests()
	require.Len(t, reqs, 2)
	for _, r := range reqs {
		require.Empty(t, r.RunAs)
	}
	require.NotNil(t, reqs[1].Retry)
	require.Equal(t, "alice", reqs[1].Retry.Writer.User)
	_, mark = h.lastMark(t)
	require.Equal(t, channels.OutputDone, mark.Results[0].Status)
}

// TestJobMark_WriterPrincipalType pins the principal type a run_as writer
// shows: integration for an integration: name, system for a system: one.
func TestJobMark_WriterPrincipalType(t *testing.T) {
	t.Parallel()
	for runAs, want := range map[string]string{"integration:watch-bot": "integration", breachBot: "system"} {
		h := newHarness(t, withYAML(watchRealtime(runAs)))
		th := &timerHarness{harness: h, tm: h.timers(t)}
		th.postTo(t, user("alice"), watchAddr, "text", props("body", "hi"))
		th.at(24 * time.Hour)
		th.round(t)
		_, mark := th.lastMark(t)
		require.Equal(t, runAs, mark.Writer.User)
		require.Equal(t, want, mark.Writer.PrincipalType, runAs)
	}
}

// TestOutputs_JobMarkOnPersonTransitionRemoved pins that a trigger's job
// mark whose transition a reload gave roles: is transition_removed: its
// outputs now follow the person rules, which run_as must not write.
func TestOutputs_JobMarkOnPersonTransitionRemoved(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	th.svcPostAsTrigger(t, "escalated", props("note", "escalated"))
	_, mark := th.lastMark(t)
	require.True(t, mark.Job)
	person := strings.Replace(watchRealtime(breachBot), "              by: trigger\n              outputs:",
		"              roles: [lead]\n              outputs:", 1)
	th.cfg.set(Snapshot{Config: testCompiled(t, person)})
	th.clock.Add(time.Second)
	w := &fakeOutputs{}
	require.Equal(t, 1, th.runOutputs(t, w).Ran)
	require.Empty(t, w.requests())
	_, mark = th.lastMark(t)
	require.Equal(t, []channels.OutputResult{{
		Kind: channels.OutputKindTransition, Status: channels.OutputFailed, Code: OutputCodeTransitionRemoved,
	}}, mark.Results)
}

// TestOutputs_LegacyRelaMarkRunsNoOutputs pins that a mark from before job
// marks, written by system:realtime and not a job mark, runs no outputs
// even when the transition now declares some: nothing writes as rela.
func TestOutputs_LegacyRelaMarkRunsNoOutputs(t *testing.T) {
	t.Parallel()
	th := newWatchHarness(t)
	now := th.clock.Now()
	_, err := th.store.Store.Append(context.Background(), watchKey, channels.NewMessage{
		Type: "escalated", Author: systemAuthor(), At: now, Props: map[string]any{"note": "escalated"},
		TriggeredBy: "trigger:old",
		Session: &channels.SessionStep{
			Decide: func(channels.Channel) (channels.SessionMove, bool) {
				return channels.SessionMove{From: "detected", To: "escalated"}, true
			},
			NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
			TriggeredBy: channels.SessionMarker + "watch",
			Due:         func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} },
			Mark: func(channels.SessionMove) *channels.Action {
				return &channels.Action{PendingAt: now, Outputs: &channels.OutputMark{
					Writer: systemAuthor(), AnchorType: "ticket",
				}}
			},
		},
	})
	require.NoError(t, err)
	_, mark := th.lastMark(t)
	require.False(t, mark.Job)
	th.clock.Add(time.Second)
	w := &fakeOutputs{}
	require.Equal(t, 1, th.runOutputs(t, w).Ran)
	require.Empty(t, w.requests(), "no output writes as system:realtime")
	n, mark := th.lastMark(t)
	require.Equal(t, channels.ActionDone, n.Action.Status)
	require.Empty(t, mark.Results)
}
