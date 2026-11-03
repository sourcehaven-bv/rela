package realtime

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// TestService_ConcurrentPostReadReload runs posts, head reads, Takes and
// configuration reloads in parallel, for the race detector. A post either
// lands or is refused because the configuration changed under it.
func TestService_ConcurrentPostReadReload(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()
	configs := make([]*realtimecfg.Compiled, 4)
	for i := range configs {
		configs[i] = testCompiled(t, testRealtime)
	}
	var landed atomic.Int64
	var wg sync.WaitGroup
	for w := range 6 {
		wg.Go(func() {
			who := user(fmt.Sprintf("u%d", w))
			for i := range 15 {
				in := text("x")
				if i%2 == 1 {
					in = typing()
				}
				_, err := h.svc.Post(as(who), room, in)
				if err == nil {
					landed.Add(1)
					continue
				}
				d, ok := errors.AsType[*DenialError](err)
				require.True(t, ok, "want a denial, got %v", err)
				require.Contains(t, []string{TypeConfigChanged, TypeRateLimited}, d.Type)
			}
		})
		wg.Go(func() {
			for range 15 {
				_, err := h.svc.Head(as(user("bob")), room)
				require.NoError(t, err)
			}
		})
	}
	wg.Go(func() {
		for i := range 40 {
			h.cfg.set(Snapshot{Config: configs[i%len(configs)]})
		}
	})
	wg.Go(func() {
		for range 40 {
			sub.Take()
		}
	})
	wg.Wait()
	require.Positive(t, landed.Load())
}

// TestCreate_ConcurrentRespectsCap pins that parallel creates on one
// anchor never pass channels_per_entity, and that channels of another
// type do not count against it.
func TestCreate_ConcurrentRespectsCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), text("the room channel is another type"))
	var ok, refused atomic.Int64
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			_, err := h.svc.Create(as(user(fmt.Sprintf("u%d", i), "chat.read")), "ticket", "TKT-1", "topics",
				CreateInput{Title: "t"})
			if err == nil {
				ok.Add(1)
				return
			}
			requireDenial(t, err, http.StatusConflict, TypeTooManyChannels)
			refused.Add(1)
		})
	}
	wg.Wait()
	require.EqualValues(t, 3, ok.Load())
	require.EqualValues(t, 7, refused.Load())
}

// TestStoreError_NotFound pins that a store miss after the operation read
// the channel or message is a 404, not a 500.
func TestStoreError_NotFound(t *testing.T) {
	t.Parallel()
	miss := fmt.Errorf("gone: %w", channels.ErrNotFound)
	requireDenial(t, storeError(miss), http.StatusNotFound, TypeChannelNotFound)
	requireDenial(t, messageStoreError(miss), http.StatusNotFound, TypeMessageNotFound)
	requireDenial(t, messageStoreError(channels.ErrRemoved), http.StatusConflict, TypeRemoved)
	boom := errors.New("boom")
	require.ErrorIs(t, messageStoreError(boom), boom)
}

// TestAuthz_OffBeforeAccess pins that realtime being off is answered
// before the caller's access is evaluated, so an unauthenticated caller
// learns the feature is off, and that an access failure other than a
// missing principal is a wrapped error, not a denial.
func TestAuthz_OffBeforeAccess(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	boom := errors.New("acl unavailable")
	_, err := h.svc.Head(failingAccess(boom), room)
	require.ErrorIs(t, err, boom)
	_, isDenial := errors.AsType[*DenialError](err)
	require.False(t, isDenial)
	_, err = h.svc.Head(failingAccess(fmt.Errorf("%w: unstamped", ErrNoPrincipal)), room)
	requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)

	h.cfg.set(Snapshot{})
	_, err = h.svc.Head(t.Context(), room)
	require.ErrorIs(t, err, ErrRealtimeOff)
	_, err = h.svc.Head(failingAccess(boom), room)
	require.ErrorIs(t, err, ErrRealtimeOff)
}

