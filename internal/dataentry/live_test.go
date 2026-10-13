package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Live-view tests (TKT-HJ5Z9Y). Each drives the full router over real HTTP
// with real store writes: a stream, a PUT of subscriptions, then writes.

const (
	testUserHeader  = "X-Test-User"
	testRolesHeader = "X-Test-Roles"
	testScopeHeader = "X-Test-Scopes"
)

// liveEnv is a running app with live views and an HTTP server on its router.
type liveEnv struct {
	app *App
	srv *httptest.Server
}

// newLiveEnv builds a test app, lets setup seed it and set its ACL, then
// starts the change feed and the live worker. The principal comes from the
// X-Test-User header; X-Test-Roles or X-Test-Scopes make it a verified
// principal with those comma-separated roles or scopes.
func newLiveEnv(t *testing.T, setup func(app *App)) *liveEnv {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	app.sseTiming = sseTimings{flush: 10 * time.Millisecond, ping: time.Hour}
	app.live.window = 20 * time.Millisecond
	if setup != nil {
		setup(app)
	}
	app.SetPrincipalResolver(func(r *http.Request) principal.Principal {
		user := r.Header.Get(testUserHeader)
		if user == "" {
			user = principal.Unknown
		}
		roles, scopes := r.Header.Get(testRolesHeader), r.Header.Get(testScopeHeader)
		if roles != "" || scopes != "" {
			return principal.VerifiedFrom(user, principal.ToolDataEntry, principal.Claims{
				Roles: splitNonEmpty(roles), Scopes: splitNonEmpty(scopes),
			})
		}
		return principal.Principal{User: user, Tool: principal.ToolDataEntry}
	})
	srv := httptest.NewServer(app.NewRouter())
	t.Cleanup(srv.Close)
	app.startStoreEventBridge()
	t.Cleanup(app.StopWatching)
	return &liveEnv{app: app, srv: srv}
}

// open opens a v1 stream as user and returns it with its connection id.
func (e *liveEnv) open(t *testing.T, user string) (c *sseClient, conn string) {
	t.Helper()
	c = openStream(t, e.srv, "/api/v1/_events", http.Header{testUserHeader: {user}})
	f := c.next(t)
	if f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready", f)
	}
	return c, frameData(t, f)["conn"]
}

// liveSub is one entry of a PUT body.
type liveSubEntry struct {
	Key  string `json:"key"`
	Path string `json:"path"`
}

// put replaces conn's subscriptions as hdr's principal and returns the
// status and body.
func (e *liveEnv) put(t *testing.T, hdr http.Header, conn string, subs ...liveSubEntry) (code int, body string) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"subscriptions": subs})
	if err != nil {
		t.Fatal(err)
	}
	return e.putRaw(t, hdr, "/api/v1/_events/"+conn+"/subscriptions", string(b))
}

