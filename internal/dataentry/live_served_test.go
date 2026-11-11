package dataentry

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// A client can fetch a response that no replay saw: its refetch after a 204
// or a frame lands after the replay that set the baseline. When the change
// it saw is reverted before the next round, that round replays the baseline
// again. Comparing against the baseline alone sends no frame, and the client
// keeps the reverted state (BUG-33Q36BI).

// collectPairs is a changed round's collect without the round generation.
func (h *liveHub) collectPairs(only *roundFilter) []*replayPair {
	pairs, _ := h.collect(true, only)
	return pairs
}

// keysAfterSteps runs n ticks and returns the live:changed keys c got by
// then. A round marks its frames before its tick finishes, so the slack
// covers delivery only.
func keysAfterSteps(t *testing.T, s *liveStepper, c *sseClient, n int) []string {
	t.Helper()
	s.step(t, n)
	var keys []string
	deadline := time.After(liveFrameSlack)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				return keys
			}
			if f.event == "live:changed" {
				keys = append(keys, frameData(t, f)["key"])
			}
		case <-deadline:
			return keys
		}
	}
}

// settleStream runs ticks until c is quiet, so a test starts from a stream with no
// frame owed.
func settleStream(t *testing.T, s *liveStepper, c *sseClient) {
	t.Helper()
	s.step(t, 2)
	liveDrain(c)
}

