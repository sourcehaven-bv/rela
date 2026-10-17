package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/presence"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Presence tests (TKT-MHXH87). The HTTP ones drive the full router with a
// fake presence clock; the real ticker still runs the rounds.

// fakeClock is a presence clock the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

var presenceT0 = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func newFakeClock() *fakeClock { return &fakeClock{t: presenceT0} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newPresenceEnv is newLiveEnv with a fake presence clock.
func newPresenceEnv(t *testing.T, setup func(app *App)) (*liveEnv, *fakeClock) {
	t.Helper()
	clk := newFakeClock()
	env := newLiveEnv(t, func(app *App) {
		app.live.now = clk.Now
		if setup != nil {
			setup(app)
		}
	})
	return env, clk
}

func presencePath(plural, id string) string { return "/api/v1/" + plural + "/" + id + "/_presence" }

// presenceOf GETs path as user and requires 200.
func presenceOf(t *testing.T, env *liveEnv, user, path string) v1.PresenceResponse {
	t.Helper()
	code, b := env.get(t, user, path)
	if code != http.StatusOK {
		t.Fatalf("GET %s as %s = %d %s", path, user, code, b)
	}
	var resp v1.PresenceResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return resp
}

func viewerUsers(resp v1.PresenceResponse) []string {
	out := make([]string, len(resp.Viewers))
	for i, v := range resp.Viewers {
		out[i] = v.User
	}
	return out
}

// waitViewers polls path as user until the viewers are want.
func waitViewers(t *testing.T, env *liveEnv, user, path string, want ...string) v1.PresenceResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp := presenceOf(t, env, user, path)
		got := viewerUsers(resp)
		if slices.Equal(got, want) {
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatalf("viewers of %s for %s = %v, want %v", path, user, got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitTicks waits until the live worker has finished n more ticks, the
// round of each included. The first may have started before the call, so
// n=2 covers one whole tick.
func (e *liveEnv) waitTicks(t *testing.T, n uint64) {
	t.Helper()
	target := e.app.live.ticks.Load() + n
	deadline := time.Now().Add(2 * time.Second)
	for e.app.live.ticks.Load() < target {
		if time.Now().After(deadline) {
			t.Fatalf("the live worker did not finish %d ticks", n)
		}
		time.Sleep(time.Millisecond)
	}
}

// presenceFrameSlack covers delivery of a frame a round already marked: the
// stream writes it as soon as it wakes, so this is transport time only.
const presenceFrameSlack = 50 * time.Millisecond

// presenceQuiet asserts that c gets no live:changed frame from the next two
// whole ticks.
func presenceQuiet(t *testing.T, env *liveEnv, c *sseClient) {
	t.Helper()
	env.waitTicks(t, 3)
	deadline := time.After(presenceFrameSlack)
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

// AC1: every miss is the entity GET's 404, and a live subscription to it
// records nobody. A wrong method is a 405.
func TestPresence_MissIsTheEntityGet404(t *testing.T) {
	env, _ := newPresenceEnv(t, aclTicketsOnly(t))
	cases := []struct {
		name, plural, id, query string
		subscribable            bool // a PUT accepts the path
	}{
		{"hidden", "features", "FEAT-1", "", true},
		{"absent", "tickets", "TKT-404", "", true},
		{"wrong plural", "features", "TKT-1", "", true},
		{"hidden in a world", "features", "FEAT-1", "?world=default", true},
		{"malformed address", "tickets", "TKT-1@", "", false},
		{"invalid face", "tickets", "TKT-1@-bad", "", false},
	}
	_, conn := env.open(t, "alice")
	subs := make([]liveSubEntry, 0, len(cases))
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, got := env.get(t, "alice", presencePath(tc.plural, tc.id)+tc.query)
			wantCode, want := env.get(t, "alice", "/api/v1/"+tc.plural+"/"+tc.id+tc.query)
			requireUniform404(t, code, got, wantCode, want)
		})
		if tc.subscribable {
			subs = append(subs, liveSubEntry{fmt.Sprintf("p%d", i), presencePath(tc.plural, tc.id) + tc.query})
		}
	}
	env.subscribe(t, "alice", conn, subs...)
	env.waitTicks(t, 2)
	if n := env.app.live.presence.Len(); n != 0 {
		t.Fatalf("tracker holds %d entries after subscriptions to misses, want 0", n)
	}

	res, _ := env.do(t, "alice", http.MethodPost, presencePath("tickets", "TKT-1"), "", nil)
	if res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != http.MethodGet {
		t.Fatalf("POST _presence = %d Allow=%q, want 405 GET", res.StatusCode, res.Header.Get("Allow"))
	}
	res, _ = env.do(t, "alice", http.MethodGet, presencePath("tickets", "TKT-1"), "", nil)
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
}

// AC2: a join frames the other subscribers; a leave after close or
// unsubscribe takes the grace period on the fake clock; a rejoin within
// grace keeps since. A plain GET never joins.
func TestPresence_JoinAndGraceLeave(t *testing.T) {
	env, clk := newPresenceEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	path := presencePath("tickets", "TKT-1")
	alice, aliceConn := env.open(t, "alice")
	env.subscribe(t, "alice", aliceConn, liveSubEntry{"p", path})
	waitViewers(t, env, "bob", path, "alice")
	presenceQuiet(t, env, alice) // nobody else joined: alice's response did not move

	// A plain GET by carol does not join.
	presenceOf(t, env, "carol", path)
	env.waitTicks(t, 2)
	waitViewers(t, env, "alice", path)

	bob, bobConn := env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", path})
	if got := changedKeys(t, alice, 1); got[0] != "p" {
		t.Fatalf("alice's frames = %v, want [p]", got)
	}
	resp := waitViewers(t, env, "alice", path, "bob")
	since := resp.Viewers[0].Since
	if since != "2026-10-12T10:00:00Z" || resp.Viewers[0].Tool != principal.ToolDataEntry || resp.More != 0 {
		t.Fatalf("viewer = %+v more=%d", resp.Viewers[0], resp.More)
	}

	// Close: bob stays for the grace period.
	bob.close()
	clk.advance(10 * time.Second)
	presenceQuiet(t, env, alice)
	waitViewers(t, env, "alice", path, "bob")

	// Rejoin within grace keeps since and sends nothing.
	_, bobConn = env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", path})
	clk.advance(10 * time.Second)
	presenceQuiet(t, env, alice)
	if got := waitViewers(t, env, "alice", path, "bob").Viewers[0].Since; got != since {
		t.Fatalf("since after a rejoin within grace = %s, want %s", got, since)
	}

	// Unsubscribe: gone at 15 s, not before.
	env.subscribe(t, "bob", bobConn)
	clk.advance(presence.Grace - time.Second)
	presenceQuiet(t, env, alice)
	waitViewers(t, env, "alice", path, "bob")
	clk.advance(time.Second)
	if got := changedKeys(t, alice, 1); got[0] != "p" {
		t.Fatalf("alice's frames = %v, want [p]", got)
	}
	waitViewers(t, env, "alice", path)
}

// AC3: losing read access removes the viewer at once, with no grace; the
// revoked viewer and the remaining one both get a frame.
func TestPresence_RevocationLeavesAtOnce(t *testing.T) {
	env, _ := newPresenceEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		for _, id := range []string{"alice", "bob", "editors"} {
			seedEntity(app, &entity.Entity{ID: id, Type: "person", Properties: map[string]any{"title": id}})
		}
		seedRelation(app, entity.NewRelation("alice", "member-of", "editors"))
		seedRelation(app, entity.NewRelation("bob", "member-of", "editors"))
		app.acl = mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			Assignments: map[string]string{"editors": "viewer"},
		}, app.store)
	})
	path := presencePath("tickets", "TKT-1")
	alice, aliceConn := env.open(t, "alice")
	bob, bobConn := env.open(t, "bob")
	env.subscribe(t, "alice", aliceConn, liveSubEntry{"p", path})
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", path})
	waitViewers(t, env, "alice", path, "bob")
	waitViewers(t, env, "bob", path, "alice")
	liveDrain(alice)
	liveDrain(bob)

	k := entity.RelationKey{From: "bob", Type: "member-of", To: "editors"}
	if err := env.app.store.DeleteRelation(context.Background(), k); err != nil {
		t.Fatal(err)
	}
	if got := changedKeys(t, bob, 1); got[0] != "p" {
		t.Fatalf("bob's frames = %v, want [p]", got)
	}
	if got := changedKeys(t, alice, 1); got[0] != "p" {
		t.Fatalf("alice's frames = %v, want [p]", got)
	}
	waitViewers(t, env, "alice", path)
	if n := env.app.live.presence.Len(); n != 1 {
		t.Fatalf("tracker holds %d entries, want alice's only", n)
	}
}

