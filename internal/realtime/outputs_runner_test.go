package realtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

var outKey = channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "breach"}

var errTransient = errors.New("transient")

// pendingCreate posts contained with a writer whose first write fails
// transiently, which leaves the mark at seq 2 pending.
func pendingCreate(t *testing.T) (*harness, *fakeOutputs) {
	t.Helper()
	return pendingCreateAs(t, OutputPoster{})
}

// pendingCreateAs is pendingCreate for a poster with the identity facts p.
func pendingCreateAs(t *testing.T, p OutputPoster) (*harness, *fakeOutputs) {
	t.Helper()
	w := &fakeOutputs{errs: []error{errTransient}, poster: p}
	h := newOutputsHarness(t, w)
	res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	require.Equal(t, OutputsPending, res.Outputs.Status)
	return h, w
}

func (h *harness) mark(t *testing.T) *channels.OutputMark {
	t.Helper()
	return h.notice(t, 2).Action.Outputs
}

func (h *harness) runOutputs(t *testing.T, w OutputWriter) OutputsReport {
	t.Helper()
	rep, err := h.outputsRunner(t, w).Run(context.Background())
	require.NoError(t, err)
	return rep
}

func TestRunner_RetryAfterCrash(t *testing.T) {
	t.Parallel()
	// Two writes of the slowest kind fit in a claim: a holder that is
	// still writing is not taken over.
	require.GreaterOrEqual(t, realtimelimits.OutputClaimTTL, 2*(promoteTimeout+2*markTimeout))

	h, w := pendingCreate(t)
	h.clock.Add(60 * time.Second)
	// The holder writes progress at 60 s, so its claim runs to 150 s.
	cur := h.notice(t, 2)
	next := *cur.Action
	next.At, next.PendingAt = h.clock.Now(), h.clock.Now().Add(realtimelimits.OutputClaimTTL)
	next.Outputs = cur.Action.Outputs.Clone()
	next.Outputs.ClaimedAt = h.clock.Now()
	_, err := h.store.SetAction(context.Background(), outKey, 2, channels.OutputsAction, channels.ActionPending, next)
	require.NoError(t, err)

	h.clock.Add(60 * time.Second)
	require.Equal(t, 0, h.runOutputs(t, w).Ran, "a holder that writes progress is not taken over")
	require.Equal(t, 1, h.mark(t).Attempt)

	h.clock.Add(31 * time.Second)
	require.Equal(t, 1, h.runOutputs(t, w).Ran)
	m := h.mark(t)
	require.Equal(t, 2, m.Attempt)
	require.Equal(t, channels.ActionDone, h.notice(t, 2).Action.Status)
	require.Equal(t, channels.OutputDone, m.Results[0].Status)
	require.Len(t, w.requests(), 2)
}

func TestRunner_RetryAsPoster(t *testing.T) {
	t.Parallel()
	h, w := pendingCreate(t)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	h.runOutputs(t, w)
	reqs := w.requests()
	require.Nil(t, reqs[0].Retry, "the inline write runs as the request")
	require.Equal(t, &OutputIdentity{Writer: channels.Author{User: "alice", Tool: "data-entry"}}, reqs[1].Retry)
	require.Equal(t, "ticket", reqs[1].Target.AnchorType, "the mark keeps the anchor type")

	t.Run("identity facts", func(t *testing.T) {
		t.Parallel()
		// The poster's facts at the post travel in the mark, so the retry
		// is judged as the post was.
		facts := OutputPoster{Unmatched: true, UserEntity: true}
		h, w := pendingCreateAs(t, facts)
		h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
		h.runOutputs(t, w)
		require.Equal(t, facts, w.requests()[1].Retry.OutputPoster)
	})

	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		h, w := pendingCreate(t)
		h.clock.Add(realtimelimits.OutputRetryWindow + time.Second)
		h.runOutputs(t, w)
		require.Len(t, w.requests(), 1, "an expired mark is not run")
		require.Equal(t, []channels.OutputResult{{Kind: "create", Status: "failed", Code: OutputCodeExpired}},
			h.mark(t).Results)
	})
}

// danglingStore lists one pending outputs ref that names a message
// without a mark.
type danglingStore struct {
	*countingStore
}

