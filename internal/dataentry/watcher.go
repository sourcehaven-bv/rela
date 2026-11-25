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
	"github.com/Sourcehaven-BV/rela/internal/readtime"
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

// sseHandler owns the live-reload event stream (/api/events and
// /api/v1/_events) and the bridge that feeds store changes into it.
// Extracted from App to take the per-connection loop and its read-gate
// helpers off that god-object.
//
// The collaborators are closures over App rather than captured values: test
// builders reassign App.broker, App.acl and App.leases after construction,
// and a captured copy would go stale (the attachmentHandler idiom).
type sseHandler struct {
	// schema is the reloadable snapshot; a relation change reads its
	// metamodel to find the relation's end types.
	schema func() *Schema
	// acl is the policy a stream re-derives its read gate from after a
	// relation change or a new UTC day.
	acl func() acl.ACL
	// broker carries the non-entity frames (refresh, git, config-error).
	broker func() *eventBroker
	// feed numbers store change events and lets a stream resume.
	feed func() *changefeed.Bus
	// live holds the live-view subscriptions of v1 streams.
	live func() *liveHub
	// source is the store whose change watcher the bridge starts the feed on.
	source func() changefeed.Source
	// leaseWake and channelWake wake the live-view worker on lease and
	// channel changes.
	leaseWake   func() leaseWaker
	channelWake func() func() channelChanges

	// timing holds the stream's flush and ping intervals; tests shorten them
	// before serving.
	timing sseTimings
}

// newSSEHandler wires the handler to app's live collaborators.
func newSSEHandler(app *App) *sseHandler {
	return &sseHandler{
		schema:      app.State,
		acl:         func() acl.ACL { return app.acl },
		broker:      func() *eventBroker { return app.broker },
		feed:        func() *changefeed.Bus { return app.feed },
		live:        func() *liveHub { return app.live },
		source:      func() changefeed.Source { return app.store },
		leaseWake:   func() leaseWaker { return app.leases.wakeSource() },
		channelWake: func() func() channelChanges { return app.channels.wakeSource() },
		timing:      defaultSSETimings,
	}
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
		default:
		}
	}
	b.mu.Unlock()
}

// broadcastConfigError tells subscribers that a reload of file
// (data-entry.yaml or realtime.yaml) was rejected and the previous version of
// file is still serving. The frame carries the file name and the error's
// public message, never a host path. applied names the other file when the
// same reload put it in force (realtime.yaml applied, data-entry.yaml then
// refused against it); empty when nothing was applied.
func (b *eventBroker) broadcastConfigError(file string, ce *configError, applied string) {
	frame := map[string]string{"file": file, "error": ce.public}
	if applied != "" {
		frame["applied"] = applied
	}
	data, _ := json.Marshal(frame)
	b.broadcastEvent(sseEvent{Name: "config-error", Data: string(data)})
}

// broadcastGitStatus sends a git status update event.
func (b *eventBroker) broadcastGitStatus() {
	b.broadcastEvent(sseEvent{Name: "git:status", Data: "{}"})
}

// StartWatching subscribes to `data-entry.yaml` (and `realtime.yaml` when
// [SetRealtime] wired a holder) and asks the store to watch its own files when
// it implements [storeWatcher]. The store's observers handle reindexing.
//
// Only the config-subscriber failure is returned; a store-watcher failure is
// logged, because live reload is not a startup requirement. [App.StopWatching]
// releases the config subscription; the store owns its watcher.
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

	if sw, ok := a.store.(storeWatcher); ok {
		if err := sw.StartWatching(); err != nil {
			slog.Warn("store watcher not started", "error", err)
		}
	} else {
		slog.Debug("store does not implement watching; live-reload of external edits disabled",
			"store", fmt.Sprintf("%T", a.store))
	}

	a.sse.startStoreEventBridge()

	return nil
	// coverage-ignore-end
}

