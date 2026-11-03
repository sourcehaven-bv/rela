package realtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// pendingAssessed drives the breach channel to assessed with the assessed
// set (status) left pending at seq 4.
func pendingAssessed(t *testing.T) (*harness, *fakeOutputs) {
	t.Helper()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w, "status: assessed")
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	w.errs = []error{errors.New("transient")}
	h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	require.Equal(t, channels.ActionPending, h.notice(t, 4).Action.Status)
	return h, w
}

// TestOutputs_SupersedeReadUnderLock pins that a run reads the later
// moves under the channel's outputs lock: a retry of an older set that
// waits behind a newer move's run sees the newer value and does not
// overwrite it.
func TestOutputs_SupersedeReadUnderLock(t *testing.T) {
	t.Parallel()
	h, w := pendingAssessed(t)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)

	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	w.mu.Lock()
	w.write = func(r OutputRequest) {
		if strings.Contains(r.Origin, "assessed->notified") {
			once.Do(func() {
				close(started)
				<-release
			})
		}
	}
	w.mu.Unlock()

	var wg sync.WaitGroup
	wg.Go(func() { h.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1")) })
	<-started // the newer move's run holds the lock, mid-write
	wg.Go(func() {
		_, err := h.outputsRunner(t, w).Run(context.Background())
		require.NoError(t, err)
	})
	// Give the runner time to reach the lock; it must not read the later
	// moves before the newer run is done.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	require.Equal(t, channels.OutputSuperseded, h.notice(t, 4).Action.Outputs.Results[0].Status,
		"the older set sees the newer move's set")
	for _, r := range w.requests() {
		if r.Retry != nil {
			require.NotEqual(t, "assessed", r.Props["status"], "the retry never writes the older value")
		}
	}
}

// TestOutputs_SupersedeBeyondFirstPage pins that a retry reads every later
// move, not only the first page after its notice: in a busy channel a
// newer set far down the log still supersedes it.
func TestOutputs_SupersedeBeyondFirstPage(t *testing.T) {
	t.Parallel()
	h, w := pendingAssessed(t)
	for i := range 2 * supersedePage {
		_, err := h.store.Append(context.Background(), outKey, channels.NewMessage{
			Type: "text", Author: channels.Author{User: "alice", Tool: "data-entry"},
			At: h.clock.Now().Add(time.Duration(i) * time.Millisecond), Props: map[string]any{"body": "busy"},
		})
		require.NoError(t, err)
	}
	h.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1"))
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	rep := h.runOutputs(t, w)
	require.Equal(t, 1, rep.Ran)
	require.Equal(t, channels.OutputSuperseded, h.notice(t, 4).Action.Outputs.Results[0].Status)
}

// TestOutputs_CloseDetachedFromClient pins that the on_final close does
// not fail when the client has gone: it runs on its own context.
func TestOutputs_CloseDetachedFromClient(t *testing.T) {
	t.Parallel()
	h, _ := pendingAssessed(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := h.svc.outputs.close(ctx, &markRun{key: outKey})
	require.Equal(t, channels.OutputResult{Kind: channels.OutputKindClose, Status: channels.OutputDone}, res)
	head, err := h.store.Head(context.Background(), outKey)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, head.State)
}

// TestOutputs_HoldInline pins the inline run's claim check: a claim
// another runner took is left to it, and a claim near the end of its TTL
// is renewed before the run.
func TestOutputs_HoldInline(t *testing.T) {
	t.Parallel()
	h, _ := pendingCreate(t)
	e := &h.svc.outputs
	r, ok, err := e.load(context.Background(), outKey, 2)
	require.NoError(t, err)
	require.True(t, ok)
	claim := r.notice.Action.Outputs.Claim

	require.False(t, e.holdInline(context.Background(), &r, "another"), "a claim taken by another is left")
	require.True(t, e.holdInline(context.Background(), &r, claim), "a fresh claim holds")

	h.clock.Add(realtimelimits.OutputClaimTTL/2 + time.Second)
	require.True(t, e.holdInline(context.Background(), &r, claim))
	m := h.mark(t)
	require.Equal(t, claim, m.Claim)
	require.Equal(t, 1, m.Attempt)
	require.True(t, m.ClaimedAt.Equal(h.clock.Now()), "the claim is renewed")
}

// TestLocal_OutputLocksPerChannel pins that the outputs lock of one
// channel never blocks another, and that an idle lock is dropped.
func TestLocal_OutputLocksPerChannel(t *testing.T) {
	t.Parallel()
	l := &Local{}
	a := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "breach"}
	b := channels.Key{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "breach"}
	unlockA := l.lockOutputs(a)

	done := make(chan struct{})
	go func() {
		l.lockOutputs(b)()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the lock of another channel blocked")
	}

	waited := make(chan struct{})
	go func() {
		l.lockOutputs(a)()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("the same channel's lock did not wait")
	case <-time.After(20 * time.Millisecond):
	}
	unlockA()
	<-waited
	l.outputLocks.mu.Lock()
	defer l.outputLocks.mu.Unlock()
	require.Empty(t, l.outputLocks.locks, "idle locks are dropped")
}