// AC4: a presence change on one face replays only the subscriptions on that
// face. A subscriber who cannot read the entity gets no frame and no replay.
func TestPresence_RoundReplaysOnlyThatFace(t *testing.T) {
	env, _ := newPresenceEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, feature("FEAT-1", "hidden"))
		app.acl = mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"viewer": {Read: []string{"ticket"}},
				"admin":  {Read: []string{"ticket", "feature"}},
			},
			Assignments: map[string]string{"alice": "viewer", "bob": "admin", "dave": "admin"},
		}, app.store)
	})
	feat, tkt := presencePath("features", "FEAT-1"), presencePath("tickets", "TKT-1")
	alice, aliceConn := env.open(t, "alice")
	env.subscribe(t, "alice", aliceConn,
		liveSubEntry{"hidden", feat}, liveSubEntry{"p", tkt}, liveSubEntry{"list", "/api/v1/tickets"})
	_, bobConn := env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", feat})
	waitViewers(t, env, "dave", feat, "bob")
	// The tick that published bob, and the presence round it ran, are
	// over before the counting handler goes in.
	env.waitTicks(t, 2)

	var mu sync.Mutex
	var replayed []string
	inner := *env.app.live.replay.Load()
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		replayed = append(replayed, principal.From(r.Context()).User+" "+r.URL.Path)
		mu.Unlock()
		inner.ServeHTTP(w, r)
	})
	env.app.live.replay.Store(&h)

	_, daveConn := env.open(t, "dave")
	env.subscribe(t, "dave", daveConn, liveSubEntry{"p", feat})
	waitViewers(t, env, "bob", tkt, "alice")
	waitViewers(t, env, "dave", feat, "bob")
	waitViewers(t, env, "bob", feat, "dave")
	presenceQuiet(t, env, alice)

	mu.Lock()
	defer mu.Unlock()
	if len(replayed) == 0 {
		t.Fatal("no replay ran")
	}
	for _, r := range replayed {
		if r != "bob "+feat && r != "dave "+feat {
			t.Errorf("a presence change on FEAT-1 replayed %q", r)
		}
	}
}

