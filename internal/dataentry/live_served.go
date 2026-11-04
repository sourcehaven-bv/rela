package dataentry

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"net/http"
)

// Served responses (BUG-33Q36BI). A subscription's baseline is the hash of
// the hub's last replay, but the client holds what it last fetched. Its
// refetch after a 204 or a frame runs later than that replay and can show a
// change that is reverted before the next tick. That tick's replay then
// equals the baseline, and without this record no frame would follow.
//
// recordServed hashes each GET the API serves to a subscribed principal on
// a subscribed URL, the same way a replay hashes. It wraps the API mux below
// the transport gates (host, origin), which a replay skips, so a response
// they refuse is never recorded. A hash that differs from the baseline is
// kept on the subscription, which goes into pending, so the next tick
// replays it and [liveHub.commitRound] frames when the replay differs from
// what was served. The frame tells the principal only that a response it was
// served differs from the one it would get now, which polling also tells.
// Its timing follows the principal's own GET: the replay is due on the next
// tick. The one exception is the pending rule of [liveHub.fallback]: a topic
// round that has no room for a subscription it took only because it was
// pending sends no frame, and a later tick replays it.
//
// A record goes to every subscription of the principal's share key on that
// URL, in every stream. All unauthenticated callers share one key, so one
// anonymous tab's fetch can frame another's; that costs a refetch and tells
// nothing a poll would not.

// liveServedStreak is how many frames in a row served records may cause
// while the baseline is never served; see [servedState].streak.
const liveServedStreak = 3

// servedState is what a subscription was served since its last committed
// replay, beyond its baseline. Guarded by the conn's mu.
type servedState struct {
	// hash is a served response's hash that differed from the baseline
	// when it was recorded; "" when none. mixed is set once two distinct
	// such hashes were recorded: a replay then differs from at least one,
	// so it frames whatever it finds.
	hash  string
	mixed bool
	// gen is the round generation ([liveHub.gen]) when the last hash was
	// recorded. A round compares only records made before it collected,
	// so a record that lands during a round waits for the next one.
	gen uint64
	// streak counts the frames served records caused in a row. A replay
	// that matches what was served, or a served baseline, resets it. At
	// liveServedStreak no more records are kept: a response a replay can
	// never match would otherwise frame, refetch and frame on every tick.
	streak int
}

func (h *liveHub) recordServed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refs := h.servedTargets(r)
		if len(refs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		sw := &servedWriter{ResponseWriter: w, h: sha256.New()}
		next.ServeHTTP(sw, r)
		// A 304 sends no body. The API's ETags derive from the content, so
		// the client's copy matches the data now and holds nothing a replay
		// would not. A failed write left the client without this body.
		if sw.status() == http.StatusNotModified || sw.failed {
			return
		}
		h.noteServed(refs, liveHash(sw.status(), sw.h))
	})
}

// servedTargets returns the subscriptions a GET of r serves: those of the
// stashed principal's share key on the replay URL r rebuilds to. A principal
// with no stream costs one look in byFP, under fpMu and not h.mu, so a GET
// never waits for a round's commit.
func (h *liveHub) servedTargets(r *http.Request) []subRef {
	if r.Method != http.MethodGet {
		return nil
	}
	p, ok := replayPrincipalFrom(r.Context())
	if !ok {
		return nil
	}
	fp := p.ShareKey()
	h.fpMu.Lock()
	conns := make([]*liveConn, 0, len(h.byFP[fp]))
	for c := range h.byFP[fp] {
		conns = append(conns, c)
	}
	h.fpMu.Unlock()
	if len(conns) == 0 {
		return nil
	}
	raw := r.URL.EscapedPath()
	if r.URL.RawQuery != "" {
		raw += "?" + r.URL.RawQuery
	}
	lp, err := parseLivePath(raw, h.meta())
	if err != nil {
		return nil
	}
	url := lp.String()
	var refs []subRef
	for _, c := range conns {
		c.mu.Lock()
		for key, s := range c.subs {
			if s.url == url {
				refs = append(refs, subRef{conn: c, key: key, sub: s})
			}
		}
		c.mu.Unlock()
	}
	return refs
}

// noteServed records hash as served to each of refs that is still current.
func (h *liveHub) noteServed(refs []subRef, hash string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ref := range refs {
		current := h.conns[ref.conn.id] == ref.conn
		ref.conn.mu.Lock()
		if s := ref.conn.subs[ref.key]; current && s == ref.sub {
			h.noteServedLocked(s, hash)
		}
		ref.conn.mu.Unlock()
	}
}

// noteServedLocked records hash as served to s. Callers hold h.mu and the
// conn's mu. While s is syncing its baseline is empty, so any record counts
// and costs one replay more than needed when it equals the baseline to come.
func (h *liveHub) noteServedLocked(s *liveSub, hash string) {
	sv := &s.served
	switch {
	case hash == s.hash:
		sv.streak = 0
		return
	case sv.streak >= liveServedStreak:
		return
	case sv.hash == "":
		sv.hash = hash
	case sv.hash != hash:
		sv.mixed = true
	}
	sv.gen = h.gen
	h.addPendingLocked(s)
}

// servedMoved reports whether a replay with hash, committed by the round of
// generation gen, differs from what s was served before that round
// collected, and clears those records. A record made later stays for the
// next round.
func (s *liveSub) servedMoved(hash string, gen uint64) bool {
	sv := &s.served
	if sv.hash == "" || sv.gen >= gen {
		return false
	}
	moved := sv.mixed || sv.hash != hash
	if moved {
		sv.streak++
	} else {
		sv.streak = 0
	}
	sv.hash, sv.mixed = "", false
	return moved
}

// servedWriter passes a response through and hashes its status and body as
// a replay's [hashWriter] does.
type servedWriter struct {
	http.ResponseWriter
	h      hash.Hash
	code   int
	failed bool // a write to the client failed
}

func (w *servedWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *servedWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	w.h.Write(b)
	n, err := w.ResponseWriter.Write(b)
	if err != nil {
		w.failed = true
	}
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *servedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *servedWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

// liveHash is the hash a replay and a served response both record: the
// status and the body's digest.
func liveHash(status int, body hash.Hash) string {
	return fmt.Sprintf("%d:%x", status, body.Sum(nil))
}
