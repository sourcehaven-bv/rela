package dataentry

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	ygows "github.com/Sourcehaven-BV/rela/third_party/ygows"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// CollabPathPrefix is where the co-editing relay is mounted (TKT-CJL604).
// A room is one entity body: /api/v1/_collab/{type}/{id}.
const CollabPathPrefix = "/api/v1/_collab/"

// Close codes the relay sends. The SPA dispatches on them.
const (
	// collabCloseRejoin asks the client to reconnect: the connection
	// reached its lifetime cap, its credential expired, or the
	// configuration it was admitted under was replaced. The reconnect
	// authorizes again, so a changed permission applies.
	collabCloseRejoin = 4001
	// collabCloseStaleBuild tells the client its SPA build differs from
	// the server's. Peers on different builds may serialize the same
	// document differently, so the client reloads instead of editing.
	collabCloseStaleBuild = 4409
	// collabCloseGone tells a client that its room belongs to an earlier
	// server process. The rooms of that process are gone, so the client
	// must not refill a new room with its old document.
	collabCloseGone = 4410
	// collabCloseTooLarge ends a peer whose update would take the room's
	// document past collabMaxRoomUpdateBytes.
	collabCloseTooLarge = 4413
)

// Relay limits. A room is one entity body, so the caps are generous for
// people and tight for anything automated.
const (
	collabMaxPeersPerRoom = 32
	collabMaxRooms        = 2048
	collabMaxConnections  = 4096
	// collabMaxConnsPerPrincipal keeps one principal from taking the
	// room and peer slots everyone shares.
	collabMaxConnsPerPrincipal = 16
	collabMaxMessageBytes      = 4 << 20
	collabMaxUpdateBytes       = 2 << 20
	// collabMaxRoomUpdateBytes caps the update bytes a room takes while
	// it is loaded. A document only grows (deletes keep tombstones), so
	// this bounds its memory. Typing a body sends a few dozen bytes per
	// keystroke.
	collabMaxRoomUpdateBytes = 32 << 20
	// Per peer: typing and cursor moves send a few messages a second.
	collabMessageRate  = 50
	collabMessageBurst = 200
	// Awareness: one browser tab is one client.
	collabMaxAwarenessBytesPerRoom   = 1 << 20
	collabMaxAwarenessClientsPerRoom = 128
	collabMaxAwarenessClientsPerConn = 2
	// collabRecheck is how often an open connection checks its access
	// again. A change to the entity or its relations that hides it, or
	// that takes away update, ends the connection within this time.
	collabRecheck = 30 * time.Second
	// collabLifetime bounds one connection. The principal's identity and
	// memberships are resolved when the connection opens, so this is how
	// long a membership change or a logout can take to apply.
	collabLifetime = 2 * time.Minute
	// collabRoomIdle keeps an empty room so a client that reconnects after
	// the lifetime cap or a network blip finds its document. The SPA stops
	// rejoining after three minutes without word from the relay (EXPIRE_MS
	// in collabRoom.ts), so a client never refills a room that was dropped.
	collabRoomIdle = 5 * time.Minute
)

// collabCloseError is a context cancel cause the relay turns into a close
// frame (ygo reads CloseCode from the cause).
type collabCloseError struct {
	code   int
	reason string
}

func (c collabCloseError) Error() string { return c.reason }

func (c collabCloseError) CloseCode() (code int, reason string) { return c.code, c.reason }

// CollabRelay is the process-wide co-editing relay. It holds the live
// documents, so one relay outlives the app generations a Configure save
// replaces: a room keeps its document while its peers reconnect to the
// new generation. Admission is per generation (see [EnableCollab]).
//
// The relay never writes to the store. Clients save the body through the
// ordinary PATCH path, under their own ACL and audit.
type CollabRelay struct {
	srv   *ygows.Server
	build string
	// epoch names this process's rooms; see [CollabRelay.Epoch].
	epoch string

	mu sync.Mutex
	// roomBytes is the update bytes each loaded room took.
	roomBytes map[string]int64
	// conns is the open connections per principal.
	conns map[string]int
}