func (e *liveEnv) putRaw(t *testing.T, hdr http.Header, path, reqBody string) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, e.srv.URL+path,
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(req.Header, hdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// subscribe PUTs subs as user, requires 204 and waits for the baselines.
func (e *liveEnv) subscribe(t *testing.T, user, conn string, subs ...liveSubEntry) {
	t.Helper()
	if code, body := e.put(t, as(user), conn, subs...); code != http.StatusNoContent {
		t.Fatalf("PUT = %d %s, want 204", code, body)
	}
	e.waitBaselines(t)
}

// get fetches path as user and returns the status and body.
func (e *liveEnv) get(t *testing.T, user, path string) (code int, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, e.srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(testUserHeader, user)
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// waitBaselines waits until every subscription has taken its baseline.
func (e *liveEnv) waitBaselines(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.app.live.allBaselined() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("subscriptions did not take their baselines")
}

// allBaselined reports whether every subscription has a hash.
func (h *liveHub) allBaselined() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.conns {
		c.mu.Lock()
		for _, s := range c.subs {
			if s.hash == "" {
				c.mu.Unlock()
				return false
			}
		}
		c.mu.Unlock()
	}
	return true
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func as(user string) http.Header { return http.Header{testUserHeader: {user}} }

// changedKeys reads n live:changed frames and returns their keys in order.
// entity:changed frames, which every write sends, are skipped.
func changedKeys(t *testing.T, c *sseClient, n int) []string {
	t.Helper()
	keys := make([]string, 0, n)
	for len(keys) < n {
		f := c.next(t)
		switch f.event {
		case "entity:changed":
		case "live:changed":
			keys = append(keys, frameData(t, f)["key"])
		default:
			t.Fatalf("frame = %+v, want live:changed", f)
		}
	}
	return keys
}

// requireChanged reads len(want) live:changed frames and fails unless their
// keys are want, in any order. A round marks its keys one replay at a time,
// so the stream can wake between two marks and split one round's keys over
// two wakes; only the set is promised, not the order.
func requireChanged(t *testing.T, c *sseClient, want ...string) {
	t.Helper()
	got := changedKeys(t, c, len(want))
	slices.Sort(got)
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Fatalf("changed keys = %v, want %v", got, want)
	}
}

// liveQuiet asserts that no live:changed frame arrives within quietWindow.
func liveQuiet(t *testing.T, c *sseClient) {
	t.Helper()
	deadline := time.After(quietWindow)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				return
			}
			if f.event == "live:changed" {
				t.Fatalf("unexpected frame %+v", f)
			}
		case <-deadline:
			return
		}
	}
}

// liveDrain discards frames until the stream has been quiet for quietWindow.
func liveDrain(c *sseClient) {
	for {
		select {
		case _, ok := <-c.frames:
			if !ok {
				return
			}
		case <-time.After(quietWindow):
			return
		}
	}
}

func update(t *testing.T, app *App, e *entity.Entity) {
	t.Helper()
	if err := app.store.UpdateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

// ticket1 is TKT-1 with the given title; every live test watches it.
func ticket1(title string) *entity.Entity {
	return &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": title}}
}

func feature(id, title string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "feature", Properties: map[string]any{"title": title}}
}

// quietWindow is long enough for a round to run and frame.
const quietWindow = 300 * time.Millisecond

