package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// hubWith returns a hub whose replays run handler.
func hubWith(handler http.Handler) *liveHub {
	h := newLiveHub(nil)
	h.replay.Store(&handler)
	return h
}

// seedSub adds a subscription with the baseline hash "old" to c.
func seedSub(c *liveConn, key, url string) {
	c.mu.Lock()
	c.subs[key] = &liveSub{url: url, hash: "old"}
	c.mu.Unlock()
}

func subHash(c *liveConn, key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs[key].hash
}

// Past the round cap, a subscription is not replayed: it is framed and keeps
// its baseline. The next round starts where the cap cut, so every
// subscription is replayed in turn.
func TestLiveHub_RoundCapFallsBack(t *testing.T) {
	var replays atomic.Int64
	h := hubWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		replays.Add(1)
		_, _ = w.Write([]byte("same"))
	}))
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	for i := range liveRoundCap + 1 {
		seedSub(c, fmt.Sprintf("k%03d", i), fmt.Sprintf("/api/v1/tickets/TKT-%03d", i))
	}

	h.round(context.Background(), true)

	if got := replays.Load(); got != liveRoundCap {
		t.Fatalf("replays = %d, want %d", got, liveRoundCap)
	}
	last := fmt.Sprintf("k%03d", liveRoundCap)
	if got := c.takeChanged(); len(got) != liveRoundCap+1 {
		// Every replayed one changed ("old" → hash of "same"), plus the
		// fallback.
		t.Fatalf("changed = %d keys, want %d", len(got), liveRoundCap+1)
	}
	if subHash(c, last) != "old" {
		t.Fatal("the subscription past the cap lost its baseline")
	}

	// The cut one is replayed now and frames against its kept baseline; the
	// cut moves to a subscription whose response did not move, which is
	// framed by the fallback alone.
	replays.Store(0)
	h.round(context.Background(), true)
	if got := replays.Load(); got != liveRoundCap {
		t.Fatalf("second round replays = %d, want %d", got, liveRoundCap)
	}
	if subHash(c, last) == "old" {
		t.Fatal("the rotated round did not replay the subscription past the cap")
	}
	got := c.takeChanged()
	if len(got) != 2 || !slices.Contains(got, last) {
		t.Fatalf("changed = %v, want %s and the newly cut one", got, last)
	}
}

// A panicking replay is recovered: the server keeps running and the
// subscription keeps its baseline.
func TestLiveHub_PanickingReplayRecovered(t *testing.T) {
	h := hubWith(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	seedSub(c, "k", "/api/v1/tickets")

	h.round(context.Background(), true)

	if subHash(c, "k") != "old" || len(c.takeChanged()) != 0 {
		t.Fatal("a panicking replay changed the baseline or framed")
	}
}

// The PUT takes the baseline before it answers, so a change after the 204 is
// compared against a response no newer than the client's refetch. A baseline
// the PUT could not take is taken by a round, with a frame.
func TestLiveHub_BaselineBeforeAnswer(t *testing.T) {
	var fail atomic.Bool
	h := hubWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			panic("unfinished")
		}
		_, _ = w.Write([]byte("body"))
	}))
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	put := func(key, url string) {
		t.Helper()
		c.mu.Lock()
		s := &liveSub{url: url, syncing: true}
		c.subs[key] = s
		c.mu.Unlock()
		h.baseline(context.Background(), c, []*liveSub{s})
	}

	put("taken", "/api/v1/tickets")
	if subHash(c, "taken") == "" {
		t.Fatal("the PUT did not take the baseline")
	}

	fail.Store(true)
	put("late", "/api/v1/features")
	if subHash(c, "late") != "" {
		t.Fatal("an unfinished baseline replay set a hash")
	}
	fail.Store(false)
	h.round(context.Background(), false)
	if got := c.takeChanged(); len(got) != 1 || got[0] != "late" {
		t.Fatalf("changed = %v, want [late]", got)
	}
}

// A round skips a subscription whose PUT is still taking its baseline.
func TestLiveHub_RoundSkipsSyncing(t *testing.T) {
	var replays atomic.Int64
	h := hubWith(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		replays.Add(1)
	}))
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	c.mu.Lock()
	c.subs["k"] = &liveSub{url: "/api/v1/tickets", syncing: true}
	c.mu.Unlock()

	h.round(context.Background(), true)
	if replays.Load() != 0 {
		t.Fatal("a round replayed a subscription the PUT is baselining")
	}
}

// A replay that does not finish keeps the old hash and sends no frame.
func TestLiveHub_UnfinishedReplayKeepsHash(t *testing.T) {
	h := hubWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("new"))
	}))
	c := h.register(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	seedSub(c, "k", "/api/v1/tickets")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.round(ctx, true)

	if subHash(c, "k") != "old" || len(c.takeChanged()) != 0 {
		t.Fatal("an unfinished replay changed the baseline or framed")
	}
}

// Response headers are not part of the hash; status and body are.
func TestReplayHash_StatusAndBodyOnly(t *testing.T) {
	p := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	respond := func(status int, header, body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Request-Id", header)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
	hash := func(h http.Handler) string {
		t.Helper()
		got, ok := replayHash(context.Background(), h, p, "/api/v1/tickets")
		if !ok {
			t.Fatal("replay did not finish")
		}
		return got
	}
	base := hash(respond(http.StatusOK, "a", "body"))
	if hash(respond(http.StatusOK, "b", "body")) != base {
		t.Error("a header change moved the hash")
	}
	if hash(respond(http.StatusNotFound, "a", "body")) == base {
		t.Error("a status change did not move the hash")
	}
	if hash(respond(http.StatusOK, "a", "other")) == base {
		t.Error("a body change did not move the hash")
	}
}

// Rounds and PUT baselines share one replay limit.
func TestLiveHub_ReplayLimitShared(t *testing.T) {
	var inFlight, peak atomic.Int64
	h := hubWith(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		inFlight.Add(-1)
	}))
	var wg sync.WaitGroup
	for u := range 3 {
		c := h.register(principal.Principal{User: fmt.Sprintf("u%d", u), Tool: principal.ToolDataEntry})
		seedSub(c, "seeded", "/api/v1/tickets")
		added := make([]*liveSub, 8)
		for i := range added {
			added[i] = &liveSub{url: fmt.Sprintf("/api/v1/tickets/TKT-%d", i), syncing: true}
		}
		wg.Go(func() { h.baseline(context.Background(), c, added) })
	}
	wg.Go(func() { h.round(context.Background(), true) })
	wg.Wait()
	if got := peak.Load(); got > liveParallel {
		t.Fatalf("peak replays in flight = %d, want at most %d", got, liveParallel)
	}
}