// collabAdmitKey carries the admission verdict from the handler to the
// relay's Authorize hook on the request context.
type collabAdmitKey struct{}

// collabAdmission is the verdict the handler passes to Authorize.
type collabAdmission struct {
	readOnly bool
	user     string
}

// collabPeerColor is the cursor color a client may choose: a plain
// #rrggbb value, so it cannot carry markup or a CSS expression.
var collabPeerColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// collabStampUser returns the awareness stamp for a connection of user.
// Other editors label a cursor with its authenticated principal, never a
// name the client chose. The color is the client's when it is a plain
// #rrggbb value and is dropped otherwise. The rest of the state, such as
// the cursor position, passes through.
func collabStampUser(user string) func(map[string]any) map[string]any {
	return func(state map[string]any) map[string]any {
		stamped := map[string]any{"name": user}
		if u, ok := state["user"].(map[string]any); ok {
			if c, ok := u["color"].(string); ok && collabPeerColor.MatchString(c) {
				stamped["color"] = c
			}
		}
		state["user"] = stamped
		return state
	}
}

// NewCollabRelay builds the relay. build is the SPA build id the server
// embeds (see [EmbeddedBuildID]); a client on another build is refused.
// An empty build skips the check, as in development.
func NewCollabRelay(build string) *CollabRelay {
	c := &CollabRelay{
		build:     build,
		epoch:     newCollabEpoch(),
		roomBytes: map[string]int64{},
		conns:     map[string]int{},
	}
	srv := ygows.NewServer()
	srv.MaxPeersPerRoom = collabMaxPeersPerRoom
	srv.MaxRooms = collabMaxRooms
	srv.MaxConnections = collabMaxConnections
	srv.MaxMessageBytes = collabMaxMessageBytes
	srv.MaxUpdateBytes = collabMaxUpdateBytes
	srv.MessageRateLimit = collabMessageRate
	srv.MessageRateBurst = collabMessageBurst
	srv.MaxAwarenessBytesPerRoom = collabMaxAwarenessBytesPerRoom
	srv.MaxAwarenessClientsPerRoom = collabMaxAwarenessClientsPerRoom
	srv.MaxAwarenessClientsPerConn = collabMaxAwarenessClientsPerConn
	srv.RoomIdleTimeout = collabRoomIdle
	srv.OnPeerUpdate = c.admitUpdate
	srv.OnUnloadDocument = func(_ context.Context, room string) {
		c.mu.Lock()
		delete(c.roomBytes, room)
		c.mu.Unlock()
	}
	// requireSameOrigin already checked the Origin of the upgrade, with
	// the server's own allow-list; ygo's check would repeat it without
	// that list and refuse the dev origins it admits.
	srv.AllowedOrigins = []string{"*"}
	srv.Authorize = func(r *http.Request) (ygows.ConnectionConfig, bool) {
		adm, ok := r.Context().Value(collabAdmitKey{}).(collabAdmission)
		if !ok {
			// Not admitted by the handler: fail closed.
			return ygows.ConnectionConfig{}, false
		}
		return ygows.ConnectionConfig{
			ReadOnly:       adm.readOnly,
			StampAwareness: collabStampUser(adm.user),
			Owner:          adm.user,
		}, true
	}
	c.srv = srv
	return c
}

// Epoch names the rooms of this process. The SPA joins with the epoch it
// was served; a client of an earlier process is refused, because its old
// document would refill a room a peer may have seeded anew from the store.
func (c *CollabRelay) Epoch() string { return c.epoch }

func newCollabEpoch() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b)
}