// AC2: the PUT accepts the three shapes and refuses everything else.
func TestLive_PutValidation(t *testing.T) {
	env := newLiveEnv(t, nil)
	_, conn := env.open(t, "alice")
	alice := as("alice")

	if code, body := env.put(t, alice, conn,
		liveSubEntry{"e", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"l", "/api/v1/tickets?per_page=20"},
		liveSubEntry{"v", "/api/v1/_views/ticket/TKT-1"},
	); code != http.StatusNoContent {
		t.Fatalf("valid PUT = %d %s", code, body)
	}

	tooMany := make([]liveSubEntry, liveMaxPerConn+1)
	for i := range tooMany {
		tooMany[i] = liveSubEntry{fmt.Sprintf("k%d", i), "/api/v1/tickets"}
	}
	cases := []struct {
		name string
		subs []liveSubEntry
	}{
		{"event stream", []liveSubEntry{{"k", "/api/v1/_events"}}},
		{"export", []liveSubEntry{{"k", "/api/v1/tickets/_export"}}},
		{"document", []liveSubEntry{{"k", "/api/v1/_documents/x"}}},
		{"view with extra segment", []liveSubEntry{{"k", "/api/v1/_views/ticket/a/b"}}},
		{"escaped slash", []liveSubEntry{{"k", "/api/v1/tickets/TKT%2F1"}}},
		{"double slash", []liveSubEntry{{"k", "/api/v1//tickets"}}},
		{"dot dot", []liveSubEntry{{"k", "/api/v1/features/../tickets"}}},
		{"absolute URL", []liveSubEntry{{"k", "http://example.com/api/v1/tickets"}}},
		{"unknown query key", []liveSubEntry{{"k", "/api/v1/tickets?format=csv"}}},
		{"too many", tooMany},
		{"duplicate key", []liveSubEntry{{"k", "/api/v1/tickets"}, {"k", "/api/v1/features"}}},
		{"bad key characters", []liveSubEntry{{"k y", "/api/v1/tickets"}}},
		{"empty key", []liveSubEntry{{"", "/api/v1/tickets"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, body := env.put(t, alice, conn, tc.subs...); code != http.StatusBadRequest {
				t.Fatalf("PUT = %d %s, want 400", code, body)
			}
		})
	}

	t.Run("unknown field", func(t *testing.T) {
		code, _ := env.putRaw(t, alice, "/api/v1/_events/"+conn+"/subscriptions", `{"subs":[]}`)
		if code != http.StatusBadRequest {
			t.Fatalf("PUT = %d, want 400", code)
		}
	})
	t.Run("wrong method", func(t *testing.T) {
		code, _ := env.get(t, "alice", "/api/v1/_events/"+conn+"/subscriptions")
		if code != http.StatusMethodNotAllowed {
			t.Fatalf("GET = %d, want 405", code)
		}
	})
}

// AC3: a write frames every subscription whose response it changed, and only
// those.
func TestLive_WriteFramesChangedSubscriptions(t *testing.T) {
	env := newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, feature("FEAT-1", "f"))
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"detail", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"list", "/api/v1/tickets"},
	)
	liveQuiet(t, c) // taking a baseline sends nothing

	update(t, env.app, ticket1("renamed"))
	requireChanged(t, c, "detail", "list")

	update(t, env.app, feature("FEAT-1", "renamed"))
	liveQuiet(t, c)
}

// aclTicketsOnly gives alice read on tickets and nothing else.
func aclTicketsOnly(t *testing.T) func(app *App) {
	t.Helper()
	return func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, feature("FEAT-1", "hidden"))
		seedRelation(app, entity.NewRelation("TKT-1", "implements", "FEAT-1"))
		setTestACL(t, app, mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			Assignments: map[string]string{"alice": "viewer"},
		}, app.store))
	}
}

// AC4: a write alice cannot read sends her no frame, on any shape, including
// an entity GET of a hidden id. A frame would be sent only if her response
// changed, so this also pins that her responses are stable across rounds.
func TestLive_HiddenWriteSendsNoFrame(t *testing.T) {
	env := newLiveEnv(t, aclTicketsOnly(t))
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"detail", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"list", "/api/v1/tickets"},
		liveSubEntry{"view", "/api/v1/_views/ticket/TKT-1"},
		liveSubEntry{"hidden", "/api/v1/features/FEAT-1"},
		liveSubEntry{"hidden-list", "/api/v1/features"},
	)
	if code, _ := env.get(t, "alice", "/api/v1/features/FEAT-1"); code != http.StatusNotFound {
		t.Fatalf("GET hidden feature = %d, want 404", code)
	}

	update(t, env.app, feature("FEAT-1", "secret rename"))
	write(t, env.app, feature("FEAT-2", "new secret"))
	liveQuiet(t, c)

	// The stream is still live: a visible write frames.
	update(t, env.app, ticket1("renamed"))
	requireChanged(t, c, "detail", "list", "view")
}

// AC5: losing access through a membership change frames the subscription,
// and the refetch is a 404.
func TestLive_LosingAccessFrames(t *testing.T) {
	env := newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
		seedEntity(app, &entity.Entity{ID: "editors", Type: "person", Properties: map[string]any{"title": "Editors"}})
		seedRelation(app, entity.NewRelation("alice", "member-of", "editors"))
		setTestACL(t, app, mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			Assignments: map[string]string{"editors": "viewer"},
		}, app.store))
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})

	k := entity.RelationKey{From: "alice", Type: "member-of", To: "editors"}
	if err := env.app.store.DeleteRelation(context.Background(), k); err != nil {
		t.Fatal(err)
	}
	if got := changedKeys(t, c, 1); got[0] != "detail" {
		t.Fatalf("changed keys = %v, want [detail]", got)
	}
	if code, _ := env.get(t, "alice", "/api/v1/tickets/TKT-1"); code != http.StatusNotFound {
		t.Fatalf("GET after losing access = %d, want 404", code)
	}
}

