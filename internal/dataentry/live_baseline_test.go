package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// A change that lands while a PUT takes its baseline must not be lost. The
// PUT's replay reads the old response; the change lands before the PUT
// stores it as the baseline, and a round skips the subscription because it
// is still syncing. The client's refetch after the 204 shows the change. If
// nothing replays the subscription after its baseline is stored, the
// baseline stays the old response, and a later change back to it sends no
// frame: the client keeps showing the intermediate state.

// liveFrameSlack covers delivery of a frame a round already marked: the
// stream writes it as soon as it wakes, so this is transport time only.
const liveFrameSlack = 50 * time.Millisecond

// hookBaseline makes the next replay of the live hub run as usual and then
// call during before it returns, so during runs inside the PUT's baseline
// window, after the replay has read the data. during runs off the test
// goroutine and must not fail the test itself.
func hookBaseline(env *liveEnv, during func()) {
	inner := *env.app.live.replay.Load()
	var fired atomic.Bool // not a sync.Once: during may run replays itself
	var wrapped http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
		if fired.CompareAndSwap(false, true) {
			during()
		}
	})
	env.app.live.replay.Store(&wrapped)
}

// ticksPassed waits until the live worker has finished n more ticks, the
// round of each included, and reports false when it did not in time. The
// first tick may have started before the call, so n=2 covers one whole
// tick.
func (e *liveEnv) ticksPassed(n uint64) bool {
	target := e.app.live.ticks.Load() + n
	deadline := time.Now().Add(2 * time.Second)
	for e.app.live.ticks.Load() < target {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
	return true
}

// liveKeysAfterTicks waits three ticks and returns the live:changed keys c
// got by then.
func liveKeysAfterTicks(t *testing.T, env *liveEnv, c *sseClient) []string {
	t.Helper()
	if !env.ticksPassed(3) {
		t.Fatal("the live worker did not tick")
	}
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

// putDuringBaseline PUTs subs as user while during runs inside the baseline
// window, followed by three ticks so a changed round sees the syncing
// subscription. It fails unless the PUT answered 204 and the ticks passed.
func putDuringBaseline(t *testing.T, env *liveEnv, user, conn string, during func() error, subs ...liveSubEntry) {
	t.Helper()
	var err error
	ticked := true
	hookBaseline(env, func() {
		err = during()
		ticked = env.ticksPassed(3)
	})
	if code, body := env.put(t, as(user), conn, subs...); code != http.StatusNoContent {
		t.Fatalf("PUT = %d %s", code, body)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !ticked {
		t.Fatal("the live worker did not tick during the baseline")
	}
}

func TestLive_WriteDuringBaselineIsKept(t *testing.T) {
	env := newLiveEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	c, conn := env.open(t, "alice")

	putDuringBaseline(t, env, "alice", conn, func() error {
		return env.app.store.UpdateEntity(t.Context(), ticket1("two"))
	}, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	// The client refetches now and shows "two". The response moved past the
	// baseline, so a frame for it is allowed but not required.
	liveKeysAfterTicks(t, env, c)

	update(t, env.app, ticket1("one"))
	if got := liveKeysAfterTicks(t, env, c); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the change back: changed keys = %v, want [detail]", got)
	}
}

// A write the subscriber cannot read, landing in the baseline window, sends
// no frame: the replay owed after the baseline finds the response unchanged.
func TestLive_HiddenWriteDuringBaselineSendsNoFrame(t *testing.T) {
	env := newLiveEnv(t, aclTicketsOnly(t))
	c, conn := env.open(t, "alice")

	putDuringBaseline(t, env, "alice", conn, func() error {
		return env.app.store.UpdateEntity(t.Context(), feature("FEAT-1", "moved"))
	}, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"}, liveSubEntry{"list", "/api/v1/tickets"},
		liveSubEntry{"view", "/api/v1/_views/ticket/TKT-1"})
	if got := liveKeysAfterTicks(t, env, c); len(got) != 0 {
		t.Fatalf("changed keys = %v after a hidden write, want none", got)
	}
}

func TestLive_LeaseDuringBaselineIsKept(t *testing.T) {
	env := newLeaseEnv(t)
	c, conn := env.open(t, "carol")

	var leaseID string
	putDuringBaseline(t, env.liveEnv, "carol", conn, func() error {
		res, b := env.do(t, "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_lease", `{"ttl_seconds":30}`, nil)
		if res.StatusCode != http.StatusCreated {
			return fmt.Errorf("claim = %d %s", res.StatusCode, b)
		}
		var cl struct {
			LeaseID string `json:"lease_id"`
		}
		if err := json.Unmarshal(b, &cl); err != nil {
			return err
		}
		leaseID = cl.LeaseID
		return nil
	}, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	// The client refetches now and shows the claim.
	liveKeysAfterTicks(t, env.liveEnv, c)

	if res, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(leaseID), nil); res.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d %s", res.StatusCode, b)
	}
	if got := liveKeysAfterTicks(t, env.liveEnv, c); len(got) != 1 || got[0] != "detail" {
		t.Fatalf("after the release: changed keys = %v, want [detail]", got)
	}
}

// The same holds for a presence round. An anonymous watcher never joins, so
// its own join cannot replay it: bob joins while its PUT takes the presence
// baseline, and his leave later must still frame it.
func TestPresence_JoinDuringBaselineIsKept(t *testing.T) {
	env, clk := newPresenceEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	path := presencePath("tickets", "TKT-1")
	anon, anonConn := env.open(t, "")
	_, bobConn := env.open(t, "bob")

	putDuringBaseline(t, env, "", anonConn, func() error {
		if code, body := env.put(t, as("bob"), bobConn, liveSubEntry{"p", path}); code != http.StatusNoContent {
			return fmt.Errorf("bob's PUT = %d %s", code, body)
		}
		return nil
	}, liveSubEntry{"p", path})
	// The watcher refetches now and sees bob.
	waitViewers(t, env, "", path, "bob")
	liveKeysAfterTicks(t, env, anon)

	if code, body := env.put(t, as("bob"), bobConn); code != http.StatusNoContent {
		t.Fatalf("bob's empty PUT = %d %s", code, body)
	}
	clk.advance(16 * time.Second) // past the grace
	if got := liveKeysAfterTicks(t, env, anon); len(got) != 1 || got[0] != "p" {
		t.Fatalf("after bob left: changed keys = %v, want [p]", got)
	}
	waitViewers(t, env, "", path)
}