// admitUpdate refuses an update that would take its room past
// collabMaxRoomUpdateBytes. The peer is closed; its client saves what it
// holds through PATCH and edits alone.
func (c *CollabRelay) admitUpdate(info ygows.PeerUpdateInfo, apply func() error) error {
	n := int64(len(info.Update))
	c.mu.Lock()
	over := c.roomBytes[info.Room]+n > collabMaxRoomUpdateBytes
	if !over {
		c.roomBytes[info.Room] += n
	}
	c.mu.Unlock()
	if over {
		return &ygows.PeerClose{Code: collabCloseTooLarge, Reason: "document too large"}
	}
	return apply()
}

// acquire counts a connection of user, or reports that user holds the
// most it may.
func (c *CollabRelay) acquire(user string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conns[user] >= collabMaxConnsPerPrincipal {
		return false
	}
	c.conns[user]++
	return true
}

func (c *CollabRelay) release(user string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conns[user]--
	if c.conns[user] <= 0 {
		delete(c.conns, user)
	}
}

// Shutdown closes every connection and waits for them, or for ctx.
// http.Server.Shutdown does not close hijacked connections, so the
// process calls this after it.
func (c *CollabRelay) Shutdown(ctx context.Context) error {
	return c.srv.Shutdown(ctx)
}

// EmbeddedBuildID returns the build id the SPA build wrote next to its
// bundle, or "" when the bundle has none (a development tree).
func EmbeddedBuildID() string {
	b, err := fs.ReadFile(staticFiles, "static/v2/build-id.txt")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// EnableCollab admits a's requests to relay. A package function because
// App is at its plimsoll method cap. Call it after [App.SetSecurityConfig]
// and before serving. Retire ends the connections a admitted, which then
// reconnect to the new generation.
//
// Nil: relay rejected — leave collab off by not calling this.
func EnableCollab(a *App, relay *CollabRelay) error {
	if relay == nil {
		return errors.New("dataentry: EnableCollab requires a relay")
	}
	// The relay accepts any Origin because requireSameOrigin, which the
	// security config installs, checks the upgrade first.
	if a.security == nil {
		return errors.New("dataentry: EnableCollab requires SetSecurityConfig first")
	}
	a.collab = &collabHandler{
		relay:    relay,
		reader:   func() visibleReader { return a.visibleReader },
		acl:      func() acl.ACL { return a.acl },
		retired:  make(chan struct{}),
		lifetime: collabLifetime,
		recheck:  collabRecheck,
		now:      time.Now,
	}
	return nil
}

// collabHandler admits one generation's requests to the relay.
type collabHandler struct {
	relay  *CollabRelay
	reader func() visibleReader
	acl    func() acl.ACL
	// retired is closed when the generation is replaced.
	retired    chan struct{}
	retireOnce sync.Once
	lifetime   time.Duration
	recheck    time.Duration
	now        func() time.Time
}

// close ends every connection this generation admitted.
func (h *collabHandler) close() {
	h.retireOnce.Do(func() { close(h.retired) })
}

// handleCollab serves GET /api/v1/_collab/{type}/{id}.
//
// Join needs read access: a hidden or missing entity is the uniform 404.
// A principal who may not update the entity joins read-only; the relay
// drops its document updates. The room is the canonical address of the
// row read, so every alias of one entity shares a room. Access is checked
// again every recheck interval while the connection is open.
//
// Every reader receives the whole body. That holds only while `visible:`
// cannot redact the body (see TODO(body-redaction) in
// internal/visibility/policyreader.go); a redactable body needs a room per
// redaction.
func handleCollab(h *collabHandler, w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Co-editing is not enabled", "")
		return
	}
	if !isWebSocketUpgrade(r) {
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "WebSocket upgrade required", "")
		return
	}
	e, ok := readAddressedOr404(w, r, h.reader(), r.PathValue("type"), r.PathValue("id"))
	if !ok {
		return
	}
	writable := h.mayUpdate(r.Context(), e)
	user := principal.From(r.Context()).User
	if !h.relay.acquire(user) {
		writeV1Error(w, r, http.StatusTooManyRequests, "too_many_requests", "Too many co-editing connections", "")
		return
	}
	defer h.relay.release(user)

	lifetime := h.lifetime
	if expiry, has := credentialExpiry(r.Context()); has {
		lifetime = min(lifetime, expiry.Sub(h.now()))
		if lifetime <= 0 {
			writeV1Error(w, r, http.StatusUnauthorized, "unauthorized", "Credential expired", "")
			return
		}
	}
	ctx, cancel := context.WithCancelCause(r.Context())
	defer cancel(nil)
	timer := time.AfterFunc(lifetime, func() {
		cancel(collabCloseError{collabCloseRejoin, "connection lifetime reached"})
	})
	defer timer.Stop()
	go h.watch(ctx, r, e, writable, cancel)

	if refuse := h.refusal(r); refuse != nil {
		// Admit read-only, then close with the code as soon as the upgrade
		// is done: a browser cannot read an HTTP refusal of a WebSocket,
		// only a close code.
		writable = false
		w = &hijackHook{ResponseWriter: w, after: func() { cancel(*refuse) }}
	}

	ctx = context.WithValue(ctx, collabAdmitKey{}, collabAdmission{readOnly: !writable, user: user})
	r = r.WithContext(ctx)
	r.SetPathValue("room", collabRoom(e.Type, e.ID, string(e.Face)))
	h.relay.srv.ServeHTTP(w, r)
}