// AC6: a view frames when an entity it shows through a traversal changes.
func TestLive_ViewFramesOnRelatedChange(t *testing.T) {
	env := newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, feature("FEAT-1", "f"))
		seedRelation(app, entity.NewRelation("TKT-1", "implements", "FEAT-1"))
		app.State().Cfg.Views["ticket_detail"] = ViewConfig{
			Title:    "Ticket",
			Entry:    ViewEntry{Type: "ticket"},
			Traverse: []ViewTraverse{{From: "entry", Follow: "implements", CollectAs: "features"}},
			Sections: []ViewSection{{
				Heading: "Features", Source: "features", Display: "table",
				Columns: []ListColumn{{Property: "title"}},
			}},
		}
	})
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn, liveSubEntry{"view", "/api/v1/_views/ticket/TKT-1"})

	update(t, env.app, feature("FEAT-1", "renamed"))
	if got := changedKeys(t, c, 1); got[0] != "view" {
		t.Fatalf("changed keys = %v, want [view]", got)
	}
}

// AC7: only the stream's own principal may set its subscriptions.
func TestLive_PutOwnership(t *testing.T) {
	env := newLiveEnv(t, nil)
	_, conn := env.open(t, "alice")
	sub := liveSubEntry{"k", "/api/v1/tickets"}

	cases := []struct {
		name string
		hdr  http.Header
		conn string
		want int
	}{
		{"another user", as("bob"), conn, http.StatusNotFound},
		{"unknown id", as("alice"), "AAAAAAAAAAAAAAAAAAAAAA", http.StatusNotFound},
		{"changed claims", http.Header{testUserHeader: {"alice"}, testRolesHeader: {"admin"}}, conn, http.StatusConflict},
		{"owner", as("alice"), conn, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, body := env.put(t, tc.hdr, tc.conn, sub); code != tc.want {
				t.Fatalf("PUT = %d %s, want %d", code, body, tc.want)
			}
		})
	}
}

// The per-identity cap counts subscriptions across all of a user's streams.
func TestLive_PutIdentityCap(t *testing.T) {
	env := newLiveEnv(t, nil)
	full := make([]liveSubEntry, liveMaxPerConn)
	for i := range full {
		full[i] = liveSubEntry{fmt.Sprintf("k%d", i), "/api/v1/tickets"}
	}
	for i := range liveMaxPerIdentity / liveMaxPerConn {
		_, conn := env.open(t, "alice")
		if code, body := env.put(t, as("alice"), conn, full...); code != http.StatusNoContent {
			t.Fatalf("stream %d: PUT = %d %s", i, code, body)
		}
	}
	_, conn := env.open(t, "alice")
	if code, _ := env.put(t, as("alice"), conn, full[0]); code != http.StatusTooManyRequests {
		t.Fatalf("PUT past the identity cap = %d, want 429", code)
	}
	_, other := env.open(t, "bob")
	if code, _ := env.put(t, as("bob"), other, full[0]); code != http.StatusNoContent {
		t.Fatalf("another user's PUT = %d, want 204", code)
	}

	// Without authentication every caller is the same unknown user, so the
	// identity cap would be shared by everyone; only the per-stream cap holds.
	for i := range liveMaxPerIdentity/liveMaxPerConn + 1 {
		_, conn := env.open(t, "")
		if code, body := env.put(t, as(""), conn, full...); code != http.StatusNoContent {
			t.Fatalf("unauthenticated stream %d: PUT = %d %s", i, code, body)
		}
	}
}

