package dataentry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Live views (TKT-HJ5Z9Y). A stream subscribes to GET resources; after a
// change the live worker replays each subscribed GET as the subscriber,
// through the same handlers and ACL middleware as a real request, and hashes
// the response. When the hash moves, the stream gets a live:changed frame
// naming the client's own key, and the client refetches.
//
// The frame reveals no more than polling the GET would: it is sent only when
// that principal's response changed. That holds only while a response is a
// pure function of the data the principal can read, which the determinism
// tests in live_test.go pin for every shape.

const (
	liveWindow         = 500 * time.Millisecond // round clock: changes within one tick share a round
	liveReplayTimeout  = 5 * time.Second
	liveParallel       = 4                // replays in flight at once, rounds and PUT baselines together
	liveRoundCap       = 256              // replays per round; the rest fall back to refetch
	liveRoundBudget    = 10 * time.Second // no replay starts later in a round; the rest fall back
	liveMaxPerConn     = 16
	liveMaxPerIdentity = 64
	liveMaxBody        = 16 << 10
)

// liveKeyPattern is the client label allowlist for a subscription key.
var liveKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_:.-]{1,64}$`)

// liveHub owns the live subscriptions of every stream and the worker that
// replays them. Streams only write frames. Replays run on the worker, where
// connections of one principal share a replay, and in a PUT taking the
// baseline of its new subscriptions; slots bounds both together.
type liveHub struct {
	meta   func() *metamodel.Metamodel
	window time.Duration // liveWindow; tests shorten it before start
	slots  chan struct{} // cap liveParallel: one per replay in flight

	noReplay sync.Once // logs a round that finds no replay handler, once

	// replay is the handler chain a replay runs through, installed by
	// NewRouter. Until then rounds do nothing.
	replay atomic.Pointer[http.Handler]

	mu    sync.Mutex
	conns map[string]*liveConn
	// pending holds the subscriptions a changed round could not replay: it
	// skipped them while their PUT took the baseline, or had no room or time
	// for them ([liveHub.fallback]). The next tick replays them, so a
	// change that round saw is compared against the baseline they end up
	// with. A round that selects them takes the set. Guarded by mu.
	pending map[*liveSub]struct{}

	kick chan struct{} // cap 1: a PUT asks for a baseline round

	// rotate is where the next over-cap round starts in the sorted pairs, so
	// the cap passes over every subscription in turn instead of always
	// dropping the same tail. Worker goroutine only.
	rotate int

	// ticks counts the ticks the worker has finished, the round of each
	// included. Tests wait on it instead of on the wall clock.
	ticks atomic.Uint64

	runMu  sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newLiveHub(meta func() *metamodel.Metamodel) *liveHub {
	return &liveHub{
		meta: meta, window: liveWindow, slots: make(chan struct{}, liveParallel),
		conns: make(map[string]*liveConn), kick: make(chan struct{}, 1),
	}
}

// liveConn is one stream's subscription set.
type liveConn struct {
	id    string
	owner principal.Principal // pre-resolution, as the JWT gate stamped it
	fp    string              // owner.ShareKey(): connections with equal keys share replays

	mu      sync.Mutex
	subs    map[string]*liveSub
	changed map[string]struct{}
	notify  chan struct{} // cap 1, never closed
}

// liveSub is one subscription. hash is the baseline: the PUT takes it
// before answering, so the client's refetch after the 204 is at least as new
// and any later change differs from it. It is empty only when that replay
// did not finish; the round that then takes it also sends a frame, because
// the client may have fetched a newer response than the baseline. syncing is
// set while the PUT takes the baseline, and rounds skip the subscription.
//
// A change can land after the PUT's replay read the data and before the
// baseline is stored. The client's refetch shows it, but the baseline does
// not, so a later change back to the baseline would send no frame. A changed
// round that skips a syncing subscription therefore sets recheck, and the
// PUT then leaves the subscription pending for the next tick's replay.
type liveSub struct {
	url     string // replay URL rebuilt by livePath.String
	hash    string
	syncing bool
	recheck bool
}

// register adds a stream owned by p and returns its connection.
func (h *liveHub) register(p principal.Principal) *liveConn {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// coverage-ignore: crypto/rand does not fail on supported platforms.
		panic(fmt.Sprintf("live: connection id: %v", err))
	}
	c := &liveConn{
		id:      base64.RawURLEncoding.EncodeToString(raw[:]),
		owner:   p,
		fp:      p.ShareKey(),
		subs:    make(map[string]*liveSub),
		changed: make(map[string]struct{}),
		notify:  make(chan struct{}, 1),
	}
	h.mu.Lock()
	h.conns[c.id] = c
	h.mu.Unlock()
	return c
}

// unregister removes c. Safe to call more than once.
func (h *liveHub) unregister(c *liveConn) {
	h.mu.Lock()
	delete(h.conns, c.id)
	h.mu.Unlock()
}

// takeChanged returns the keys whose response changed since the last call,
// sorted, and clears them.
// wake is signaled when a subscription changed; takeChanged reads which.
func (c *liveConn) wake() <-chan struct{} { return c.notify }

func (c *liveConn) takeChanged() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]string, 0, len(c.changed))
	for k := range c.changed {
		keys = append(keys, k)
	}
	clear(c.changed)
	slices.Sort(keys)
	return keys
}

// markChanged records key as changed and wakes the stream. Callers hold c.mu.
func (c *liveConn) markChanged(key string) {
	c.changed[key] = struct{}{}
	select {
	case c.notify <- struct{}{}:
	default:
	}
}

// --- PUT /api/v1/_events/{conn}/subscriptions ---

type liveSubscriptionRequest struct {
	Subscriptions []struct {
		Key  string `json:"key"`
		Path string `json:"path"`
	} `json:"subscriptions"`
}

// handlePut replaces a stream's subscription set.
func (h *liveHub) handlePut(w http.ResponseWriter, r *http.Request) {
	rest, _ := strings.CutPrefix(r.URL.Path, "/api/v1/_events/")
	connID, tail, _ := strings.Cut(rest, "/")
	if tail != "subscriptions" || connID == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use PUT", "")
		return
	}

	caller, ok := replayPrincipalFrom(r.Context())
	h.mu.Lock()
	c := h.conns[connID]
	h.mu.Unlock()
	if !ok || c == nil || !c.owner.SameIdentity(caller) {
		// An unknown id and another caller's id get the same answer.
		writeV1Error(w, r, http.StatusNotFound, "stream_not_found", "No such event stream", "")
		return
	}
	if !c.owner.Equal(caller) {
		writeV1Error(w, r, http.StatusConflict, "stream_principal_stale",
			"The event stream was opened with different credentials; reconnect it", "")
		return
	}

	var req liveSubscriptionRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, liveMaxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Invalid subscription body", err.Error())
		return
	}
	if len(req.Subscriptions) > liveMaxPerConn {
		writeV1Error(w, r, http.StatusBadRequest, "too_many_subscriptions",
			fmt.Sprintf("At most %d subscriptions per stream", liveMaxPerConn), "subscriptions")
		return
	}
	meta := h.meta()
	next := make(map[string]string, len(req.Subscriptions))
	for i, s := range req.Subscriptions {
		at := fmt.Sprintf("subscriptions[%d]", i)
		if !liveKeyPattern.MatchString(s.Key) {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_key",
				"A key is 1-64 characters of letters, digits and _ : . -", at+".key")
			return
		}
		if _, dup := next[s.Key]; dup {
			writeV1Error(w, r, http.StatusBadRequest, "duplicate_key", "Duplicate key "+s.Key, at+".key")
			return
		}
		lp, err := parseLivePath(s.Path, meta)
		if err != nil {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_path", err.Error(), at+".path")
			return
		}
		next[s.Key] = lp.String()
	}
	added, ok := h.replace(c, next)
	if !ok {
		writeV1Error(w, r, http.StatusTooManyRequests, "too_many_subscriptions",
			fmt.Sprintf("At most %d subscriptions across your streams", liveMaxPerIdentity), "subscriptions")
		return
	}

	// The replays must not inherit the PUT's context: it carries the PUT's
	// resolved ACL request and world, which a replay builds for itself. Only
	// its cancellation carries over.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer context.AfterFunc(r.Context(), cancel)()
	//nolint:contextcheck // detached on purpose, see above; cancelled with r
	if !h.baseline(ctx, c, added) {
		select {
		case h.kick <- struct{}{}:
		default:
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// replace sets c's subscriptions to next (key to replay URL) and returns the
// ones that need a baseline. It reports false, changing nothing, when c's
// identity would hold more than liveMaxPerIdentity. Unauthenticated callers
// all share one identity, so the cap does not apply to them.
func (h *liveHub) replace(c *liveConn, next map[string]string) (added []*liveSub, ok bool) {
	// h.mu is held from the count to the commit, so concurrent PUTs on one
	// identity's streams cannot each pass the cap against the others' old
	// sets.
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.owner.User != principal.Unknown && h.identityTotalLocked(c, len(next)) > liveMaxPerIdentity {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	subs := make(map[string]*liveSub, len(next))
	for key, u := range next {
		if old := c.subs[key]; old != nil && old.url == u {
			subs[key] = old // unchanged: keep its baseline
		} else {
			s := &liveSub{url: u, syncing: true}
			subs[key] = s
			added = append(added, s)
		}
	}
	c.subs = subs
	for key := range c.changed {
		if subs[key] == nil {
			delete(c.changed, key)
		}
	}
	return added, true
}

// baseline takes the baseline of each added subscription of c before the PUT
// answers. It reports false when a replay did not finish; that subscription
// is left for a round.
func (h *liveHub) baseline(ctx context.Context, c *liveConn, added []*liveSub) bool {
	hp := h.replay.Load()
	hashes := make([]string, len(added))
	oks := make([]bool, len(added))
	if hp != nil {
		var wg sync.WaitGroup
	acquire:
		for i, s := range added {
			select {
			case h.slots <- struct{}{}:
			case <-ctx.Done():
				break acquire // the rest stay unfinished, for a round
			}
			wg.Go(func() {
				defer func() { <-h.slots }()
				hashes[i], oks[i] = replayHash(ctx, *hp, c.owner, s.url)
			})
		}
		wg.Wait()
	}
	all := true
	h.mu.Lock()
	current := h.conns[c.id] == c
	c.mu.Lock()
	for i, s := range added {
		s.syncing = false
		if oks[i] {
			s.hash = hashes[i]
		} else {
			all = false
		}
		if s.recheck {
			s.recheck = false
			if current {
				h.addPendingLocked(s)
			}
		}
	}
	c.mu.Unlock()
	h.mu.Unlock()
	return all
}

// identityTotalLocked is how many subscriptions c's owner would hold across
// all streams if c held n. Callers hold h.mu.
func (h *liveHub) identityTotalLocked(c *liveConn, n int) int {
	total := n
	for _, other := range h.conns {
		if other == c || !other.owner.SameIdentity(c.owner) {
			continue
		}
		other.mu.Lock()
		total += len(other.subs)
		other.mu.Unlock()
	}
	return total
}

// --- worker ---

// leaseWaker is the lease wake-up source the live worker listens to: a
// claim, a status change, a release or an expiry may change a response's
// `_lease`. A renew that only extends the lease does not wake it. A renew
// that moves the expiry earlier does, and that round sends no frame:
// `_lease` has no expiry, so no response changes.
type leaseWaker interface {
	Subscribe() (wake <-chan struct{}, cancel func())
}

// start runs the live worker on feed, and on leases when it is non-nil,
// until stop. A second start before stop is a no-op.
func (h *liveHub) start(feed *changefeed.Bus, leases leaseWaker) {
	h.runMu.Lock()
	defer h.runMu.Unlock()
	if h.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel, h.done = cancel, make(chan struct{})
	sub := feed.Subscribe("")
	var leaseWake <-chan struct{} // nil: never ready
	unsubscribe := func() {}
	if leases != nil {
		leaseWake, unsubscribe = leases.Subscribe()
	}
	go func() {
		defer close(h.done)
		defer sub.Close()
		defer unsubscribe()
		h.run(ctx, sub, leaseWake)
	}()
}

// stop ends the worker and waits for it. Safe before start and repeated.
func (h *liveHub) stop() {
	h.runMu.Lock()
	cancel, done := h.cancel, h.done
	h.cancel = nil
	h.runMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// liveFeed is what the worker needs from its change-feed subscription.
type liveFeed interface {
	Wake() <-chan struct{}
	Take() changefeed.Batch
}

// run runs one round at a time: a changed round on each tick of a fixed
// clock when anything changed since the last one, a round over the pending
// subscriptions on a tick when nothing changed, and a baseline round when a
// PUT asks for one. A lease wake marks the round dirty like a write does.
//
// The clock is fixed so that a round's start never depends on when a change
// arrived. If the first change started the window instead, a write to a
// type the subscriber cannot read would move when its next frame arrives,
// which polling would not reveal. The same holds for a lease on an entity
// the subscriber cannot read.
func (h *liveHub) run(ctx context.Context, feed liveFeed, leaseWake <-chan struct{}) {
	tick := time.NewTicker(h.window)
	defer tick.Stop()
	dirty := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-feed.Wake():
			dirty = true
		case <-leaseWake:
			dirty = true
		case <-h.kick:
			h.round(ctx, false) // baselines only; nothing changed
		case <-tick.C:
			switch {
			case dirty:
				dirty = false
				feed.Take() // any change re-reads every subscription
				h.round(ctx, true)
			case h.hasPending():
				h.replayRound(ctx, true, true)
			}
			h.ticks.Add(1)
		}
	}
}

// replayPair is one distinct (principal, URL) replay and the subscriptions
// that share it.
type replayPair struct {
	owner principal.Principal
	url   string
	subs  []subRef
	hash  string
	ok    bool
}

type subRef struct {
	conn *liveConn
	key  string
}

// round replays the subscriptions and flags changed ones. When changed is
// false only subscriptions without a baseline are replayed.
func (h *liveHub) round(ctx context.Context, changed bool) {
	h.replayRound(ctx, changed, false)
}

// replayRound replays what collect(changed, pendingOnly) selects.
func (h *liveHub) replayRound(ctx context.Context, changed, pendingOnly bool) {
	hp := h.replay.Load()
	if hp == nil {
		h.noReplay.Do(func() {
			slog.Error("live: no replay handler installed; NewRouter installs it, so live views send no frames")
		})
		return
	}
	start := time.Now()
	pairs := h.collect(changed, pendingOnly)
	if len(pairs) == 0 {
		return
	}
	overflow := 0
	if len(pairs) > liveRoundCap {
		start := h.rotate % len(pairs)
		pairs = append(pairs[start:], pairs[:start]...)
		h.rotate = start + liveRoundCap
		overflow = len(pairs) - liveRoundCap
		h.fallback(pairs[liveRoundCap:])
		pairs = pairs[:liveRoundCap]
	}

	ctx, stats := store.WithQueryStats(ctx)
	var wg sync.WaitGroup
	started := len(pairs)
	for i, p := range pairs {
		h.slots <- struct{}{}
		if time.Since(start) > liveRoundBudget {
			<-h.slots
			started = i
			break
		}
		wg.Go(func() {
			defer func() { <-h.slots }()
			p.hash, p.ok = replayHash(ctx, *hp, p.owner, p.url)
		})
	}
	wg.Wait()
	if started < len(pairs) {
		overflow += len(pairs) - started
		h.fallback(pairs[started:])
		pairs = pairs[:started]
	}

	for _, p := range pairs {
		if !p.ok {
			continue
		}
		for _, ref := range p.subs {
			ref.conn.mu.Lock()
			s := ref.conn.subs[ref.key]
			if s != nil && s.url == p.url {
				if s.hash != p.hash {
					// A late baseline frames too: the client may hold a
					// newer response than the one it replaces.
					ref.conn.markChanged(ref.key)
				}
				s.hash = p.hash
			}
			ref.conn.mu.Unlock()
		}
	}
	slog.Debug("live: round", "replays", len(pairs), "overflow", overflow,
		"duration_ms", time.Since(start).Milliseconds(),
		"queries", stats.Queries(), "db_ms", stats.Duration().Milliseconds())
}

// collect groups the current subscriptions into distinct replays, in a
// stable order; round rotates where the cap cuts it. A changed round takes
// the pending set and marks the syncing subscriptions it skips for a
// recheck; pendingOnly keeps only the pending ones. A baseline round
// (changed false) saw no change, so it does neither.
func (h *liveHub) collect(changed, pendingOnly bool) []*replayPair {
	byKey := make(map[string]*replayPair)
	h.mu.Lock()
	var due map[*liveSub]struct{}
	if changed {
		due, h.pending = h.pending, nil
	}
	for _, c := range h.conns {
		c.mu.Lock()
		for key, s := range c.subs {
			if s.syncing {
				s.recheck = s.recheck || changed
				continue
			}
			if !changed && s.hash != "" {
				continue
			}
			if _, ok := due[s]; pendingOnly && !ok {
				continue
			}
			k := c.fp + " " + s.url
			p := byKey[k]
			if p == nil {
				p = &replayPair{owner: c.owner, url: s.url}
				byKey[k] = p
			}
			p.subs = append(p.subs, subRef{conn: c, key: key})
		}
		c.mu.Unlock()
	}
	h.mu.Unlock()
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	pairs := make([]*replayPair, len(keys))
	for i, k := range keys {
		pairs[i] = byKey[k]
	}
	return pairs
}

// fallback handles replays a round had no room or time for: each
// subscription is told it changed, so the client refetches. It keeps its
// baseline and goes into pending, so the next tick replays it and frames
// again if the response moved past that baseline. Without that replay a
// later change back to the baseline would send no frame. Pending shrinks by
// up to liveRoundCap replays a tick, and a replay whose result is unchanged
// frames nothing, so the set empties and the rounds stop.
func (h *liveHub) fallback(pairs []*replayPair) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range pairs {
		for _, ref := range p.subs {
			ref.conn.mu.Lock()
			if s := ref.conn.subs[ref.key]; s != nil && s.url == p.url {
				ref.conn.markChanged(ref.key)
				h.addPendingLocked(s)
			}
			ref.conn.mu.Unlock()
		}
	}
}

// addPendingLocked leaves s for the next tick's replay. Callers hold h.mu.
func (h *liveHub) addPendingLocked(s *liveSub) {
	if h.pending == nil {
		h.pending = make(map[*liveSub]struct{})
	}
	h.pending[s] = struct{}{}
}

// hasPending reports whether a subscription waits for a pending replay.
func (h *liveHub) hasPending() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pending) > 0
}

// replayHash runs a GET of url as p through handler and hashes the status
// and body. Headers are not hashed. It reports false when the replay did not
// finish (timeout, cancellation), so the caller keeps the old hash.
func replayHash(
	ctx context.Context, handler http.Handler, p principal.Principal, url string,
) (sum string, ok bool) {
	// net/http recovers a panicking handler per request; a replay runs on
	// the worker, where a panic would end the process.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("live: replay panicked", "url", url, "panic", r)
			sum, ok = "", false
		}
	}()
	ctx, cancel := context.WithTimeout(principal.With(ctx, p), liveReplayTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		slog.Warn("live: build replay request", "err", err)
		return "", false
	}
	hw := &hashWriter{h: sha256.New(), header: http.Header{}}
	handler.ServeHTTP(hw, req)
	if err := ctx.Err(); err != nil {
		slog.Warn("live: replay did not finish", "err", err)
		return "", false
	}
	status := hw.status
	if status == 0 {
		status = http.StatusOK
	}
	return fmt.Sprintf("%d:%x", status, hw.h.Sum(nil)), true
}

// hashWriter is a ResponseWriter that keeps only a hash of the body.
type hashWriter struct {
	h      hash.Hash
	header http.Header
	status int
}

func (w *hashWriter) Header() http.Header { return w.header }

func (w *hashWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *hashWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.h.Write(b)
}

// --- replay principal ---

type replayPrincipalKey struct{}

// stashReplayPrincipal saves the principal as the transport stamped it,
// before attachACLRequest resolves it, so a replay starts from the same
// identity a real request does and is resolved exactly once.
func stashReplayPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), replayPrincipalKey{}, principal.From(r.Context()))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// replayPrincipalFrom returns the principal saved by stashReplayPrincipal.
// It reports false when the middleware did not run: the context then holds
// only the resolved principal, and a replay would resolve it a second time.
func replayPrincipalFrom(ctx context.Context) (principal.Principal, bool) {
	p, ok := ctx.Value(replayPrincipalKey{}).(principal.Principal)
	return p.Clone(), ok
}
