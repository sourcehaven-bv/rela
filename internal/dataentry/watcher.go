package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// storeWatcher is the optional capability dataentry needs from a
// store to drive live-reload of entity/relation files. fsstore
// satisfies it; in-memory backends don't. Defined here at the
// consumer per CLAUDE.md "interfaces at the call site".
type storeWatcher interface {
	StartWatching() error
}

// sseEvent is a non-entity frame (refresh, config-error, config-changed,
// git, git:status) broadcast to every subscriber. Name and Data are the
// pre-rendered wire frame; they carry no entity identity, so there is
// nothing to gate. Entity changes do not travel through the broker: each
// connection reads them from the change feed ([changefeed.Bus]) and gates
// them itself (TKT-POT9GQ, TKT-3A1200).
type sseEvent struct {
	Name string
	Data string
}

// eventBroker manages SSE client connections for live-reload notifications.
type eventBroker struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
	closed  bool // set by close; later subscribers get a closed channel
}

func newEventBroker() *eventBroker {
	return &eventBroker{clients: make(map[chan sseEvent]struct{})}
}

func (b *eventBroker) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 4)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.clients[ch] = struct{}{}
	return ch
}

// close ends every stream. A subscriber reads what is already buffered,
// then sees its channel closed and returns.
func (b *eventBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}

func (b *eventBroker) unsubscribe(ch chan sseEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// broadcast sends a simple non-entity event (refresh / git).
func (b *eventBroker) broadcast(eventType string) {
	b.broadcastEvent(sseEvent{Name: eventType, Data: eventType})
}

// broadcastEvent fans an event out to every subscriber. Lossy: a full
// client buffer drops the event (the SSE feed is a hint, not a log).
func (b *eventBroker) broadcastEvent(event sseEvent) {
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default: // skip slow clients
		}
	}
	b.mu.Unlock()
}

// broadcastConfigError tells subscribers that a reload of file
// (data-entry.yaml or realtime.yaml) was rejected and the previous config is
// still serving. The frame carries the file name and the error's public
// message, never a host path.
func (b *eventBroker) broadcastConfigError(file string, ce *configError) {
	data, _ := json.Marshal(map[string]string{"file": file, "error": ce.public})
	b.broadcastEvent(sseEvent{Name: "config-error", Data: string(data)})
}

// broadcastGitStatus sends a git status update event.
func (b *eventBroker) broadcastGitStatus() {
	b.broadcastEvent(sseEvent{Name: "git:status", Data: "{}"})
}

// StartWatching begins file watching for live-reload. It does two
// independent things:
//
//  1. Subscribes to `data-entry.yaml` via the config loader so SPA
//     reloads pick up dashboard / palette / form config changes, and to
//     `realtime.yaml` when [SetRealtime] wired a holder.
//  2. Asks the store to start watching its own entity/relation files
//     (feature-detected via [storeWatcher] — fsstore implements it;
//     in-memory backends don't). The store's observer chain handles
//     reindex automatically; no callback wiring is needed at this
//     layer.
//
// The returned error covers only the config-subscriber failure
// (step 1). Store-watcher errors (step 2) are logged via slog.Warn
// because store watching is a live-reload nice-to-have, not a
// startup requirement.
//
// Stop via [App.StopWatching]. Note: that only releases the config
// subscription — the store-watcher lifecycle is owned by the store
// (closed when the store is closed).
//
// coverage-ignore-func: requires real filesystem events via the file watcher
func (a *App) StartWatching() error {
	// coverage-ignore-start: os-fs-event: startup wiring for file-watcher-backed config subscription, store
	// watcher, and store-event SSE bridge (project marks it coverage-ignore)
	// (1) data-entry.yaml subscription.
	if sub, ok := a.cfgLoader.(config.Subscriber); ok {
		stop, err := sub.Subscribe(context.Background(), ConfigFile, func() {
			if a.reloadPaused.Load() > 0 {
				return
			}
			// A rejection is already logged and broadcast; nothing more to do.
			_ = a.reloadConfig()
		})
		if err != nil {
			return err
		}
		a.stopConfigWatch = stop
		if err := watchRealtime(a, sub); err != nil {
			return err
		}
	}

	// (2) Store-level watcher (fsstore). Errors are logged, not
	// returned — store watching is a live-reload nice-to-have, not a
	// startup requirement. Stores that don't implement [storeWatcher]
	// (memstore, etc.) get a debug log instead of silence so
	// "why isn't live-reload working" doesn't require a goose chase.
	if sw, ok := a.store.(storeWatcher); ok {
		if err := sw.StartWatching(); err != nil {
			slog.Warn("store watcher not started", "error", err)
		}
	} else {
		slog.Debug("store does not implement watching; live-reload of external edits disabled",
			"store", fmt.Sprintf("%T", a.store))
	}

	// (3) Store-event -> change feed. Feeds the store's change watcher into
	// the bus every SSE connection reads. This is the path that makes a
	// write by ANOTHER process visible live (the pgstore cross-process feed,
	// TKT-WZYWM9) and surfaces fsstore's external-file-edit events. Local API
	// writes flow through here too, so the write handlers do not broadcast.
	a.startStoreEventBridge()

	return nil
	// coverage-ignore-end
}