// Presence is per face, so a bare id of a faced type is the write path's
// 422 face_required, naming the faces, once the read gate admitted the
// entity. A caller who cannot read it gets the entity GET's 404 instead.
func TestPresence_FacedTypeRequiresAFace(t *testing.T) {
	get := func(app *App, d *acl.Declarative, path string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, d)))
		return rec
	}

	app, d, _ := facedAppWithPolicy(t, acl.RoleDef{Read: allTicketFaces})
	rec := get(app, d, presencePath("tickets", "TKT-1"))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "face_required") ||
		!strings.Contains(rec.Body.String(), "TKT-1@draft") {

		t.Fatalf("bare presence = %d %s, want 422 face_required naming the faces", rec.Code, rec.Body)
	}
	if rec := get(app, d, presencePath("tickets", "TKT-1@draft")); rec.Code != http.StatusOK {
		t.Fatalf("faced presence = %d %s, want 200", rec.Code, rec.Body)
	}

	hidden, hd, _ := facedAppWithPolicy(t, acl.RoleDef{Read: []string{"feature"}})
	for _, addr := range []string{"TKT-1", "TKT-1@draft"} {
		got := get(hidden, hd, presencePath("tickets", addr))
		want := get(hidden, hd, "/api/v1/tickets/"+addr)
		requireUniform404(t, got.Code, got.Body.Bytes(), want.Code, want.Body.Bytes())
	}
}