func TestLive_LeaseClaimedAndReleasedBetweenRoundsFrames(t *testing.T) {
	var clk *liveStepper
	env := newLeaseEnvWith(t, func(app *App) { clk = manualClock(app) })
	c, conn := env.open(t, "carol")
	env.subscribe(t, "carol", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	settleStream(t, clk, c)
	_, before := env.get(t, "carol", "/api/v1/tickets/TKT-1")

	claimed := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"ttl_seconds":30}`)
	// The client refetches and shows the claim.
	if code, seen := env.get(t, "carol", "/api/v1/tickets/TKT-1"); code != http.StatusOK || bytes.Equal(seen, before) {
		t.Fatalf("GET after the claim = %d, unchanged %v: the test needs carol to see the claim", code,
			bytes.Equal(seen, before))
	}
	if res, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(claimed.LeaseID),
		nil); res.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d %s", res.StatusCode, b)
	}

	if got := keysAfterSteps(t, clk, c, 3); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the release: changed keys = %v, want [detail]", got)
	}
	// The refetch hashes as the replay does, so a later round owes nothing.
	env.get(t, "carol", "/api/v1/tickets/TKT-1")
	update(t, env.app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "moved"}})
	if got := keysAfterSteps(t, clk, c, 3); len(got) != 0 {
		t.Fatalf("after the refetch: changed keys = %v, want none", got)
	}
}

func TestLive_WriteRevertedBetweenRoundsFrames(t *testing.T) {
	var clk *liveStepper
	env := newLiveEnv(t, func(app *App) {
		clk = manualClock(app)
		seedEntity(app, ticket1("one"))
		seedEntity(app, feature("FEAT-1", "here"))
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	settleStream(t, clk, c)

	update(t, env.app, ticket1("two"))
	if code, body := env.get(t, "alice", "/api/v1/tickets/TKT-1"); code != http.StatusOK ||
		!bytes.Contains(body, []byte(`"two"`)) {

		t.Fatalf("GET = %d %s, want the change", code, body)
	}
	update(t, env.app, ticket1("one"))

	if got := keysAfterSteps(t, clk, c, 3); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the change back: changed keys = %v, want [detail]", got)
	}
	// The frame was owed once: the next rounds find what the client fetched.
	env.get(t, "alice", "/api/v1/tickets/TKT-1")
	update(t, env.app, feature("FEAT-1", "elsewhere"))
	if got := keysAfterSteps(t, clk, c, 3); len(got) != 0 {
		t.Fatalf("after an unrelated write: changed keys = %v, want none", got)
	}
}

// What another principal fetched says nothing about what this client holds:
// bob's fetch of the new state does not stand in for alice's.
func TestLive_OtherPrincipalsFetchStillFrames(t *testing.T) {
	var clk *liveStepper
	env := newLiveEnv(t, func(app *App) {
		clk = manualClock(app)
		seedEntity(app, ticket1("one"))
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	_, bobConn := env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	settleStream(t, clk, c)

	update(t, env.app, ticket1("two"))
	env.get(t, "bob", "/api/v1/tickets/TKT-1")

	if got := keysAfterSteps(t, clk, c, 3); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("changed keys = %v, want [detail]: alice fetched nothing new", got)
	}
}

// A hidden write and its revert, with a fetch in between, send no frame:
// the fetch is the baseline, so no response the subscriber was served moved.
func TestLive_HiddenWriteRevertedBetweenRoundsSendsNoFrame(t *testing.T) {
	var clk *liveStepper
	env := newLiveEnv(t, func(app *App) {
		clk = manualClock(app)
		aclTicketsOnly(t)(app)
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"list", "/api/v1/tickets"}, liveSubEntry{"view", "/api/v1/_views/ticket/TKT-1"})
	settleStream(t, clk, c)

	update(t, env.app, feature("FEAT-1", "moved"))
	for _, path := range []string{"/api/v1/tickets/TKT-1", "/api/v1/tickets", "/api/v1/_views/ticket/TKT-1"} {
		env.get(t, "alice", path)
	}
	update(t, env.app, feature("FEAT-1", "hidden"))

	if got := keysAfterSteps(t, clk, c, 3); len(got) != 0 {
		t.Fatalf("changed keys = %v after a hidden write, want none", got)
	}
}

// onlySub returns user's one subscription with its conn and key.
func (h *liveHub) onlySub(t *testing.T, user string) subRef {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.conns {
		if c.owner.User != user {
			continue
		}
		c.mu.Lock()
		var ref subRef
		for key, s := range c.subs {
			ref = subRef{conn: c, key: key, sub: s}
		}
		c.mu.Unlock()
		if ref.sub != nil {
			return ref
		}
	}
	t.Fatalf("%s has no subscription", user)
	return subRef{}
}

// A record that lands after the round that replayed the reverted state
// committed still frames: it leaves the subscription pending, so the next
// tick replays it. The record is injected, since a real GET cannot be held
// past a commit here.
func TestLive_ServedAfterCommitFrames(t *testing.T) {
	var clk *liveStepper
	env := newLiveEnv(t, func(app *App) {
		clk = manualClock(app)
		seedEntity(app, ticket1("one"))
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	settleStream(t, clk, c)

	update(t, env.app, ticket1("two"))
	update(t, env.app, ticket1("one"))
	// The change back is not the response the client holds: `_updated`
	// moved. The client fetches it, and is then up to date.
	if got := keysAfterSteps(t, clk, c, 3); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the change back: changed keys = %v, want [detail]", got)
	}
	env.get(t, "alice", "/api/v1/tickets/TKT-1")
	if got := keysAfterSteps(t, clk, c, 3); len(got) != 0 {
		t.Fatalf("after the fetch: changed keys = %v, want none", got)
	}
	// The response the client fetched while the ticket said "two".
	env.app.live.noteServed([]subRef{env.app.live.onlySub(t, "alice")}, "200:two")
	if got := keysAfterSteps(t, clk, c, 1); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the late record: changed keys = %v, want [detail]", got)
	}
}

func TestLiveSub_ServedRecords(t *testing.T) {
	h := newLiveHub(nil)
	s := &liveSub{hash: "A"}

	h.noteServedLocked(s, "A")
	if s.served.hash != "" || len(h.pending) != 0 {
		t.Fatalf("a served baseline is recorded: %+v, pending %d", s.served, len(h.pending))
	}

	// A record made during a round waits for the next one.
	h.gen = 3
	h.noteServedLocked(s, "B")
	if _, ok := h.pending[s]; !ok || s.served.hash != "B" {
		t.Fatalf("served = %+v, pending %v; want B, pending", s.served, ok)
	}
	if s.servedMoved("A", 3) || s.served.hash != "B" {
		t.Fatalf("the round that collected before the record compared it: %+v", s.served)
	}
	if !s.servedMoved("A", 4) || s.served.hash != "" || s.served.streak != 1 {
		t.Fatalf("the next round did not frame and clear: %+v", s.served)
	}

	// The same transient state served again, as when a lease is claimed
	// and released twice, is recorded again.
	h.noteServedLocked(s, "B")
	if s.served.hash != "B" {
		t.Fatalf("a repeated state was not recorded: %+v", s.served)
	}
	// Two distinct records frame whatever the replay finds.
	h.noteServedLocked(s, "C")
	if !s.served.mixed || !s.servedMoved("B", 5) || s.served.streak != 2 {
		t.Fatalf("mixed records: %+v", s.served)
	}
	// A replay that matches what was served ends the streak.
	h.noteServedLocked(s, "B")
	if s.servedMoved("B", 6) || s.served.streak != 0 {
		t.Fatalf("a matching replay kept the streak: %+v", s.served)
	}
}

// A response a replay never matches frames liveServedStreak times, then no
// more until the baseline is served, so it cannot loop at tick rate.
func TestLiveSub_ServedStreakStopsLoop(t *testing.T) {
	h := newLiveHub(nil)
	s := &liveSub{hash: "A"}
	gen := uint64(0)
	frames := 0
	for range 2 * liveServedStreak {
		clear(h.pending)
		h.noteServedLocked(s, "X")
		gen++
		if s.servedMoved("A", gen) {
			frames++
		}
	}
	if frames != liveServedStreak || len(h.pending) != 0 {
		t.Fatalf("frames = %d, pending %d; want %d frames and none pending", frames, len(h.pending), liveServedStreak)
	}
	h.noteServedLocked(s, "A")
	h.noteServedLocked(s, "X")
	if s.served.hash != "X" {
		t.Fatalf("after the baseline was served, X was not recorded: %+v", s.served)
	}
}