func (s danglingStore) StalePending(ctx context.Context, now time.Time) ([]channels.PendingAction, error) {
	out, err := s.countingStore.StalePending(ctx, now)
	return append(out, channels.PendingAction{Key: outKey, Seq: 1, Name: channels.OutputsAction}), err
}

func TestRunner_DanglingRef(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.postTo(t, user("bob"), outCh, "text", props("body", "hi"))
	o, err := NewOutputs(OutputsDeps{
		Store: danglingStore{h.store}, Config: h.cfg, Writer: w, Local: h.local, Clock: h.clock.Now,
	})
	require.NoError(t, err)
	for range 2 {
		rep, err := o.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, 0, rep.Ran)
	}
	n := 0
	o.warned.Range(func(any, any) bool { n++; return true })
	require.Equal(t, 1, n, "logged once")
	require.Empty(t, w.requests())
}

func TestRunner_TransientRetries(t *testing.T) {
	t.Parallel()
	conflict := &OutputError{Code: OutputCodeConflict, Retryable: true}
	w := &fakeOutputs{errs: []error{errTransient, conflict, errTransient, errTransient, errTransient, errTransient}}
	h := newOutputsHarness(t, w)
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	for range realtimelimits.MaxOutputAttempts + 2 {
		h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
		h.runOutputs(t, w)
	}
	require.Len(t, w.requests(), realtimelimits.MaxOutputAttempts)
	require.Equal(t, channels.ActionDone, h.notice(t, 2).Action.Status)
	require.Equal(t, []channels.OutputResult{{Kind: "create", Status: "failed", Code: OutputCodeError}}, h.mark(t).Results)
}

func TestRunner_TransitionRemoved(t *testing.T) {
	t.Parallel()
	h, w := pendingCreate(t)
	yaml := replaceOnce(t, outputsRealtime("status: x"), "            contained:\n              to: contained\n",
		"            contained-gone:\n              to: contained\n")
	yaml = replaceOnce(t, yaml, "      contained: {", "      contained-gone: {")
	yaml = replaceOnce(t, yaml, "      text: ", "      contained: { properties: { note: { type: markdown } }, post: [member] }\n      text: ")
	h.cfg.set(Snapshot{Config: testCompiled(t, yaml)})
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	h.runOutputs(t, w)
	require.Len(t, w.requests(), 1)
	require.Equal(t, []channels.OutputResult{{Kind: "transition", Status: "failed", Code: OutputCodeTransitionRemoved}},
		h.mark(t).Results)
}

func TestRunner_Degraded(t *testing.T) {
	t.Parallel()
	h, w := pendingCreate(t)
	snap := h.cfg.Load()
	snap.Degraded = true
	h.cfg.set(snap)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	require.Equal(t, 0, h.runOutputs(t, w).Ran)
	snap.Degraded = false
	h.cfg.set(snap)
	require.Equal(t, 1, h.runOutputs(t, w).Ran)
	require.Equal(t, channels.ActionDone, h.notice(t, 2).Action.Status)
}