// collabEpoch is the relay epoch h admits to, or "" without co-editing.
func collabEpoch(h *collabHandler) string {
	if h == nil {
		return ""
	}
	return h.relay.Epoch()
}

// refusal is the close a connection gets right after its upgrade, or nil.
func (h *collabHandler) refusal(r *http.Request) *collabCloseError {
	q := r.URL.Query()
	if build := q.Get("build"); h.relay.build != "" && build != "" && build != h.relay.build {
		return &collabCloseError{collabCloseStaleBuild, "stale client build"}
	}
	if epoch := q.Get("epoch"); epoch != "" && epoch != h.relay.epoch {
		return &collabCloseError{collabCloseGone, "room ended by a server restart"}
	}
	return nil
}

func (h *collabHandler) mayUpdate(ctx context.Context, e *entity.Entity) bool {
	return h.acl().AuthorizeWrite(ctx, translateVerb("update", e.Type, e.ID, e.Face)).Allow
}

// watch ends the connection when the generation retires, or when a recheck
// finds that the principal lost read access or, for a writer, update.
// It returns when ctx ends.
func (h *collabHandler) watch(
	ctx context.Context, r *http.Request, joined *entity.Entity, writable bool, cancel context.CancelCauseFunc,
) {
	tick := time.NewTicker(h.recheck)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.retired:
			cancel(collabCloseError{collabCloseRejoin, "configuration replaced"})
			return
		case <-tick.C:
			if !h.stillAdmitted(r, joined, writable) {
				cancel(collabCloseError{collabCloseRejoin, "access changed"})
				return
			}
		}
	}
}

// stillAdmitted reads the row again through the gated reader, under the
// identity the connection opened with. A read error counts as lost access.
func (h *collabHandler) stillAdmitted(r *http.Request, joined *entity.Entity, writable bool) bool {
	e, ok, err := h.reader().address(r.Context(), joined.Type, r.PathValue("id"))
	if err != nil || !ok || e.ID != joined.ID || e.Face != joined.Face {
		return false
	}
	return !writable || h.mayUpdate(r.Context(), e)
}

// collabRoom names the room of one entity row.
func collabRoom(entityType, id, face string) string {
	if face == "" {
		return fmt.Sprintf("%s/%s", entityType, id)
	}
	return fmt.Sprintf("%s/%s@%s", entityType, id, face)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// hijackHook runs after once the connection is hijacked, that is once the
// WebSocket upgrade has taken over the connection.
type hijackHook struct {
	http.ResponseWriter
	after func()
}

func (h *hijackHook) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(h.ResponseWriter).Hijack()
	if err == nil {
		h.after()
	}
	return conn, rw, err
}

func (h *hijackHook) Unwrap() http.ResponseWriter { return h.ResponseWriter }