// startStoreEventBridge starts the change feed on the store's change watcher.
// A feed that does not start is logged, and SSE keeps working without entity
// changes.
//
// The feed NEVER carries audit records: it holds entity types only. Forwarding
// `denied-write` subjects would leak principal-to-entity topology to every
// subscriber. An audit-aware channel needs a separate, ACL-gated feed.
// TestSSE_DoesNotFlowAuditEvents pins this.
func (h *sseHandler) startStoreEventBridge() {
	feed := h.feed()
	if err := feed.Start(h.source()); err != nil {
		slog.Warn("sse: change feed not started", "error", err)
		return
	}
	h.live().start(feed, h.leaseWake(), h.channelWake())
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
// Returns a stop function to shut down the fetcher. See
// [gitHandler.startFetch].
//
// coverage-ignore-func: delegates to the timer-driven background fetcher
func (a *App) StartGitFetch() (stop func()) {
	return a.gitAPI.startFetch()
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
	cfg, ce := readConfig(a)
	if ce != nil {
		slog.Warn("config reload rejected; keeping previous config", "error", ce.err)
		a.broker.broadcastConfigError(ConfigFile, ce, "")
		return ce
	}
	publishConfig(a, cfg)
	a.broker.broadcast("refresh")
	return nil
}

// readConfig reads data-entry.yaml from disk and checks it, including its
// `channels:` section against the realtime config in force. The caller
// holds a.reloadMu. A free function because App's method count is pinned
// (plimsoll).
func readConfig(a *App) (*Config, *configError) {
	cfgData, err := a.cfgLoader.Load(context.Background(), ConfigFile)
	if err != nil {
		return nil, &configError{public: configReadFailedMessage, err: err}
	}
	cfg, err := loadConfig(cfgData, a.Meta(), a.paths.Root, a.cfgLoader)
	if err == nil {
		err = checkChannelsPresentation(cfg, a.realtime)
	}
	if err != nil {
		return nil, &configError{public: publicConfigErrorMessage(err, a.paths.Root), err: err}
	}
	return cfg, nil
}

// publishConfig puts cfg in force. The caller holds a.reloadMu and
// broadcasts refresh.
func publishConfig(a *App, cfg *Config) {
	slog.Info("config reloaded")
	meta := a.Meta()
	warnListVisibilityConflicts(cfg, meta, a.acl)

	a.schema.Reload(cfg, meta, true)

	if err := a.palette.Reresolve(cfg.Palette); err != nil {
		slog.Warn("watcher: keeping previous user palette",
			"file", userPaletteFile, "error", err)
	}
}

// handleSSE serves Server-Sent Events for live reload. Events: ready (first
// frame, carries the resume token), refresh (also the first frame with
// reason "resync" when the resume token is unusable), config-error,
// config-changed (the stream ends), git / git:status, entity:changed (data
// {"type"} with no id, sent only for types the principal may read), and ping.
//
// ready, resync, the last entity:changed of a flush and ping carry an `id:`
// token. A client resumes with it in Last-Event-ID or last_event_id and gets
// one entity:changed per readable type that changed since.
//
//archplan:auth=required
func (h *sseHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	expiry, hasExpiry := credentialExpiry(r.Context())
	if hasExpiry && !time.Now().Before(expiry) {
		return
	}

	broker := h.broker()
	ch := broker.subscribe()
	defer broker.unsubscribe(ch)

	token := r.Header.Get("Last-Event-ID")
	if token == "" {
		token = r.URL.Query().Get("last_event_id")
	}
	sub := h.feed().Subscribe(token)
	defer sub.Close()

	rc := http.NewResponseController(w)
	conn := &sseConn{w: w, flusher: flusher, setDeadline: rc.SetWriteDeadline, timing: h.timing, expiry: expiry}
	ready := map[string]string{}
	var live liveFrames
	if r.URL.Path == "/api/v1/_events" {
		if owner, ok := replayPrincipalFrom(r.Context()); ok {
			hub := h.live()
			c := hub.register(owner)
			defer hub.unregister(c)
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

	h.runSSELoop(conn, r, ch, sub, sub.Start(), live)
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

	// expiry is when the stream's credential stops being valid; zero when
	// it has none (TKT-5I6REX). runSSELoop ends the stream at this moment.
	// A write deadline never reaches past it, so a blocked write cannot
	// keep an expired stream open.
	expiry time.Time
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
		deadline := time.Now().Add(sseMissedPings * c.timing.ping)
		if !c.expiry.IsZero() && c.expiry.Before(deadline) {
			deadline = c.expiry
		}
		err := c.setDeadline(deadline)
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

// runSSELoop is the per-connection event loop; start is the subscription's
// feed position. Entity changes are gated by the connection's cached
// per-type read verdicts and flushed on a timer. A relation change re-derives
// a fresh read gate, coalesced into the flush window, because membership may
// have changed.
//
// The last frame of a flush carries the cursor and a ping carries the flushed
// position, so a resuming client misses nothing the feed counted. Other events
// pass through ungated. The stream ends on a closed broker, the request
// context, credential expiry or right after a config-changed frame.
func (h *sseHandler) runSSELoop(
	conn *sseConn, r *http.Request, ch <-chan sseEvent, feed changeSource, start uint64, live liveFrames,
) {
	st := &sseStream{
		h:        h,
		conn:     conn,
		feed:     feed,
		gate:     readGateFromContext(r.Context()),
		gateDay:  readDay(readtime.Or(r.Context(), nowFunc)),
		verdicts: make(map[string]bool),
		pending:  make(map[string]struct{}),
		cursor:   start,
		flushed:  start,
	}

	flush := time.NewTimer(conn.timing.flush)
	if !flush.Stop() {
		<-flush.C
	}
	defer flush.Stop()
	flushArmed := false
	ping := time.NewTicker(conn.timing.ping)
	defer ping.Stop()
	var liveWake <-chan struct{}
	if live != nil {
		liveWake = live.wake()
	}
	var expired <-chan time.Time
	if !conn.expiry.IsZero() {
		expire := time.NewTimer(time.Until(conn.expiry))
		defer expire.Stop()
		expired = expire.C
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
		case <-expired:
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

// sseStream is the per-connection state of [sseHandler.runSSELoop].
type sseStream struct {
	h        *sseHandler
	conn     *sseConn
	feed     changeSource
	gate     readGate
	verdicts map[string]bool     // entityType -> visible; cached per connection
	pending  map[string]struct{} // types to write at the next flush
	regate   bool                // a coalesced relation change is pending → re-derive on flush
	gateDay  string              // UTC day of the gate's read time; a new day re-derives it
	cursor   uint64              // feed position taken so far
	flushed  uint64              // feed position whose changes are all gated and written
}

// broker writes a non-entity frame: pre-rendered and ungated.
func (st *sseStream) broker(event sseEvent) error {
	if event.Name == "" {
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
	st.regate = st.regate || batch.Relation
	st.regate = st.regate || readDay(nowFunc()) != st.gateDay
	types := batch.Types
	if len(batch.RelationTypes) > 0 {
		state := st.h.schema()
		types = slices.Clone(types)
		for _, rel := range batch.RelationTypes {
			types = append(types, relationEndTypes(state, rel)...)
		}
	}
	for _, typ := range types {
		if st.regate || st.typeVisible(ctx, typ) {
			st.pending[typ] = struct{}{}
		}
	}
	st.cursor = batch.Seq
	if len(st.pending) == 0 && !st.regate {
		st.flushed = st.cursor
		return false
	}
	return true
}

// flush writes one entity:changed frame per pending readable type, the
// last one carrying the resume token for the cursor.
func (st *sseStream) flush(ctx context.Context) error {
	if st.regate {
		now := nowFunc()
		gate, ok := st.h.freshReadGate(readtime.With(ctx, now), st.gate)
		if !ok && readDay(now) != st.gateDay {
			clear(st.pending)
			st.flushed = st.cursor
			return nil
		}
		st.gate = gate
		st.gateDay = readDay(now)
		clear(st.verdicts)
		st.regate = false
	}
	var visible []string
	for typ := range st.pending {
		if st.typeVisible(ctx, typ) {
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

// readDay is the UTC calendar day of t, the unit a conditional grant is
// judged in.
func readDay(t time.Time) string { return t.UTC().Format(time.DateOnly) }

// freshReadGate returns a read gate whose member-of closure is resolved
// against the CURRENT graph, for use after a role-conferring relation
// change invalidates the connection's cached membership, or after a new
// UTC day may have ended a conditional grant. When ACL is a
// *acl.Declarative it re-resolves the principal's identity (a fresh
// member-of walk) and opens a new acl.Request on it. For NopACL the verdict
// is constant-true, so it returns current with ok true. On a resolution
// failure it returns current with ok false; the caller decides whether the
// prior gate is still safe to use.
func (h *sseHandler) freshReadGate(ctx context.Context, current readGate) (readGate, bool) {
	d, ok := h.acl().(*acl.Declarative)
	if !ok || d == nil {
		return current, true
	}
	req, err := acl.OpenRequest(ctx, d, principal.From(ctx))
	if err != nil {
		slog.Warn("sse: re-deriving read gate failed", "err", err)
		return current, false
	}
	gate, err := newACLReadGate(req)
	if err != nil {
		slog.Warn("sse: re-deriving read gate failed", "err", err)
		return current, false
	}
	return gate, true
}

// typeVisible returns whether the connection's principal may read the type
// at all under st.gate, caching the verdict in st.verdicts for the
// connection's lifetime (cleared on a relation change). DenyAll → not visible (withhold the
// nudge). AllowAll / Query → visible (the principal can read at least
// some entities of the type, so "the type changed, re-fetch" leaks
// nothing beyond the gated list they could already poll).
//
// Fail-closed: an unresolvable / zero verdict withholds (RR-MTUW2N) —
// the only signal on the wire is the type, so there is no error to
// echo; the miss degrades to "no nudge", and the client's periodic
// reconnect re-fetch reconciles.
func (st *sseStream) typeVisible(ctx context.Context, entityType string) bool {
	if v, ok := st.verdicts[entityType]; ok {
		return v
	}
	rqr := st.gate.ReadQuery(ctx, entityType)
	visible := rqr.AllowAll || rqr.Query != nil
	st.verdicts[entityType] = visible
	return visible
}

// noCacheMiddleware sets no-cache headers on dynamic responses so browsers
// fetch fresh data after a reload. /static/ bypasses it.
//
// API responses are per-principal: when a principal header is configured,
// `Vary` names it so a cache that ignores no-store still keys on identity.
func (a *App) noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if a.principalHeader != "" {
			w.Header().Add("Vary", a.principalHeader)
		}
		next.ServeHTTP(w, r)
	})
}