// A closed stream's id is gone: its PUT is a 404.
func TestLive_ClosedStreamForgotten(t *testing.T) {
	env := newLiveEnv(t, nil)
	c, conn := env.open(t, "alice")
	c.close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		code, _ := env.put(t, as("alice"), conn, liveSubEntry{"k", "/api/v1/tickets"})
		if code == http.StatusNotFound {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PUT to a closed stream = %d, want 404", code)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The legacy stream has no connection id.
func TestLive_LegacyStreamHasNoConn(t *testing.T) {
	env := newLiveEnv(t, nil)
	c := openStream(t, env.srv, "/api/events", as("alice"))
	f := c.next(t)
	if f.event != "ready" || f.data != "{}" {
		t.Fatalf("legacy ready = %+v, want empty data", f)
	}
}

// AC10: a replay hashes exactly what the same principal's GET returns, for
// row gating, a redacted field, a world parameter and a principal resolved
// through principal_property.
func TestLive_ReplayMatchesGet(t *testing.T) {
	redacted := func(app *App) {
		aclTicketsOnly(t)(app)
		app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"status": false}}}
		update(t, app, &entity.Entity{ID: "TKT-1", Type: "ticket",
			Properties: map[string]any{"title": "one", "status": "SECRET-STATUS"}})
	}
	byEmail := func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, &entity.Entity{ID: "PERS-alice", Type: "person",
			Properties: map[string]any{"email": "alice@example.com"}})
		d, err := acl.NewDeclarative(&acl.Policy{
			UserEntityType:    "person",
			PrincipalProperty: "email",
			Roles:             map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			Assignments:       map[string]string{"PERS-alice": "viewer"},
		}, acl.NewStoreGraph(app.store), app.store,
			acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(app.store)))
		if err != nil {
			t.Fatal(err)
		}
		setTestACL(t, app, d)
	}
	cases := []struct {
		name  string
		setup func(app *App)
		user  string
		paths map[string]int // path → the GET status that makes the case meaningful
	}{
		{"row gating", aclTicketsOnly(t), "alice", map[string]int{
			"/api/v1/tickets/TKT-1":       http.StatusOK,
			"/api/v1/tickets":             http.StatusOK,
			"/api/v1/_views/ticket/TKT-1": http.StatusOK,
			"/api/v1/features/FEAT-1":     http.StatusNotFound,
		}},
		{"redacted field", redacted, "alice", map[string]int{
			"/api/v1/tickets/TKT-1": http.StatusOK,
			"/api/v1/tickets":       http.StatusOK,
		}},
		{"world parameter", aclTicketsOnly(t), "alice", map[string]int{
			"/api/v1/tickets/TKT-1?world=nope": 0,
		}},
		{"principal_property", byEmail, "alice@example.com", map[string]int{
			"/api/v1/tickets/TKT-1": http.StatusOK,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newLiveEnv(t, tc.setup)
			handler := *env.app.live.replay.Load()
			p := principal.Principal{User: tc.user, Tool: principal.ToolDataEntry}
			for path, wantCode := range tc.paths {
				code, body := env.get(t, tc.user, path)
				if wantCode != 0 && code != wantCode {
					t.Fatalf("GET %s = %d, want %d: %s", path, code, wantCode, body)
				}
				if strings.Contains(string(body), "SECRET") {
					t.Fatalf("GET %s leaks a redacted value: %s", path, body)
				}
				want := fmt.Sprintf("%d:%x", code, sha256.Sum256(body))
				res := replayHash(context.Background(), handler, p, path)
				got, ok := res.hash, res.ok
				if !ok || got != want {
					t.Fatalf("%s: replay hash = %q (ok %v), GET hash = %q\nGET body: %s", path, got, ok, want, body)
				}
			}
		})
	}
}