// AC5: two tabs of one identity are one row; the caller never sees itself,
// from any tool; past 20 viewers the rest are counted in more.
func TestPresence_MergeExcludeAndCap(t *testing.T) {
	env, clk := newPresenceEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	path := presencePath("tickets", "TKT-1")
	for range 2 {
		_, conn := env.open(t, "alice")
		env.subscribe(t, "alice", conn, liveSubEntry{"p", path})
	}
	waitViewers(t, env, "bob", path, "alice")

	tr := env.app.live.presence
	key := presenceKey(ticket1("one"))
	desktop := presence.Identity{User: "alice", Tool: principal.ToolDesktop}
	tr.Join(key, presence.Slot{Conn: "desk", Sub: "p"}, desktop, clk.Now().Add(time.Second))
	for i := range 23 {
		who := presence.Identity{User: fmt.Sprintf("u%02d", i), Tool: principal.ToolDataEntry}
		tr.Join(key, presence.Slot{Conn: who.User, Sub: "p"}, who, clk.Now().Add(2*time.Second))
	}
	tr.TakeChanged(clk.Now())

	cases := []struct {
		name, user string
		first      []string
		more       int
	}{
		// alice (data entry, two tabs), alice (desktop), u00..u17; 25 in all.
		{"observer", "zed", []string{"alice", "alice", "u00"}, 5},
		// alice sees neither of her own rows; 23 others.
		{"self from either tool", "alice", []string{"u00", "u01", "u02"}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := presenceOf(t, env, tc.user, path)
			got := viewerUsers(resp)
			if len(got) != presenceViewerCap || resp.More != tc.more || !slices.Equal(got[:3], tc.first) {
				t.Fatalf("viewers = %v more=%d, want %d rows starting %v and more=%d",
					got, resp.More, presenceViewerCap, tc.first, tc.more)
			}
			if tc.user == "zed" && (resp.Viewers[0].Tool != principal.ToolDataEntry ||
				resp.Viewers[1].Tool != principal.ToolDesktop) {

				t.Fatalf("first rows = %+v, want alice's data-entry then desktop entry", resp.Viewers[:2])
			}
		})
	}
}

// AC6: the anonymous principal never joins, but still gets a frame when
// someone else does.
func TestPresence_UnknownNeverJoins(t *testing.T) {
	env, _ := newPresenceEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
	path := presencePath("tickets", "TKT-1")
	anon, conn := env.open(t, "")
	env.subscribe(t, "", conn, liveSubEntry{"p", path})
	env.waitTicks(t, 2)
	if n := env.app.live.presence.Len(); n != 0 {
		t.Fatalf("tracker holds %d entries for the anonymous principal, want 0", n)
	}

	_, bobConn := env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", path})
	if got := changedKeys(t, anon, 1); got[0] != "p" {
		t.Fatalf("anonymous frames = %v, want [p]", got)
	}
	waitViewers(t, env, "", path, "bob")
}

// AC7: retiring an app drops its presence with it; on the new app a
// reconnect rejoins.
func TestPresence_RetireDiscards(t *testing.T) {
	seed := func(app *App) { seedEntity(app, ticket1("one")) }
	path := presencePath("tickets", "TKT-1")
	old, _ := newPresenceEnv(t, seed)
	stream, conn := old.open(t, "alice")
	old.subscribe(t, "alice", conn, liveSubEntry{"p", path})
	waitViewers(t, old, "bob", path, "alice")

	fresh, _ := newPresenceEnv(t, seed)
	Retire(old.app)
	deadline := time.After(2 * time.Second)
	for closed := false; !closed; {
		select {
		case _, ok := <-stream.frames:
			closed = !ok
		case <-deadline:
			t.Fatal("Retire did not close the stream")
		}
	}
	waitViewers(t, fresh, "bob", path)

	_, conn = fresh.open(t, "alice")
	fresh.subscribe(t, "alice", conn, liveSubEntry{"p", path})
	waitViewers(t, fresh, "bob", path, "alice")
}

// AC10: a presence replay hashes exactly what the same principal's GET
// returns.
func TestPresence_ReplayMatchesGet(t *testing.T) {
	env, clk := newPresenceEnv(t, aclTicketsOnly(t))
	// A viewer for alice to see, so the 200 is not an empty list.
	bob := presence.Identity{User: "bob", Tool: principal.ToolDataEntry}
	env.app.live.presence.Join(presenceKey(ticket1("one")), presence.Slot{Conn: "c", Sub: "p"}, bob, clk.Now())
	env.app.live.presence.TakeChanged(clk.Now())
	waitViewers(t, env, "alice", presencePath("tickets", "TKT-1"), "bob")
	handler := *env.app.live.replay.Load()
	p := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	for path, wantCode := range map[string]int{
		presencePath("tickets", "TKT-1"):   http.StatusOK,
		presencePath("features", "FEAT-1"): http.StatusNotFound,
	} {
		code, body := env.get(t, "alice", path)
		if code != wantCode {
			t.Fatalf("GET %s = %d, want %d", path, code, wantCode)
		}
		res := replayHash(context.Background(), handler, p, path)
		if want := hashOf(code, body); !res.ok || res.hash != want {
			t.Fatalf("%s: replay hash %q, GET hash %q", path, res.hash, want)
		}
	}
}