// TestOps_ModeratorOwnDeleteKeepsReason pins that a moderator who gives a
// reason when deleting their own message removes it as a moderator, so the
// reason is kept.
func TestOps_ModeratorOwnDeleteKeepsReason(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mod := user("mo", "chat.moderate")
	m := h.post(t, mod, text("x"))
	got, err := h.svc.Delete(as(mod), room, m.Seq, "posted in the wrong room")
	require.NoError(t, err)
	require.Equal(t, channels.RemovalModerator, got.Removed.Kind)
	require.Equal(t, "posted in the wrong room", got.Removed.Reason)

	m = h.post(t, mod, text("y"))
	got, err = h.svc.Delete(as(mod), room, m.Seq, "")
	require.NoError(t, err)
	require.Equal(t, channels.RemovalAuthor, got.Removed.Kind, "without a reason it is the author's delete")
}

// TestPost_MessageInConcurrent pins AC 5: approve and revise posted at
// once from plan-review move the session once. The other post is refused
// 409 session_wrong_state, by the pre-check or by the store, with a title
// that names the state the winner moved to.
func TestPost_MessageInConcurrent(t *testing.T) {
	t.Parallel()
	for i := range 8 {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			h := newPocHarness(t)
			h.pocDriveTo(t, "plan-review")
			to := map[string]string{"approve": "working", "revise": "planning"}
			errs := map[string]error{}
			var mu sync.Mutex
			var wg sync.WaitGroup
			for typ := range to {
				wg.Go(func() {
					_, err := h.svc.Post(as(pocOwner()), pocRun, PostInput{Type: typ, Props: map[string]any{"note": "n"}})
					mu.Lock()
					errs[typ] = err
					mu.Unlock()
				})
			}
			wg.Wait()
			var winner, loser string
			for typ, err := range errs {
				if err == nil {
					winner = typ
				} else {
					loser = typ
				}
			}
			require.NotEmpty(t, winner, "neither post landed: %v", errs)
			require.NotEmpty(t, loser, "both posts landed")
			d := requireDenial(t, errs[loser], http.StatusConflict, TypeSessionWrongState)
			require.Equal(t, fmt.Sprintf("%s may be posted only while the session is in plan-review; it is in %s",
				loser, to[winner]), d.Title)
			v, err := h.svc.Head(as(pocOwner()), pocRun)
			require.NoError(t, err)
			require.Equal(t, to[winner], v.Channel.Session.State)
			moves := 0
			for _, r := range h.sessionAudit(t) {
				if r.From == "plan-review" {
					moves++
				}
			}
			require.Equal(t, 1, moves, "the session moved out of plan-review more than once")
		})
	}
}

// TestSessionBranches_ConcurrentDecisions pins TKT-FPXTQJ: two decide posts
// that expect the same generation but take different entries; exactly one
// moves the session and the other is refused with session_moved.
func TestSessionBranches_ConcurrentDecisions(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(branchRealtime))
	var moved, refused atomic.Int64
	var wg sync.WaitGroup
	for _, d := range []string{"approve", "revise"} {
		wg.Go(func() {
			_, err := h.svc.Post(as(user("bob")), reviewCh, PostInput{
				Type: "decide", Props: map[string]any{"decision": d}, ExpectGeneration: gen(0),
			})
			if err == nil {
				moved.Add(1)
				return
			}
			var de *DenialError
			if errors.As(err, &de) && de.Status == http.StatusConflict && de.Type == TypeSessionMoved {
				refused.Add(1)
				return
			}
			t.Errorf("unexpected error: %v", err)
		})
	}
	wg.Wait()
	require.Equal(t, int64(1), moved.Load())
	require.Equal(t, int64(1), refused.Load())
	s := h.reviewSession(t)
	require.Equal(t, int64(1), s.Generation)
	require.Contains(t, []string{"working", "planning"}, s.State)
}