// A principal resolved through principal_property gets frames: the replay
// starts from the pre-resolution identity and resolves it like a request.
func TestLive_PrincipalPropertyFrames(t *testing.T) {
	env := newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, &entity.Entity{ID: "PERS-alice", Type: "person",
			Properties: map[string]any{"email": "alice@example.com"}})
		d, err := acl.NewDeclarative(&acl.Policy{
			UserEntityType:    "person",
			PrincipalProperty: "email",
			Roles:             map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			Assignments:       map[string]string{"PERS-alice": "viewer"},
		}, acl.NewStoreGraph(app.store), app.store,
			acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(app.store)))
		if err != nil {
			t.Fatal(err)
		}
		setTestACL(t, app, d)
	})
	c, conn := env.open(t, "alice@example.com")
	env.subscribe(t, "alice@example.com", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	update(t, env.app, ticket1("renamed"))
	if got := changedKeys(t, c, 1); got[0] != "detail" {
		t.Fatalf("changed keys = %v, want [detail]", got)
	}
}

// countReplays wraps the installed replay handler with a counter.
func countReplays(app *App) *atomic.Int64 {
	var n atomic.Int64
	inner := *app.live.replay.Load()
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		inner.ServeHTTP(w, r)
	})
	app.live.replay.Store(&h)
	return &n
}

// holdReplays blocks every replay until the returned release is called.
func holdReplays(app *App) (release func()) {
	gate := make(chan struct{})
	inner := *app.live.replay.Load()
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-gate
		inner.ServeHTTP(w, r)
	})
	app.live.replay.Store(&h)
	return sync.OnceFunc(func() { close(gate) })
}

// AC9: one round replays each distinct (principal, path) once, however many
// writes it coalesces and however many streams share the subscription.
// Principals that differ only in scopes do not share.
func TestLive_ReplaySharing(t *testing.T) {
	env := newLiveEnv(t, func(app *App) {
		app.live.window = 200 * time.Millisecond
		seedEntity(app, ticket1("one"))
	})
	sub := liveSubEntry{"list", "/api/v1/tickets"}
	c1, conn1 := env.open(t, "alice")
	c2, conn2 := env.open(t, "alice")
	env.subscribe(t, "alice", conn1, sub)
	env.subscribe(t, "alice", conn2, sub)
	replays := countReplays(env.app)

	update(t, env.app, ticket1("a"))
	changedKeys(t, c1, 1)
	changedKeys(t, c2, 1)
	if got := replays.Load(); got != 1 {
		t.Fatalf("one write, two streams: %d replays, want 1", got)
	}

	replays.Store(0)
	// Rounds run one at a time, so holding replays until the burst ends
	// keeps a slow machine from spreading the burst over more ticks.
	release := holdReplays(env.app)
	for i := range 20 {
		update(t, env.app, ticket1(fmt.Sprintf("b%d", i)))
	}
	release()
	changedKeys(t, c1, 1)
	changedKeys(t, c2, 1)
	// A round may start before the burst and be held mid-burst, so the
	// burst takes at most two rounds.
	if got := replays.Load(); got > 2 {
		t.Fatalf("20 writes in a burst: %d replays, want at most 2", got)
	}
	liveDrain(c1)
	liveDrain(c2)

	scoped := http.Header{testUserHeader: {"alice"}, testScopeHeader: {"read"}}
	c3 := openStream(t, env.srv, "/api/v1/_events", scoped)
	conn3 := frameData(t, c3.next(t))["conn"]
	if code, body := env.put(t, scoped, conn3, sub); code != http.StatusNoContent {
		t.Fatalf("scoped PUT = %d %s", code, body)
	}
	env.waitBaselines(t)
	replays.Store(0)
	update(t, env.app, ticket1("c"))
	changedKeys(t, c1, 1)
	changedKeys(t, c3, 1)
	if got := replays.Load(); got != 2 {
		t.Fatalf("principals differing in scopes: %d replays, want 2", got)
	}
}