func TestRunner_NoJoinCreateNotRepeated(t *testing.T) {
	t.Parallel()
	t.Run("crash after the started record", func(t *testing.T) {
		t.Parallel()
		h, w := pendingCreate(t)
		// A holder recorded the start of its create and stopped.
		cur := h.notice(t, 2)
		next := *cur.Action
		next.At = h.clock.Now()
		next.Outputs = cur.Action.Outputs.Clone()
		next.Outputs.Results = []channels.OutputResult{{Kind: "create", Status: channels.OutputStarted}}
		_, err := h.store.SetAction(context.Background(), outKey, 2, channels.OutputsAction, channels.ActionPending, next)
		require.NoError(t, err)
		h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
		h.runOutputs(t, w)
		require.Len(t, w.requests(), 1, "the create is not repeated")
		require.Equal(t, []channels.OutputResult{{Kind: "create", Status: "failed", Code: OutputCodeUnconfirmed}},
			h.mark(t).Results)
	})
	t.Run("error after the start", func(t *testing.T) {
		t.Parallel()
		w := &fakeOutputs{}
		w.write = func(r OutputRequest) { require.NoError(t, r.Started(context.Background())) }
		w.errs = []error{errTransient}
		h := newOutputsHarness(t, w)
		res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
		require.Equal(t, OutputsFailed, res.Outputs.Status)
		require.Equal(t, "unconfirmed", res.Outputs.Results[0].Code)
	})
	t.Run("wrote nothing after the start", func(t *testing.T) {
		t.Parallel()
		w := &fakeOutputs{}
		w.write = func(r OutputRequest) {
			if r.Retry == nil {
				require.NoError(t, r.Started(context.Background()))
			}
		}
		w.errs = []error{&OutputError{Code: OutputCodeError, Retryable: true, WroteNothing: true}}
		h := newOutputsHarness(t, w)
		res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
		require.Equal(t, OutputsPending, res.Outputs.Status)
		require.Empty(t, h.mark(t).Results, "the started record is dropped")
		h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
		h.runOutputs(t, w)
		require.Equal(t, channels.OutputDone, h.mark(t).Results[0].Status)
	})
	t.Run("started record not written", func(t *testing.T) {
		t.Parallel()
		// The started record failed to store, so the writer created
		// nothing: the create is retried, not reported unconfirmed.
		w := &fakeOutputs{}
		h := newOutputsHarness(t, w)
		w.write = func(r OutputRequest) {
			if r.Retry == nil {
				h.store.failSetAction.Store(true)
			}
		}
		res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
		require.Equal(t, OutputsPending, res.Outputs.Status)
		require.Empty(t, h.mark(t).Results)
		h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
		h.runOutputs(t, w)
		require.Equal(t, channels.OutputDone, h.mark(t).Results[0].Status)
	})
	t.Run("joined create", func(t *testing.T) {
		t.Parallel()
		w := &fakeOutputs{}
		h := newOutputsHarness(t, w)
		w.join = h.store
		res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
		require.Equal(t, OutputsDone, res.Outputs.Status)
		require.Equal(t, "DOC-101", h.mark(t).Results[0].Entity.ID)
	})
}

func TestRunner_LeaderOverlap(t *testing.T) {
	t.Parallel()
	h, w := pendingCreate(t)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	a, b := h.outputsRunner(t, w), h.outputsRunner(t, w)
	var wg sync.WaitGroup
	for _, o := range []*Outputs{a, b} {
		wg.Go(func() {
			_, err := o.Run(context.Background())
			require.NoError(t, err)
		})
	}
	wg.Wait()
	require.Len(t, w.requests(), 2, "the inline write and one retry")
	require.Equal(t, 2, h.mark(t).Attempt)
}

// manyPending posts contained on n tickets, a second apart, with a writer
// that always fails transiently; the marks stay pending.
func manyPending(t *testing.T, n int) (*harness, *fakeOutputs) {
	t.Helper()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	for i := range n {
		id := fmt.Sprintf("TKT-%d", 100+i)
		h.anchors.views[id] = AnchorView{ID: id, Type: "ticket", Props: map[string]any{"owner": "alice"}}
		w.errs = append(w.errs, errTransient)
		h.postTo(t, outLead(), RawAddress{EntityType: "ticket", Anchor: id, Type: "breach"}, "contained",
			props("note", "x"))
		h.clock.Add(time.Second)
	}
	return h, w
}

func TestRunner_Cap(t *testing.T) {
	t.Parallel()
	h, w := manyPending(t, 40)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	rep := h.runOutputs(t, w)
	require.Equal(t, OutputsReport{Ran: realtimelimits.MaxOutputRunsPerRound, More: true}, rep)
	reqs := w.requests()[40:]
	require.Len(t, reqs, realtimelimits.MaxOutputRunsPerRound)
	for i, r := range reqs {
		require.Equal(t, fmt.Sprintf("session:TKT-%d/breach/detected->contained#1/0", 100+i), r.Origin, "oldest first")
	}
}

func TestRunner_Budget(t *testing.T) {
	t.Parallel()
	h, w := manyPending(t, 100)
	// The first 16 claims ran out; the rest are held.
	h.clock.Add(realtimelimits.OutputClaimTTL - 100*time.Second + 16*time.Second)
	for range 16 {
		w.errs = append(w.errs, errTransient)
	}
	h.store.staleLists.Store(0)
	h.store.messages.Store(0)
	h.store.setActions.Store(0)
	rep := h.runOutputs(t, w)
	require.Equal(t, 16, rep.Ran)
	require.Equal(t, int64(1), h.store.staleLists.Load())
	require.LessOrEqual(t, h.store.messages.Load(), int64(16), "one page read per mark")
	require.LessOrEqual(t, h.store.setActions.Load(), int64(16), "one claim per mark; the writes failed")
}