// startStoreEventBridge starts the change feed on the store's change watcher.
// A feed that does not start (it was already started or closed) leaves SSE
// connections working without entity changes, so the failure is logged
// rather than returned.
//
// # Audit-isolation invariant
//
// The change feed NEVER carries audit records. It holds entity types only,
// and the wire carries `{type}`. Subject.ID / Subject.FromID from
// `denied-write` audit rows must NOT be forwarded to SSE: the audit log is the
// only intended audience for principal-attribution detail, and broadcasting it
// would leak the principal-to-entity topology to every subscriber.
//
// If a future feature needs an audit-aware SSE channel, it must expose a
// SEPARATE feed, gated by per-subscriber ACL, with its own redaction policy.
// The regression test `TestSSE_DoesNotFlowAuditEvents` pins this invariant.
func (a *App) startStoreEventBridge() {
	if err := a.feed.Start(a.store); err != nil {
		slog.Warn("sse: change feed not started", "error", err)
		return
	}
	a.live.start(a.feed, a.leases.wakeSource(), a.channels.wakeSource())
}

// relationEndTypes is every entity type a relation of state's metamodel can
// start or end at, each once. Unknown relation types yield nothing.
func relationEndTypes(state *Schema, relation string) []string {
	if state == nil || state.Meta == nil {
		return nil
	}
	def, ok := state.Meta.Relations[relation]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range append(append([]string{}, def.From...), def.To...) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// StartGitFetch begins periodic git fetch in the background.
// Returns a stop function to shut down the fetcher.
//
// coverage-ignore-func: background goroutine with timer
func (a *App) StartGitFetch() (stop func()) {
	gitOps := a.gitOps
	cfg := a.State().Cfg.Git

	if gitOps == nil || cfg == nil || cfg.FetchInterval <= 0 {
		return func() {} // no-op if git not configured or fetch disabled
	}

	// coverage-ignore-start: os-fs-event: background goroutine driven by a real time.Ticker running git fetch (project
	// marks it coverage-ignore)
	interval := time.Duration(cfg.FetchInterval) * time.Second
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := gitOps.Fetch(); err != nil {
					slog.Warn("git fetch error", "error", err)
				} else {
					// Broadcast git status update so UI can refresh
					a.broker.broadcast("git")
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	slog.Info("git background fetch started", "interval", interval)
	return func() {
		close(done)
	}
	// coverage-ignore-end
}

// configReadFailedMessage is the config-error text sent when data-entry.yaml
// cannot be read at all. The loader's own error is logged but kept off the
// wire because it can carry the host path of the project directory.
const configReadFailedMessage = "cannot read " + ConfigFile + "; see the server log"

// configError is a rejected data-entry.yaml reload. It carries the message
// safe to show to a browser separately from the error that is logged.
type configError struct {
	public string
	err    error
}

func (e *configError) Error() string { return e.err.Error() }
func (e *configError) Unwrap() error { return e.err }

// publicConfigErrorMessage renders a [loadConfig] error for the config-error
// frame. The text describes operator-authored config, which is not a secret,
// but it must not carry the host path of the project directory. The
// deprecated-syntax error names its file by absolute path, so it is re-rendered
// with the bare file name. Any other message is stripped of the project root as
// a backstop, so a future error source cannot put the path on the wire either.
func publicConfigErrorMessage(err error, root string) string {
	var migErr *migration.Error
	if errors.As(err, &migErr) {
		safe := *migErr
		safe.FilePath = ConfigFile
		return safe.Error()
	}
	msg := err.Error()
	if root = filepath.Clean(root); root != string(filepath.Separator) && root != "." {
		msg = strings.ReplaceAll(msg, root+string(filepath.Separator), "")
	}
	return msg
}

// reloadConfig re-reads data-entry.yaml, publishes a fresh Schema snapshot
// atomically, and tells browsers to refresh. Readers observe either the
// pre-reload or the post-reload snapshot, never a torn state.
//
// The file goes through [loadConfig], the same pipeline as startup. When it
// fails, nothing is published and the previous config keeps serving:
// swapping in a config startup would refuse turns a typo into HTTP 500s at
// first use (TKT-IMBOK). Browsers get a config-error frame instead of a
// refresh, and the returned error is a *configError.
func (a *App) reloadConfig() error {
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()
	if a.State() == nil {
		return nil
	}

	reject := func(ce *configError) error {
		slog.Warn("config reload rejected; keeping previous config", "error", ce.err)
		a.broker.broadcastConfigError(ConfigFile, ce)
		return ce
	}

	meta := a.Meta()
	cfgData, err := a.cfgLoader.Load(context.Background(), ConfigFile)
	if err != nil {
		return reject(&configError{public: configReadFailedMessage, err: err})
	}
	cfg, err := loadConfig(cfgData, meta, a.paths.Root, a.cfgLoader)
	if err != nil {
		return reject(&configError{public: publicConfigErrorMessage(err, a.paths.Root), err: err})
	}
	slog.Info("config reloaded")
	warnListVisibilityConflicts(cfg, meta, a.acl)

	// The provider recomputes the style map + OpenAPI generator (co-derived
	// from cfg/meta) and publishes atomically.
	a.schema.Reload(cfg, meta, true)

	// Re-resolve the palette against the new project palette. The service
	// keeps the previous user override if the on-disk file is broken —
	// better to show stale colors than crash or wipe.
	if err := a.palette.Reresolve(cfg.Palette); err != nil {
		slog.Warn("watcher: keeping previous user palette",
			"file", userPaletteFile, "error", err)
	}
	a.broker.broadcast("refresh")
	return nil
}

// handleSSE serves Server-Sent Events for live-reload notifications.
// Connected browsers receive events when project files change or entities
// are modified. Event types on the wire:
//   - ready: first frame on a connection. Its id is the resume token for
//     the position the stream starts at. Data is {}.
//   - refresh: files changed, reload needed. Also sent as the FIRST frame,
//     in place of ready, with data {"reason":"resync"} when the presented
//     resume token is unusable; the client cannot know what it missed and
//     re-fetches everything.
//   - config-error: a data-entry.yaml reload was rejected; the previous
//     config keeps serving. Data is {"file": ..., "error": ...}
//   - config-changed: a Configure save replaced the server; the stream ends.
//   - git / git:status: Git status changed (git:status carries empty JSON)
//   - entity:changed: Entities of a type changed; data is {"type": "..."}
//     with NO id. Create/update/delete all collapse to this single
//     type-scoped staleness signal, ACL-gated per connection — a
//     connection only receives a type its principal may read (TKT-POT9GQ).
//   - ping: sent every [ssePingInterval]; data is {}.
//
// ready, refresh-on-resync, the last entity:changed of a flush, and ping
// carry an `id:` resume token. A client reconnects with it in the
// Last-Event-ID header or the last_event_id query parameter (the SPA rebuilds
// its EventSource by hand, so it cannot rely on the browser resending the
// header) and receives one entity:changed frame per readable type that
// changed since (TKT-3A1200).
func (a *App) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE is reachable only from same-origin or explicitly allow-listed
	// dev origins (enforced by requireSameOrigin middleware). No CORS
	// reflection — that previously let any website subscribe to live
	// project events.

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	// The token is untrusted input. Subscribe checks its exact length before
	// decoding, and anything it did not mint is a resync; the value is never
	// echoed or logged.
	token := r.Header.Get("Last-Event-ID")
	if token == "" {
		token = r.URL.Query().Get("last_event_id")
	}
	sub := a.feed.Subscribe(token)
	defer sub.Close()

	rc := http.NewResponseController(w)
	conn := &sseConn{w: w, flusher: flusher, setDeadline: rc.SetWriteDeadline, timing: a.sseTiming}
	ready := map[string]string{}
	var live liveFrames // nil: no live views on this stream
	if r.URL.Path == "/api/v1/_events" {
		// Live views are v1 only; the legacy stream has no connection id.
		// Without the stashed principal a replay would run as the resolved
		// one, so the stream goes without live views instead.
		if owner, ok := replayPrincipalFrom(r.Context()); ok {
			c := a.live.register(owner)
			defer a.live.unregister(c)
			ready["conn"] = c.id
			live = c
		} else {
			slog.Error("live: stream has no stashed principal; live views are off for it")
		}
	}
	first := "ready"
	if sub.Resync() {
		first, ready["reason"] = "refresh", "resync"
	}
	data, _ := json.Marshal(ready)
	if conn.frame(first, sub.Token(sub.Start()), string(data)) != nil {
		return
	}

	a.runSSELoop(conn, r, ch, sub, sub.Start(), live)
}

// sseFlushInterval bounds how long a coalesced type-change burst waits
// before flushing. A write burst (e.g. a `git pull` landing many files,
// or a cascade) produces many store events in quick succession; instead
// of one wire frame per event we accumulate the affected types over this
// window and emit one `entity:changed` frame per type. The window is a
// hint-latency tradeoff — the client re-fetches a fraction of a second
// later, invisible to users — not a correctness knob.
const sseFlushInterval = 200 * time.Millisecond

// ssePingInterval is how often an idle stream sends a ping frame. It keeps
// proxies from closing the connection and refreshes the client's resume
// token. It runs on a fixed timer, so a ping says nothing about activity.
const ssePingInterval = 25 * time.Second

// sseTimings holds the stream intervals; tests shorten them.
type sseTimings struct {
	flush time.Duration
	ping  time.Duration
}

var defaultSSETimings = sseTimings{flush: sseFlushInterval, ping: ssePingInterval}

// sseConn writes frames to one SSE connection. Every write first sets a
// write deadline, so a client that stops reading is dropped instead of
// holding its handler goroutine forever.
type sseConn struct {
	w           io.Writer
	flusher     http.Flusher
	setDeadline func(time.Time) error // nil when the writer has none (tests)
	timing      sseTimings
}

// sseMissedPings is how many ping intervals a frame write may block before
// the connection counts as dead. Two absorbs one slow write without letting a
// stalled reader hold a stream open for long.
const sseMissedPings = 2

// errStreamEnded ends a stream that finished on purpose.
var errStreamEnded = errors.New("sse: stream ended")

// frame writes one event and flushes it. id may be empty. A returned error
// means the connection is unusable and the stream must end.
func (c *sseConn) frame(name, id, data string) error {
	if c.setDeadline != nil {
		err := c.setDeadline(time.Now().Add(sseMissedPings * c.timing.ping))
		if err != nil && !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	}
	var err error
	if id != "" {
		_, err = fmt.Fprintf(c.w, "event: %s\nid: %s\ndata: %s\n\n", name, id, data)
	} else {
		_, err = fmt.Fprintf(c.w, "event: %s\ndata: %s\n\n", name, data)
	}
	if err != nil {
		return err
	}
	c.flusher.Flush()
	return nil
}

// liveFrames is what a stream needs from its live-view subscriptions: a
// signal that some changed, and their keys.
type liveFrames interface {
	wake() <-chan struct{}
	takeChanged() []string
}

// changeSource is what a stream needs from its change-feed subscription.
type changeSource interface {
	Wake() <-chan struct{}
	Take() changefeed.Batch
	Token(seq uint64) string
}

// runSSELoop is the per-connection event loop. It is split out from
// handleSSE so tests can drive it with an in-memory writer. start is the
// position the subscription began at.
//
// Entity changes are gated per-connection: the connection's principal
// read verdict for each entity type is resolved via the read gate and
// cached. A pending set of "stale types" is accumulated and flushed on a
// timer (coalescing bursts, AC5).
//
// A relation change can alter the principal's read scope (a member-of
// edge changes group membership; a role-relation / containment edge
// changes inheritance). It re-derives a FRESH read gate (a new
// acl.Request whose member-of closure is walked against the current
// graph — the connection's original Request memoizes that closure for its
// lifetime, RR-K2WKEJ) and clears the cached verdicts so the next entity
// event re-resolves against current membership. The re-derive is coalesced
// into the same flush window so a burst of relation writes triggers one
// re-walk, not one per edge.
//
// Resume tokens: cursor is the feed position taken so far; flushed is the
// position whose changes have all been gated and written. The last frame
// of a flush carries cursor, and a ping carries flushed, so a client that
// resumes from any token it saw misses no change the feed counted. The
// feed counts what the store delivered: a store that drops events on a
// full buffer drops them for resume too. When a batch holds only types the
// principal cannot read, nothing is written and flushed simply moves on,
// so a hidden entity write produces no frame and does not move a frame in
// time. A relation write of any kind starts a flush window to re-derive
// the gate, so it can make a readable frame arrive up to one window early
// (docs/acl-security.md).
//
// Non-entity events (refresh, git) pass through ungated and immediately.
// The stream ends on a closed broker channel, the request context, or
// right after a config-changed frame, so that frame is the last one
// written even when a flush or ping is due at the same moment.
func (a *App) runSSELoop(
	conn *sseConn, r *http.Request, ch <-chan sseEvent, feed changeSource, start uint64, live liveFrames,
) {
	st := &sseStream{
		app:      a,
		conn:     conn,
		feed:     feed,
		gate:     readGateFromContext(r.Context()),
		verdicts: make(map[string]bool),
		pending:  make(map[string]struct{}),
		cursor:   start,
		flushed:  start,
	}

	// One timer for the connection's lifetime, created stopped and reset
	// to (re)arm the coalescing window. flushArmed tracks whether the
	// timer is currently running so a burst doesn't keep resetting it.
	flush := time.NewTimer(conn.timing.flush)
	if !flush.Stop() {
		<-flush.C
	}
	defer flush.Stop()
	flushArmed := false
	ping := time.NewTicker(conn.timing.ping)
	defer ping.Stop()
	var liveWake <-chan struct{} // nil: no live views on this stream
	if live != nil {
		liveWake = live.wake()
	}

	ctx := r.Context()
	for {
		var err error
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			err = st.broker(event)
		case <-feed.Wake():
			if st.take(ctx) && !flushArmed {
				flush.Reset(conn.timing.flush)
				flushArmed = true
			}
		case <-flush.C:
			flushArmed = false
			err = st.flush(ctx)
		case <-liveWake:
			err = writeLiveChanged(conn, live.takeChanged())
		case <-ping.C:
			err = conn.frame("ping", feed.Token(st.flushed), "{}")
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

// writeLiveChanged writes one live:changed frame per key. The key is the
// client's own label for a subscription, so the frame carries no data.
func writeLiveChanged(conn *sseConn, keys []string) error {
	for _, key := range keys {
		data, _ := json.Marshal(map[string]string{"key": key})
		if err := conn.frame("live:changed", "", string(data)); err != nil {
			return err
		}
	}
	return nil
}

// sseStream is the per-connection state of App.runSSELoop.
type sseStream struct {
	app      *App
	conn     *sseConn
	feed     changeSource
	gate     readGate
	verdicts map[string]bool     // entityType -> visible; cached per connection
	pending  map[string]struct{} // types to write at the next flush
	regate   bool                // a coalesced relation change is pending → re-derive on flush
	cursor   uint64              // feed position taken so far
	flushed  uint64              // feed position whose changes are all gated and written
}

// broker writes a non-entity frame: pre-rendered and ungated.
func (st *sseStream) broker(event sseEvent) error {
	if event.Name == "" {
		// A zero-value frame is a programming error. Drop it rather than
		// emit a malformed empty frame.
		slog.Warn("sse: dropping zero-value broker event")
		return nil
	}
	if err := st.conn.frame(event.Name, "", event.Data); err != nil {
		return err
	}
	if event.Name == sseConfigChanged {
		return errStreamEnded
	}
	return nil
}

// take collects the changes since the last take and reports whether a
// flush is needed.
func (st *sseStream) take(ctx context.Context) bool {
	batch := st.feed.Take()
	// Coalesce: re-derive the gate at most once per flush window (a bulk
	// relation import otherwise walks member-of per edge per connection).
	st.regate = st.regate || batch.Relation
	// An edge is also part of how both ends are shown: a board column or
	// list section taken from a relation, a detail panel's relation fields.
	// Mark both ends' types stale so those views re-fetch; the read gate
	// applies to them as to any entity type.
	types := batch.Types
	if len(batch.RelationTypes) > 0 {
		state := st.app.State()
		types = slices.Clone(types)
		for _, rel := range batch.RelationTypes {
			types = append(types, relationEndTypes(state, rel)...)
		}
	}
	for _, typ := range types {
		// While a re-gate is pending the current gate is known stale, so
		// defer the visibility decision to the flush (which re-checks every
		// pending type against the fresh gate). Otherwise filter at entry to
		// avoid buffering types the principal can't read.
		if st.regate || st.app.entityTypeVisible(ctx, st.gate, st.verdicts, typ) {
			st.pending[typ] = struct{}{}
		}
	}
	st.cursor = batch.Seq
	if len(st.pending) == 0 && !st.regate {
		st.flushed = st.cursor // nothing readable to write
		return false
	}
	return true
}

// flush writes one entity:changed frame per pending readable type, the
// last one carrying the resume token for the cursor.
func (st *sseStream) flush(ctx context.Context) error {
	if st.regate {
		st.gate = st.app.freshReadGate(ctx, st.gate)
		clear(st.verdicts)
		st.regate = false
	}
	var visible []string
	for typ := range st.pending {
		if st.app.entityTypeVisible(ctx, st.gate, st.verdicts, typ) {
			visible = append(visible, typ)
		}
	}
	clear(st.pending)
	slices.Sort(visible)
	for i, typ := range visible {
		id := ""
		if i == len(visible)-1 {
			id = st.feed.Token(st.cursor)
		}
		data, _ := json.Marshal(map[string]string{"type": typ})
		if err := st.conn.frame("entity:changed", id, string(data)); err != nil {
			return err
		}
	}
	st.flushed = st.cursor
	return nil
}

// freshReadGate returns a read gate whose member-of closure is resolved
// against the CURRENT graph, for use after a role-conferring relation
// change invalidates the connection's cached membership. When ACL is a
// *acl.Declarative it re-resolves the principal's identity (a fresh
// member-of walk) and opens a new acl.Request on it; otherwise (NopACL, or a
// resolution failure) it keeps
// the existing gate — for NopACL the verdict is constant-true so there
// is nothing to refresh, and on a transient resolve error keeping the
// prior gate is fail-safe (it can only be equal-or-narrower than a stale
// allow, and the next relation change retries).
func (a *App) freshReadGate(ctx context.Context, current readGate) readGate {
	d, ok := a.acl.(*acl.Declarative)
	if !ok || d == nil {
		return current
	}
	// The ctx principal was already resolved by attachACLRequest (its User is
	// the entity id when principal_property matched), so this refresh walks
	// membership without repeating the lookup. It is an explicit edge step
	// for a long-lived stream, not a lazy evaluation fallback.
	req, err := acl.OpenRequest(ctx, d, principal.From(ctx))
	if err != nil {
		slog.Warn("sse: re-deriving read gate failed; keeping prior gate", "err", err)
		return current
	}
	gate, err := newACLReadGate(req)
	if err != nil {
		slog.Warn("sse: re-deriving read gate failed; keeping prior gate", "err", err)
		return current
	}
	return gate
}

// entityTypeVisible returns whether the connection's principal may read
// the type at all, caching the verdict for the connection's lifetime
// (cleared on a relation change). DenyAll → not visible (withhold the
// nudge). AllowAll / Query → visible (the principal can read at least
// some entities of the type, so "the type changed, re-fetch" leaks
// nothing beyond the gated list they could already poll).
//
// Fail-closed: an unresolvable / zero verdict withholds (RR-MTUW2N) —
// the only signal on the wire is the type, so there is no error to
// echo; the miss degrades to "no nudge", and the client's periodic
// reconnect re-fetch reconciles.
func (a *App) entityTypeVisible(ctx context.Context, gate readGate, cache map[string]bool, entityType string) bool {
	if v, ok := cache[entityType]; ok {
		return v
	}
	rqr := gate.ReadQuery(ctx, entityType)
	visible := rqr.AllowAll || rqr.Query != nil
	cache[entityType] = visible
	return visible
}

// noCacheMiddleware sets no-cache headers on dynamic responses so that
// browsers always fetch fresh data after file changes trigger a reload.
// This replaces the previous reloadLockMiddleware, which also held the
// now-deleted App.mu read lock — handlers now get coherent reloadable
// state via a.State() without any lock acquisition.
//
// Static files (/static/*) are served separately and bypass this
// middleware, so they retain normal caching behavior.
//
// Under ACL, API responses are additionally per-principal: when a
// principal header is configured (SetPrincipalHeader), `Vary` names it
// so any cache that ignores the no-store directive still keys on the
// identity header instead of serving principal A's filtered response
// to principal B (TKT-VMD8 AC10, RR-VDTW).
func (a *App) noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if a.principalHeader != "" {
			w.Header().Add("Vary", a.principalHeader)
		}
		next.ServeHTTP(w, r)
	})
}