// TestPersonMark_DeciderRerun pins that the plan's marked flag follows the
// last run of the decider: a backend may run it again on a later head,
// where the move has no outputs.
func TestPersonMark_DeciderRerun(t *testing.T) {
	t.Parallel()
	ct, ok := testCompiled(t, outputsRealtime("status: x")).ChannelType("breach")
	require.True(t, ok)
	plan := &outputPlan{}
	mark := personMark(ct.Session(), "contained", channels.Author{User: "alice"}, nil, time.Now(), plan)

	require.NotNil(t, mark(channels.SessionMove{From: "detected", To: "contained"}))
	require.True(t, plan.marked, "contained from detected has a create")
	require.Nil(t, mark(channels.SessionMove{From: "contained", To: "contained"}))
	require.False(t, plan.marked, "a rerun on a head where the move has no outputs clears the flag")
}

// TestSessionBranches_GateRefusesOtherEntry pins TKT-FPXTQJ AC 9: when a
// reminder count switches the entry between the pre-check and the lock,
// the gate stops a move along another entry with outputs; a switch to an
// entry without outputs still moves.
func TestSessionBranches_GateRefusesOtherEntry(t *testing.T) {
	t.Parallel()
	at := func(count int64) channels.Channel {
		return channels.Channel{State: channels.StateOpen, Session: channels.Session{
			State: "plan-review", Reminders: map[string]channels.ReminderRecord{"nudge": {Count: count}},
		}}
	}
	cases := []struct {
		name, goB string
		moves     bool
	}{
		{"other entry with outputs", "{ to: planning, outputs: [ { set: { properties: { status: late } } } ] }", false},
		{"other entry without outputs", "{ to: planning }", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ct, _ := testCompiled(t, branchOutputsRealtime(tc.goB)).ChannelType("review")
			gate := &outputGate{}
			decide := sessionDecider(context.Background(), ct.Session(), ct.MessageTypes["go"], props(), time.Now(),
				poster{roles: map[string]bool{}}, &guardFailure{}, gate)

			// The pre-check: count 1, so entry A (to working) applies.
			move, ok := decide(at(1))
			require.True(t, ok)
			require.Equal(t, "working", move.To)
			gate.active, gate.from, gate.to = true, move.From, move.To

			// Under the lock the count is 2, so entry B applies first.
			move, ok = decide(at(2))
			require.Equal(t, tc.moves, ok)
			if tc.moves {
				require.Equal(t, "planning", move.To)
			}
		})
	}
}

// TestSessionBranches_GateThroughPost pins TKT-FPXTQJ AC 9 through
// Service.Post: the pre-check reads nudge's count as 1 and checks entry
// A's outputs; a reminder fires before the append, so under the lock entry
// B applies. B with outputs is refused by the gate: the message is stored
// without a notice, an outputs mark or a write. B without outputs moves.
func TestSessionBranches_GateThroughPost(t *testing.T) {
	t.Parallel()
	review := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "review"}
	key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "review"}
	cases := []struct {
		name, goB, state string
		gen              int64
	}{
		{"other entry with outputs", "{ to: planning, outputs: [ { set: { properties: { status: late } } } ] }",
			"plan-review", 0},
		{"other entry without outputs", "{ to: planning }", "planning", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &fakeOutputs{}
			h := newHarness(t, withYAML(branchOutputsRealtime(tc.goB)), withOutputs(w))
			th := &timerHarness{harness: h, tm: h.timers(t)}
			th.postTo(t, user("bob"), review, "text", props("body", "hi"))
			th.at(time.Hour + time.Second)
			th.round(t)
			require.Equal(t, int64(1), th.head(t, key).Session.Reminders["nudge"].Count)
			h.store.beforeAppend = func() {
				th.at(2*time.Hour + 2*time.Second)
				th.round(t)
				require.Equal(t, int64(2), th.head(t, key).Session.Reminders["nudge"].Count)
			}
			res := h.postTo(t, user("bob"), review, "go", nil)
			require.Equal(t, tc.state, res.Session.State)
			require.Equal(t, tc.gen, res.Session.Generation)
			require.Nil(t, res.Outputs)
			require.Len(t, w.checks, 1, "the pre-check asked about entry A's outputs only")
			require.Empty(t, w.requests(), "nothing is written")
			last := h.readAll(t, user("bob"), review).Messages
			if tc.gen == 0 {
				require.Equal(t, "go", last[len(last)-1].Type, "the post is stored with no notice after it")
			} else {
				require.True(t, last[len(last)-1].IsSessionNotice())
			}
		})
	}
}