// hashOf is the replay hash of a response with code and body.
func hashOf(code int, body []byte) string { return fmt.Sprintf("%d:%x", code, sha256.Sum256(body)) }

// AC8: a view's entry carries `_lease` while held, and loses it on release
// and on expiry. A claim frames a live subscription to the view.
func TestPresence_ViewEntryLease(t *testing.T) {
	env := newLeaseEnv(t)
	view := "/api/v1/_views/ticket/TKT-1"
	entryLease := func() map[string]any {
		t.Helper()
		code, b := env.get(t, "carol", view)
		if code != http.StatusOK {
			t.Fatalf("GET view = %d %s", code, b)
		}
		var v struct {
			Entry json.RawMessage `json:"entry"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return leaseOf(t, v.Entry)
	}
	if l := entryLease(); l != nil {
		t.Fatalf("unheld view entry _lease = %v", l)
	}
	carol, conn := env.open(t, "carol")
	env.subscribe(t, "carol", conn, liveSubEntry{"view", view})

	c := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"status":"busy"}`)
	if got := changedKeys(t, carol, 1); got[0] != "view" {
		t.Fatalf("frames = %v, want [view]", got)
	}
	if l := entryLease(); l["user"] != "alice" || l["status"] != "busy" {
		t.Fatalf("held view entry _lease = %v", l)
	}
	if res, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(c.LeaseID), nil); res.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d %s", res.StatusCode, b)
	}
	if l := entryLease(); l != nil {
		t.Fatalf("view entry _lease after release = %v", l)
	}

	c = env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"ttl_seconds":30}`)
	exp, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	env.clock.shiftTo(exp.Add(time.Second))
	if l := entryLease(); l != nil {
		t.Fatalf("view entry _lease after expiry = %v", l)
	}
}

// S2: churning subscriptions by one identity never hold more than
// presence.MaxPerIdentity entries, in-grace ones included, and never block
// another identity. A subscription the cap refused still gets a frame when
// someone else joins its face.
func TestPresence_ChurnByOneIdentityIsCapped(t *testing.T) {
	const rounds, perPut = 5, liveMaxPerConn
	env, _ := newPresenceEnv(t, func(app *App) {
		for i := range rounds * perPut {
			seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%d", i+1), Type: "ticket",
				Properties: map[string]any{"title": "t"}})
		}
	})
	alice := presence.Identity{User: "alice", Tool: principal.ToolDataEntry}
	aliceStream, conn := env.open(t, "alice")
	for r := range rounds {
		subs := make([]liveSubEntry, perPut)
		for i := range subs {
			subs[i] = liveSubEntry{fmt.Sprintf("p%d", i), presencePath("tickets", fmt.Sprintf("TKT-%d", r*perPut+i+1))}
		}
		env.subscribe(t, "alice", conn, subs...)
		if n := env.app.live.presence.Count(alice); n > presence.MaxPerIdentity {
			t.Fatalf("after PUT %d alice holds %d entries, cap %d", r, n, presence.MaxPerIdentity)
		}
	}
	if n := env.app.live.presence.Count(alice); n != presence.MaxPerIdentity {
		t.Fatalf("alice holds %d entries, want the cap %d", n, presence.MaxPerIdentity)
	}
	liveDrain(aliceStream)
	_, bobConn := env.open(t, "bob")
	env.subscribe(t, "bob", bobConn, liveSubEntry{"p", presencePath("tickets", "TKT-80")})
	waitViewers(t, env, "carol", presencePath("tickets", "TKT-80"), "bob")
	// alice's p15 watches TKT-80, which the cap kept her off.
	if got := changedKeys(t, aliceStream, 1); got[0] != "p15" {
		t.Fatalf("alice's frames = %v, want [p15]", got)
	}
	if n := env.app.live.presence.Count(alice); n != presence.MaxPerIdentity {
		t.Fatalf("alice holds %d entries after the retry, want the cap %d", n, presence.MaxPerIdentity)
	}
}

// --- hub-level: the commit rules (S1, M2, M4) ---

// probeHandler answers status for every replay and records the entity id
// in the probe, as the presence route does. When gate is non-nil, the
// replay waits on it after signaling entered.
func probeHandler(id string, status int, entered, gate chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordPresence(r.Context(), &entity.Entity{ID: id, Type: "ticket"})
		if gate != nil {
			entered <- struct{}{}
			<-gate
		}
		w.WriteHeader(status)
	})
}

var hubAlice = principal.Principal{User: "alice", Tool: principal.ToolDataEntry}

// presenceSub is an added presence subscription on TKT-1 under key "p".
func presenceSub() keyedSub {
	return keyedSub{key: "p", sub: &liveSub{
		url: "/api/v1/tickets/TKT-1/_presence", presenceID: "TKT-1", syncing: true,
	}}
}

func addSub(c *liveConn, ks keyedSub) {
	c.mu.Lock()
	c.subs[ks.key] = ks.sub
	c.mu.Unlock()
}

// S1: a replay that outlives its subscription or its stream joins nothing.
func TestLiveHub_StaleReplayJoinsNothing(t *testing.T) {
	cases := []struct {
		name string
		run  func(h *liveHub, c *liveConn, ks keyedSub, replay func(func()))
	}{
		{"PUT drops the subscription during the baseline", func(h *liveHub, c *liveConn, ks keyedSub, replay func(func())) {
			replay(func() { h.baseline(context.Background(), c, []keyedSub{ks}) })
		}},
		{"stream closes between collect and commit", func(h *liveHub, _ *liveConn, ks keyedSub, replay func(func())) {
			ks.sub.syncing = false
			replay(func() { h.round(context.Background(), true) })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entered, gate := make(chan struct{}), make(chan struct{})
			h := hubWith(probeHandler("TKT-1", http.StatusOK, entered, gate))
			c := h.register(hubAlice)
			ks := presenceSub()
			addSub(c, ks)
			replay := func(start func()) {
				done := make(chan struct{})
				go func() { defer close(done); start() }()
				<-entered
				if strings.HasPrefix(tc.name, "PUT") {
					if _, ok := h.replace(c, map[string]liveTarget{}); !ok {
						t.Error("replace refused")
					}
				} else {
					h.unregister(c)
				}
				close(gate)
				<-done
			}
			tc.run(h, c, ks, replay)
			if n := h.presence.Len(); n != 0 {
				t.Fatalf("tracker holds %d entries, want 0", n)
			}
		})
	}

	t.Run("control: a current subscription joins", func(t *testing.T) {
		h := hubWith(probeHandler("TKT-1", http.StatusOK, nil, nil))
		c := h.register(hubAlice)
		ks := presenceSub()
		addSub(c, ks)
		h.baseline(context.Background(), c, []keyedSub{ks})
		if n := h.presence.Len(); n != 1 {
			t.Fatalf("tracker holds %d entries, want 1", n)
		}
	})
}

// M4: the probe is honored only for a presence subscription whose replay
// answered 200 and whose probe names the path's entity.
func TestLiveHub_ProbeHonouredOnlyForPresence(t *testing.T) {
	cases := []struct {
		name       string
		url, id    string // the subscription's
		recordID   string // what the handler writes in the probe
		status     int
		wantJoined bool
	}{
		{"presence 200 same id", "/api/v1/tickets/TKT-1/_presence", "TKT-1", "TKT-1", http.StatusOK, true},
		{"entity shape writing a probe", "/api/v1/tickets/TKT-1", "", "TKT-1", http.StatusOK, false},
		{"view shape writing a probe", "/api/v1/_views/ticket/TKT-1", "", "TKT-1", http.StatusOK, false},
		{"probe names another entity", "/api/v1/tickets/TKT-1/_presence", "TKT-1", "TKT-2", http.StatusOK, false},
		{"not 200", "/api/v1/tickets/TKT-1/_presence", "TKT-1", "TKT-1", http.StatusNotFound, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := hubWith(probeHandler(tc.recordID, tc.status, nil, nil))
			c := h.register(hubAlice)
			ks := keyedSub{key: "k", sub: &liveSub{url: tc.url, presenceID: tc.id, syncing: true}}
			addSub(c, ks)
			h.baseline(context.Background(), c, []keyedSub{ks})
			if got := h.presence.Len() == 1; got != tc.wantJoined {
				t.Fatalf("joined = %v, want %v", got, tc.wantJoined)
			}
		})
	}

	t.Run("probe without a probe-carrying request is ignored", func(*testing.T) {
		recordPresence(context.Background(), &entity.Entity{ID: "TKT-1"})
	})
}

// joinedHub is a hub with alice's presence subscription on TKT-1 joined
// and published. It returns the face.
func joinedHub(t *testing.T) (*liveHub, *liveConn, string) {
	t.Helper()
	h := hubWith(probeHandler("TKT-1", http.StatusOK, nil, nil))
	c := h.register(hubAlice)
	ks := presenceSub()
	addSub(c, ks)
	h.baseline(context.Background(), c, []keyedSub{ks})
	faces := h.presence.TakeChanged(h.now())
	if len(faces) != 1 || h.presence.Len() != 1 {
		t.Fatalf("fixture: faces %v, len %d", faces, h.presence.Len())
	}
	return h, c, faces[0]
}

// M2: a replay that ran and did not finish leaves without grace; the next
// presence round over that face rejoins.
func TestLiveHub_UnfinishedReplayLeavesWithoutGrace(t *testing.T) {
	h, _, face := joinedHub(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.round(ctx, true)
	if n := h.presence.Len(); n != 0 {
		t.Fatalf("tracker holds %d entries after the leave, want 0 (no grace)", n)
	}
	if got := h.presence.TakeChanged(h.now()); !slices.Equal(got, []string{face}) {
		t.Fatalf("changed = %v, want [%s]", got, face)
	}
	h.presenceRound(context.Background(), []string{face})
	if n := h.presence.Len(); n != 1 {
		t.Fatalf("tracker holds %d entries after the presence round, want 1", n)
	}
}

// A replay a round skipped keeps the join as it was: the subscription is
// framed, and the tracker sees no leave and no changed face.
func TestLiveHub_FallbackKeepsTheJoin(t *testing.T) {
	h, c, _ := joinedHub(t)
	h.fallback(h.collect(true, nil))
	if n := h.presence.Len(); n != 1 {
		t.Fatalf("tracker holds %d entries after a fallback, want 1", n)
	}
	if got := h.presence.TakeChanged(h.now()); len(got) != 0 {
		t.Fatalf("changed after a fallback = %v, want none", got)
	}
	if got := c.takeChanged(); !slices.Equal(got, []string{"p"}) {
		t.Fatalf("framed = %v, want [p]", got)
	}
}

// Overload: with more pairs on one face than a round replays, the skipped
// ones keep their join, so the face settles within a few ticks and no
// viewer's since restarts.
func TestLiveHub_OverCapPresenceRoundSettles(t *testing.T) {
	clk := newFakeClock()
	h := hubWith(probeHandler("TKT-1", http.StatusOK, nil, nil))
	h.now = clk.Now
	const n = liveRoundCap + 8
	for i := range n {
		c := h.register(principal.Principal{User: fmt.Sprintf("u%03d", i), Tool: principal.ToolDataEntry})
		ks := presenceSub()
		addSub(c, ks)
		h.baseline(context.Background(), c, []keyedSub{ks})
	}
	faces := h.presence.TakeChanged(clk.Now())
	if len(faces) != 1 || h.presence.Len() != n {
		t.Fatalf("fixture: faces %v, len %d", faces, h.presence.Len())
	}
	face := faces[0]
	for tick := 0; len(faces) > 0; tick++ {
		if tick == 3 {
			t.Fatalf("faces still changing after %d ticks: %v", tick, faces)
		}
		clk.advance(liveWindow)
		h.presenceRound(context.Background(), faces)
		faces = h.presence.TakeChanged(clk.Now())
	}
	vs := h.presence.Viewers(face)
	if len(vs) != n {
		t.Fatalf("viewers = %d, want %d", len(vs), n)
	}
	for _, v := range vs {
		if !v.Since.Equal(presenceT0) {
			t.Fatalf("%s since %v, want %v: an untouched viewer restarted", v.User, v.Since, presenceT0)
		}
	}
}

// quietFeed is a change feed that never wakes: no write reaches the hub.
type quietFeed struct{}

func (quietFeed) Wake() <-chan struct{}  { return nil }
func (quietFeed) Take() changefeed.Batch { return changefeed.Batch{} }

// A viewer revoked while the round that would check it was over the cap is
// still checked with no further write: the skipped replay stays pending and
// the next presence round runs it. The viewer is gone within a few ticks,
// nobody else's since restarts, and the rounds stop once nothing changes.
func TestLiveHub_SkippedPresenceReplayIsRescheduled(t *testing.T) {
	const n = liveRoundCap + 8
	var (
		revoked  string
		mu       sync.Mutex
		revoking bool
		replayed = map[string]int{}
	)
	h := hubWith(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := principal.From(r.Context()).User
		mu.Lock()
		replayed[user]++
		gone := revoking && user == revoked
		mu.Unlock()
		if gone {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		recordPresence(r.Context(), &entity.Entity{ID: "TKT-1", Type: "ticket"})
	}))
	clk := newFakeClock()
	h.now = clk.Now
	h.window = 5 * time.Millisecond
	conns := make([]*liveConn, n)
	for i := range n {
		c := h.register(principal.Principal{User: fmt.Sprintf("u%03d", i), Tool: principal.ToolDataEntry})
		ks := presenceSub()
		addSub(c, ks)
		h.baseline(context.Background(), c, []keyedSub{ks})
		conns[i] = c
	}
	faces := h.presence.TakeChanged(clk.Now())
	if len(faces) != 1 || h.presence.Len() != n {
		t.Fatalf("fixture: faces %v, len %d", faces, h.presence.Len())
	}
	face := faces[0]

	// The next presence round is over the cap and skips the pairs past it in
	// collect's order. Revoke the last of those as it starts.
	pairs := h.collect(true, newRoundFilter([]string{face}, nil, false))
	mu.Lock()
	revoked = pairs[len(pairs)-1].owner.User
	clear(replayed)
	revoking = true
	mu.Unlock()
	h.presenceRound(context.Background(), []string{face})
	mu.Lock()
	skipped := replayed[revoked] == 0
	mu.Unlock()
	if !skipped {
		t.Fatalf("fixture: the over-cap round replayed %s; it must skip it", revoked)
	}
	if got := h.presence.TakeChanged(clk.Now()); len(got) != 0 {
		t.Fatalf("fixture: faces changed by the over-cap round: %v", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); h.run(ctx, quietFeed{}, nil, nil) }()
	t.Cleanup(func() { cancel(); <-done })
	waitHubTicks := func(k uint64) {
		t.Helper()
		target := h.ticks.Load() + k
		deadline := time.Now().Add(5 * time.Second)
		for h.ticks.Load() < target {
			if time.Now().After(deadline) {
				t.Fatalf("the live worker did not finish %d ticks", k)
			}
			time.Sleep(time.Millisecond)
		}
	}
	listed := func() bool {
		for _, v := range h.presence.Viewers(face) {
			if v.User == revoked {
				return true
			}
		}
		return false
	}
	// One tick replays the pending pair and leaves; the next publishes.
	for tick := 0; listed(); tick++ {
		if tick == 4 {
			t.Fatalf("%s still listed after %d ticks with no write", revoked, tick)
		}
		waitHubTicks(1)
	}

	// Settle: the leave changed the face once, and that round overflows by
	// a few pairs; those drain and then no replay runs.
	settled := false
	for range 8 {
		mu.Lock()
		clear(replayed)
		mu.Unlock()
		waitHubTicks(2)
		mu.Lock()
		quiet := len(replayed) == 0
		mu.Unlock()
		if quiet && !h.hasPending() {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatal("presence rounds still run every tick")
	}
	for _, c := range conns {
		c.takeChanged()
	}
	waitHubTicks(3)
	for i, c := range conns {
		if got := c.takeChanged(); len(got) != 0 {
			t.Fatalf("u%03d framed %v after the rounds settled", i, got)
		}
	}
	vs := h.presence.Viewers(face)
	if len(vs) != n-1 {
		t.Fatalf("viewers = %d, want %d", len(vs), n-1)
	}
	for _, v := range vs {
		if !v.Since.Equal(presenceT0) {
			t.Fatalf("%s since %v, want %v: an untouched viewer restarted", v.User, v.Since, presenceT0)
		}
	}
}
