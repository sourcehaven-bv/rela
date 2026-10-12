package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Lease route tests (TKT-4YPG4Q). Most run through the full router over
// real HTTP (newLiveEnv), with a real lease service gated by the same
// declarative ACL the app uses.

// leaseRows is what the lease gate needs from the gated reader.
type leaseRows interface {
	Family(ctx context.Context, id string) (visibility.Family, bool, error)
	WriteTarget(ctx context.Context, addr string) (entity.Ref, error)
	ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
}

// testLeaseGate adapts the gated reader to lease.Gate, as appbuild's
// leaseGate does.
type testLeaseGate struct{ rows leaseRows }

func (g testLeaseGate) WriteTarget(ctx context.Context, addr string) (entity.Ref, error) {
	return g.rows.WriteTarget(ctx, addr)
}

func (g testLeaseGate) Family(ctx context.Context, id string) (lease.Family, bool, error) {
	fam, ok, err := g.rows.Family(ctx, id)
	if err != nil || !ok {
		return lease.Family{}, false, err
	}
	return lease.Family{Type: fam.Type, Faces: fam.Faces}, true, nil
}

func (g testLeaseGate) Readable(ctx context.Context, ids []string) (map[string]bool, error) {
	refs := make([]entity.Ref, 0, len(ids))
	for _, id := range ids {
		if ref, err := entity.ParseRef(id); err == nil && ref.Face.IsImplicit() {
			refs = append(refs, ref)
		}
	}
	got, err := g.rows.ResolveHeadersErr(ctx, refs)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for ref, h := range got {
		if h.Family {
			out[ref.ID] = true
		}
	}
	return out, nil
}

// shiftedClock is the wall clock moved by an offset, so a test can jump
// to just before an expiry and let real time carry it past. It can also be
// frozen, so a test can hold it still while a watcher arms its timer.
type shiftedClock struct {
	offset atomic.Int64
	frozen atomic.Pointer[time.Time] // nil: running
}

func (c *shiftedClock) Now() time.Time {
	if t := c.frozen.Load(); t != nil {
		return *t
	}
	return time.Now().Add(time.Duration(c.offset.Load()))
}

// shiftTo moves the clock so that it reads t now, and lets it run.
func (c *shiftedClock) shiftTo(t time.Time) {
	c.offset.Store(int64(time.Until(t)))
	c.frozen.Store(nil)
}

// freezeAt stops the clock at t.
func (c *shiftedClock) freezeAt(t time.Time) { c.frozen.Store(&t) }

// expiryHook runs a function once, after the next lease store NextExpiry
// call returns, so a test can act at the moment a watcher arms its timer.
type expiryHook struct {
	lease.Store
	then atomic.Pointer[func()]
}

func (e *expiryHook) NextExpiry(ctx context.Context, now time.Time) (time.Time, bool, error) {
	next, ok, err := e.Store.NextExpiry(ctx, now)
	if f := e.then.Swap(nil); f != nil {
		(*f)()
	}
	return next, ok, err
}

// revocableAuth is the app's ACL with update access revocable per user, so
// a test can take away access after a claim.
type revocableAuth struct {
	acl.ACL
	mu      sync.Mutex
	revoked map[string]bool
}

func (a *revocableAuth) AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision {
	a.mu.Lock()
	revoked := a.revoked[principal.From(ctx).User]
	a.mu.Unlock()
	if revoked {
		return acl.Decision{Allow: false, Reason: "revoked by the test"}
	}
	return a.ACL.AuthorizeWrite(ctx, req)
}

func (a *revocableAuth) revoke(user string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked[user] = true
}

// newLeaseService builds a real lease service over app's store: kvlease
// over an in-memory KV, gated through a bundle built with d (NopACL when
// nil), authorized by auth.
func newLeaseService(t *testing.T, app *App, d *acl.Declarative, auth lease.WriteAuthorizer, clock lease.Clock) *lease.Service {
	t.Helper()
	return newLeaseServiceOver(t, app, d, auth, clock, func(s lease.Store) lease.Store { return s })
}

// newLeaseServiceOver is newLeaseService with the lease store passed
// through wrap.
func newLeaseServiceOver(
	t *testing.T, app *App, d *acl.Declarative, auth lease.WriteAuthorizer, clock lease.Clock,
	wrap func(lease.Store) lease.Store,
) *lease.Service {
	t.Helper()
	opts := []appbuildtest.Option{appbuildtest.WithStore(app.store)}
	if d != nil {
		opts = append(opts, appbuildtest.WithDeclarative(d))
	}
	svc := appbuildtest.New(app.Meta(), opts...)
	t.Cleanup(func() { _ = svc.Close() })
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	if err != nil {
		t.Fatal(err)
	}
	backend, err := kvlease.New(state.NewFSKV(rfs), filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	leases, err := lease.NewService(lease.Deps{
		Store: wrap(backend), Gate: testLeaseGate{rows: svc.GatedReads().Reader}, Authorizer: auth, Clock: clock,
		Hub: lease.NewHub(), PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return leases
}

// leaseEnv is a running app with leases wired.
type leaseEnv struct {
	*liveEnv
	leases *lease.Service
	clock  *shiftedClock
	auth   *revocableAuth
	expiry *expiryHook
}

// leasePolicy: alice and bob read and update tickets; carol reads tickets;
// dave reads and updates tickets and features. Only dave reads features.
func leasePolicy(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}},
			"viewer": {Read: []string{"ticket"}},
			"admin":  {Read: []string{"ticket", "feature"}, Update: []string{"ticket", "feature"}},
		},
		Assignments: map[string]string{"alice": "editor", "bob": "editor", "carol": "viewer", "dave": "admin"},
	}, app.store)
}

// newLeaseEnv seeds TKT-1, TKT-2 and FEAT-1 (TKT-1 implements FEAT-1, TKT-1
// blocks TKT-2), applies leasePolicy and wires a lease service.
func newLeaseEnv(t *testing.T) *leaseEnv {
	t.Helper()
	env := &leaseEnv{clock: &shiftedClock{}, expiry: &expiryHook{}}
	env.liveEnv = newLiveEnv(t, func(app *App) {
		seedEntity(app, ticket1("one"))
		seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "two"}})
		seedEntity(app, feature("FEAT-1", "hidden"))
		seedRelation(app, entity.NewRelation("TKT-1", "implements", "FEAT-1"))
		seedRelation(app, entity.NewRelation("TKT-1", "blocks", "TKT-2"))
		d := leasePolicy(t, app)
		app.acl = d
		env.auth = &revocableAuth{ACL: d, revoked: map[string]bool{}}
		env.leases = newLeaseServiceOver(t, app, d, env.auth, env.clock, func(s lease.Store) lease.Store {
			env.expiry.Store = s
			return env.expiry
		})
		if err := SetLeases(app, env.leases); err != nil {
			t.Fatal(err)
		}
	})
	return env
}

// doResult is a response whose body has been read and closed.
type doResult struct {
	StatusCode int
	Header     http.Header
}

// do sends method path as user with body (none when empty) and returns the
// response with its body read.
func (e *liveEnv) do(
	t *testing.T, user, method, path, body string, hdr http.Header,
) (res doResult, respBody []byte) {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, e.srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(req.Header, hdr)
	req.Header.Set(testUserHeader, user)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return doResult{StatusCode: resp.StatusCode, Header: resp.Header}, b
}

// claim claims path's entity as user and requires 201.
func (e *leaseEnv) claim(t *testing.T, user, path, body string) v1.LeaseClaimed {
	t.Helper()
	resp, b := e.do(t, user, http.MethodPost, path+"/_lease", body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("claim %s as %s = %d %s, want 201", path, user, resp.StatusCode, b)
	}
	var c v1.LeaseClaimed
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.LeaseID) != 32 || c.Token == 0 || c.ExpiresAt == "" {
		t.Fatalf("claim answer = %+v", c)
	}
	return c
}

func leaseIDBody(id string) string { return fmt.Sprintf(`{"lease_id":%q}`, id) }

// requireUniform404 fails unless got is a 404 whose problem body, instance
// aside, is the entity GET's for want.
func requireUniform404(t *testing.T, gotCode int, got []byte, wantCode int, want []byte) {
	t.Helper()
	if gotCode != http.StatusNotFound || wantCode != http.StatusNotFound {
		t.Fatalf("codes = %d (%s) and GET %d (%s), want 404 for both", gotCode, got, wantCode, want)
	}
	if g, w := stripInstance(t, string(got)), stripInstance(t, string(want)); g != w {
		t.Fatalf("claim 404 differs from entity GET 404:\n claim=%s\n get=  %s", g, w)
	}
}

// AC1: every miss is the entity GET's 404.
func TestLeaseClaim_MissIsTheEntityGet404(t *testing.T) {
	env := newLeaseEnv(t)
	for _, tc := range []struct {
		name, user, claimPath, getPath string
	}{
		{"hidden", "alice", "/api/v1/features/FEAT-1", "/api/v1/features/FEAT-1"},
		{"nonexistent", "alice", "/api/v1/tickets/TKT-404", "/api/v1/tickets/TKT-404"},
		{"wrong plural", "dave", "/api/v1/features/TKT-1", "/api/v1/features/TKT-1"},
		{"malformed address", "alice", "/api/v1/tickets/TKT-1@", "/api/v1/tickets/TKT-1@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, b := env.do(t, tc.user, http.MethodPost, tc.claimPath+"/_lease", "", nil)
			code, want := env.get(t, tc.user, tc.getPath)
			requireUniform404(t, resp.StatusCode, b, code, want)
		})
	}

	t.Run("not wired", func(t *testing.T) {
		plain := newLiveEnv(t, func(app *App) { seedEntity(app, ticket1("one")) })
		resp, b := plain.do(t, "alice", http.MethodPost, "/api/v1/tickets/TKT-1/_lease", "", nil)
		code, want := plain.get(t, "alice", "/api/v1/tickets/TKT-404")
		requireUniform404(t, resp.StatusCode, b, code, want)
	})
}

// AC1: a bare id of a faced type is the write path's 422 face_required.
func TestLeaseClaim_BareFacedAddressIsFaceRequired(t *testing.T) {
	app, d, _ := facedAppWithPolicy(t, acl.RoleDef{Read: []string{"*"}, Update: []string{"ticket@draft", "ticket@published"}})
	if err := SetLeases(app, newLeaseService(t, app, d, d, lease.SystemClock{})); err != nil {
		t.Fatal(err)
	}
	post := func(addr string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/"+addr+"/_lease", http.NoBody)
		app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, d)))
		return rec
	}
	rec := post("TKT-1")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "face_required") {
		t.Fatalf("bare claim = %d %s, want 422 face_required", rec.Code, rec.Body)
	}
	if rec := post("TKT-1@draft"); rec.Code != http.StatusCreated {
		t.Fatalf("faced claim = %d %s, want 201", rec.Code, rec.Body)
	}
}

// AC2 and AC3: a reader without update gets a 403 that names nobody; an
// updater gets the holder.
func TestLeaseClaim_ForbiddenAndHeld(t *testing.T) {
	env := newLeaseEnv(t)
	env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"label":"agent-1","status":"fixing the title"}`)

	resp, b := env.do(t, "carol", http.MethodPost, "/api/v1/tickets/TKT-1/_lease", "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("reader claim = %d %s, want 403", resp.StatusCode, b)
	}
	for _, leak := range []string{"alice", "agent-1", "fixing", "holder", "expires_at"} {
		if bytes.Contains(b, []byte(leak)) {
			t.Errorf("403 body carries %q: %s", leak, b)
		}
	}

	resp, b = env.do(t, "bob", http.MethodPost, "/api/v1/tickets/TKT-1/_lease", "", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("held claim = %d %s, want 409", resp.StatusCode, b)
	}
	var held v1.LeaseHeld
	if err := json.Unmarshal(b, &held); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(held.Type, "/lease_held") || held.Holder.User != "alice" ||
		held.Holder.Tool != principal.ToolDataEntry || held.Holder.Label != "agent-1" ||
		held.Status != "fixing the title" || held.Since == "" || held.ExpiresAt == "" {

		t.Fatalf("held body = %s", b)
	}
	if bytes.Contains(b, []byte("lease_id")) {
		t.Fatalf("held body carries a lease id: %s", b)
	}
}

// AC2: the affordance contract holds for a claim: `_actions.update == false`
// implies a 403 claim, true a 201.
func TestAffordances_LeaseClaimFollowsUpdate(t *testing.T) {
	for _, tc := range []struct {
		name string
		acl  acl.ACL
	}{
		{"NopACL", acl.NopACL{}},
		{"ReadOnlyACL", acl.ReadOnlyACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := buildAppWithACL(t, tc.acl)
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})
			if err := SetLeases(app, newLeaseService(t, app, nil, tc.acl, lease.SystemClock{})); err != nil {
				t.Fatal(err)
			}
			update := fetchActions(t, app, "ticket", "tickets", "TKT-001")["update"]
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/TKT-001/_lease", http.NoBody)
			app.handleV1DynamicRoutes(rec, req.WithContext(aliceCtx()))
			switch {
			case update && rec.Code != http.StatusCreated:
				t.Fatalf("_actions.update=true but claim = %d %s", rec.Code, rec.Body)
			case !update && rec.Code != http.StatusForbidden:
				t.Fatalf("_actions.update=false but claim = %d %s, want 403", rec.Code, rec.Body)
			}
		})
	}
}

// AC4: renew, release and list.
func TestLeaseRenewReleaseList(t *testing.T) {
	env := newLeaseEnv(t)
	c := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"ttl_seconds":60,"status":"s1"}`)

	renew := func(user, body string) (doResult, []byte) {
		return env.do(t, user, http.MethodPost, "/api/v1/_leases/renew", body, nil)
	}
	t.Run("renew keeps an absent status and clears an empty one", func(t *testing.T) {
		resp, b := renew("alice", leaseIDBody(c.LeaseID))
		var r v1.LeaseRenewed
		if resp.StatusCode != http.StatusOK || json.Unmarshal(b, &r) != nil || r.Status != "s1" || r.Token != c.Token {
			t.Fatalf("renew = %d %s", resp.StatusCode, b)
		}
		resp, b = renew("alice", fmt.Sprintf(`{"lease_id":%q,"status":""}`, c.LeaseID))
		r = v1.LeaseRenewed{}
		if resp.StatusCode != http.StatusOK || json.Unmarshal(b, &r) != nil || r.Status != "" {
			t.Fatalf("renew clearing status = %d %s", resp.StatusCode, b)
		}
	})
	t.Run("list", func(t *testing.T) {
		resp, b := env.do(t, "alice", http.MethodGet, "/api/v1/_leases", "", nil)
		var l v1.OwnLeaseList
		if resp.StatusCode != http.StatusOK || json.Unmarshal(b, &l) != nil ||
			len(l.Data) != 1 || l.Data[0].ID != "TKT-1" || l.Data[0].ExpiresAt == "" {

			t.Fatalf("list = %d %s", resp.StatusCode, b)
		}
		resp, b = env.do(t, "bob", http.MethodGet, "/api/v1/_leases", "", nil)
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), `"data":[]`) {
			t.Fatalf("bob's list = %d %s, want empty", resp.StatusCode, b)
		}
	})
	t.Run("another user's renew is lost", func(t *testing.T) {
		resp, b := renew("bob", leaseIDBody(c.LeaseID))
		if resp.StatusCode != http.StatusConflict || !strings.Contains(string(b), "lease_lost") {
			t.Fatalf("bob's renew = %d %s, want 409 lease_lost", resp.StatusCode, b)
		}
	})
	t.Run("input errors are 400", func(t *testing.T) {
		for _, body := range []string{`{`, `{"lease_id":"nope"}`, `[]`, ``,
			fmt.Sprintf(`{"lease_id":%q,"ttl_seconds":5}`, c.LeaseID),
			fmt.Sprintf(`{"lease_id":%q,"status":"a\nb"}`, c.LeaseID),
			fmt.Sprintf(`{"lease_id":%q,"ttl":60}`, c.LeaseID),
			fmt.Sprintf(`{"leaseid":%q}`, c.LeaseID),
			fmt.Sprintf(`{"lease_id":%q}{}`, c.LeaseID)} {
			if resp, b := renew("alice", body); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("renew %q = %d %s, want 400", body, resp.StatusCode, b)
			}
		}
		for _, body := range []string{`{`, `{"ttl_seconds":99999999999}`, `{"label":"has space"}`, `{"ttl_seconds":"x"}`,
			`{"ttl":60}`, `{"ttl_secs":60}`, `{"statuss":"x"}`, `{} {}`, `{}x`} {
			if resp, b := env.do(t, "bob", http.MethodPost, "/api/v1/tickets/TKT-2/_lease", body, nil); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("claim %q = %d %s, want 400", body, resp.StatusCode, b)
			}
		}
	})
	t.Run("release is 204 always, a malformed id 400", func(t *testing.T) {
		for _, tc := range []struct {
			user, body string
			want       int
		}{
			{"bob", leaseIDBody(c.LeaseID), http.StatusNoContent},
			{"alice", leaseIDBody(c.LeaseID), http.StatusNoContent},
			{"alice", leaseIDBody(c.LeaseID), http.StatusNoContent},
			{"alice", leaseIDBody(strings.Repeat("0", 32)), http.StatusNoContent},
			{"alice", `{"lease_id":"XYZ"}`, http.StatusBadRequest},
			{"alice", `{`, http.StatusBadRequest},
		} {
			resp, b := env.do(t, tc.user, http.MethodPost, "/api/v1/_leases/release", tc.body, nil)
			if resp.StatusCode != tc.want {
				t.Errorf("release %s as %s = %d %s, want %d", tc.body, tc.user, resp.StatusCode, b, tc.want)
			}
		}
		if resp, b := renew("alice", leaseIDBody(c.LeaseID)); resp.StatusCode != http.StatusConflict {
			t.Fatalf("renew after release = %d %s, want 409", resp.StatusCode, b)
		}
	})
	t.Run("wrong methods", func(t *testing.T) {
		for _, p := range []string{"/api/v1/_leases/renew", "/api/v1/_leases/release", "/api/v1/tickets/TKT-1/_lease"} {
			if resp, _ := env.do(t, "alice", http.MethodGet, p, "", nil); resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("GET %s = %d, want 405", p, resp.StatusCode)
			}
		}
		if resp, _ := env.do(t, "alice", http.MethodPost, "/api/v1/_leases", "{}", nil); resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST /_leases = %d, want 405", resp.StatusCode)
		}
		if resp, _ := env.do(t, "alice", http.MethodGet, "/api/v1/_leases/other", "", nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET /_leases/other = %d, want 404", resp.StatusCode)
		}
	})
}

// AC4: a renew after expiry, revocation, delete, or delete and recreate is
// 409 lease_lost.
func TestLeaseRenew_LostCases(t *testing.T) {
	ctx := context.Background()
	deleteTKT1 := func(env *leaseEnv) error {
		_, err := env.app.store.DeleteFamily(ctx, "TKT-1", true)
		return err
	}
	for _, tc := range []struct {
		name  string
		after func(env *leaseEnv, c v1.LeaseClaimed) error
	}{
		{"expired", func(env *leaseEnv, c v1.LeaseClaimed) error {
			exp, err := time.Parse(time.RFC3339, c.ExpiresAt)
			env.clock.shiftTo(exp.Add(time.Second))
			return err
		}},
		{"revoked", func(env *leaseEnv, _ v1.LeaseClaimed) error {
			env.auth.revoke("alice")
			return nil
		}},
		{"deleted", func(env *leaseEnv, _ v1.LeaseClaimed) error { return deleteTKT1(env) }},
		{"deleted and recreated", func(env *leaseEnv, _ v1.LeaseClaimed) error {
			if err := deleteTKT1(env); err != nil {
				return err
			}
			// What appbuild's alias fanout does on every delete.
			if err := env.leases.EntityDeleted(ctx, "TKT-1"); err != nil {
				return err
			}
			return env.app.store.CreateEntity(ctx, ticket1("again"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newLeaseEnv(t)
			c := env.claim(t, "alice", "/api/v1/tickets/TKT-1", "")
			if err := tc.after(env, c); err != nil {
				t.Fatal(err)
			}
			resp, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/renew", leaseIDBody(c.LeaseID), nil)
			if resp.StatusCode != http.StatusConflict || !strings.Contains(string(b), "/lease_lost") {
				t.Fatalf("renew = %d %s, want 409 lease_lost", resp.StatusCode, b)
			}
			if _, b := env.get(t, "alice", "/api/v1/tickets/TKT-1"); bytes.Contains(b, []byte(`"_lease"`)) {
				t.Fatalf("a lost lease still shows: %s", b)
			}
		})
	}
}

// stubLeases answers every call with err.
type stubLeases struct{ err error }

// installLeases wires svc into app the way SetLeases does, for a test
// double SetLeases does not accept.
func installLeases(app *App, svc LeaseService) { app.leases = &leasesHandler{svc: svc} }

func (s stubLeases) Claim(context.Context, lease.ClaimRequest) (lease.Claimed, error) {
	return lease.Claimed{}, s.err
}

func (s stubLeases) Renew(context.Context, string, time.Duration, *string) (lease.Lease, error) {
	return lease.Lease{}, s.err
}
func (s stubLeases) Release(context.Context, string) error { return s.err }
func (s stubLeases) GetReadable(context.Context, []string) (map[string]lease.Lease, error) {
	return nil, s.err
}
func (s stubLeases) Mine(context.Context) ([]lease.Lease, error) { return nil, s.err }
func (s stubLeases) Subscribe() (wake <-chan struct{}, cancel func()) {
	return nil, func() {}
}

// AC4: the error mapping every route shares.
func TestLease_ErrorMapping(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name  string
		err   error
		code  int
		ptype string
	}{
		{"limit", lease.ErrLimit, http.StatusConflict, "lease_limit"},
		{"full", lease.ErrFull, http.StatusServiceUnavailable, "lease_store_full"},
		{"lost", lease.ErrLost, http.StatusConflict, "lease_lost"},
		{"invalid", fmt.Errorf("%w: status must be valid UTF-8", lease.ErrInvalid), http.StatusBadRequest, "invalid_request"},
		{"no principal", lease.ErrNoPrincipal, http.StatusForbidden, "no_principal"},
		{"internal", errors.New("disk on fire"), http.StatusInternalServerError, "lease_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := withLogLevel(t, slog.LevelDebug)
			app := newTestAppV1(t)
			seedEntity(app, ticket1("one"))
			installLeases(app, stubLeases{err: tc.err})
			router := app.NewRouter()
			for _, rq := range []struct{ method, path, body string }{
				{http.MethodPost, "/api/v1/tickets/TKT-1/_lease", ""},
				{http.MethodPost, "/api/v1/_leases/renew", leaseIDBody(id)},
				{http.MethodPost, "/api/v1/_leases/release", leaseIDBody(id)},
				{http.MethodGet, "/api/v1/_leases", ""},
			} {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(rq.method, rq.path, strings.NewReader(rq.body)).
					WithContext(aliceCtx()))
				if rec.Code != tc.code || !strings.Contains(rec.Body.String(), "/"+tc.ptype) {
					t.Errorf("%s %s = %d %s, want %d %s", rq.method, rq.path, rec.Code, rec.Body, tc.code, tc.ptype)
				}
				if tc.code == http.StatusServiceUnavailable && rec.Header().Get("Retry-After") != "30" {
					t.Errorf("%s %s: Retry-After = %q, want 30", rq.method, rq.path, rec.Header().Get("Retry-After"))
				}
			}
			if strings.Contains(logs.String(), id) {
				t.Fatalf("a log line carries the lease id: %s", logs)
			}
		})
	}
}

// leaseOf decodes body and returns its `_lease` object, or nil.
func leaseOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var e map[string]any
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	l, _ := e["_lease"].(map[string]any)
	return l
}

// AC5 and AC9: `_lease` on the entity GET and list rows, without expiry or
// lease id; no 304 while held; the ETag unchanged.
func TestLease_OnEntityGetAndListRows(t *testing.T) {
	env := newLeaseEnv(t)
	resp, b := env.do(t, "carol", http.MethodGet, "/api/v1/tickets/TKT-1", "", nil)
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != http.StatusOK || etag == "" || leaseOf(t, b) != nil {
		t.Fatalf("unheld GET = %d etag=%q %s", resp.StatusCode, etag, b)
	}
	inm := http.Header{"If-None-Match": {etag}}
	if cond, _ := env.do(t, "carol", http.MethodGet, "/api/v1/tickets/TKT-1", "", inm); cond.StatusCode != http.StatusOK {
		t.Fatalf("unheld conditional GET = %d, want 200 (no 304 while leases are wired)", cond.StatusCode)
	}

	c := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"label":"a1","status":"busy"}`)

	resp, b = env.do(t, "carol", http.MethodGet, "/api/v1/tickets/TKT-1", "", inm)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("held conditional GET = %d, want 200 (no 304 while held)", resp.StatusCode)
	}
	if got := resp.Header.Get("ETag"); got != etag {
		t.Fatalf("ETag moved on claim: %q -> %q", etag, got)
	}
	l := leaseOf(t, b)
	if l["user"] != "alice" || l["tool"] != principal.ToolDataEntry || l["label"] != "a1" ||
		l["status"] != "busy" || l["since"] == nil || len(l) != 5 {

		t.Fatalf("_lease = %v", l)
	}
	if bytes.Contains(b, []byte(c.LeaseID)) || bytes.Contains(b, []byte("expires_at")) {
		t.Fatalf("GET carries the lease id or expiry: %s", b)
	}

	_, b = env.get(t, "carol", "/api/v1/tickets")
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	for _, row := range list.Data {
		_, has := row["_lease"]
		if has != (row["id"] == "TKT-1") {
			t.Errorf("row %v: _lease present = %v", row["id"], has)
		}
	}
	if bytes.Contains(b, []byte(c.LeaseID)) || bytes.Contains(b, []byte("expires_at")) {
		t.Fatalf("list carries the lease id or expiry: %s", b)
	}

	// If-Match keeps working for the holder's own write.
	resp, b = env.do(t, "alice", http.MethodPatch, "/api/v1/tickets/TKT-1",
		`{"properties":{"title":"edited"}}`, http.Header{"If-Match": {etag}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH with the pre-claim ETag = %d %s, want 200", resp.StatusCode, b)
	}
}

// The ETag does not cover `_lease`, so with leases wired the entity GET
// never answers 304: a copy cached while the entity was held would keep a
// `_lease` that a release has removed.
func TestLease_NoNotModifiedAfterRelease(t *testing.T) {
	env := newLeaseEnv(t)
	c := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"status":"busy"}`)
	resp, b := env.do(t, "carol", http.MethodGet, "/api/v1/tickets/TKT-1", "", nil)
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != http.StatusOK || etag == "" || leaseOf(t, b) == nil {
		t.Fatalf("held GET = %d etag=%q %s, want 200 with _lease", resp.StatusCode, etag, b)
	}
	if rel, relBody := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(c.LeaseID), nil); rel.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d %s", rel.StatusCode, relBody)
	}
	resp, b = env.do(t, "carol", http.MethodGet, "/api/v1/tickets/TKT-1", "", http.Header{"If-None-Match": {etag}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("conditional GET with the held copy's ETag = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("ETag"); got != etag {
		t.Fatalf("ETag moved on release: %q -> %q", etag, got)
	}
	if l := leaseOf(t, b); l != nil {
		t.Fatalf("_lease after release = %v, want absent", l)
	}
}

// AC5: only the entity GET and list rows carry `_lease`. Views, history and
// includes never do, for a held entity the reader can see or one it cannot.
func TestLease_NotOnOtherSurfaces(t *testing.T) {
	env := newLeaseEnv(t)
	env.claim(t, "dave", "/api/v1/features/FEAT-1", `{"status":"secret work"}`)
	env.claim(t, "dave", "/api/v1/tickets/TKT-2", `{"status":"visible work"}`)

	for _, user := range []string{"alice", "dave"} {
		for _, path := range []string{
			"/api/v1/tickets/TKT-1?include=implements,blocks",
			"/api/v1/tickets?include=implements,blocks",
			"/api/v1/_views/ticket/TKT-1",
			"/api/v1/_views/ticket/TKT-2",
			"/api/v1/_history/tickets/TKT-2",
			"/api/v1/features/FEAT-1",
			"/api/v1/tickets/TKT-2/relations",
		} {
			_, b := env.get(t, user, path)
			otherSurface := strings.Contains(path, "/_views/") || strings.Contains(path, "/_history/") ||
				strings.Contains(path, "/relations")
			hidden := user == "alice" && strings.Contains(path, "FEAT-1")
			if (otherSurface || hidden) && bytes.Contains(b, []byte(`"_lease"`)) {
				t.Errorf("%s as %s carries _lease: %s", path, user, b)
			}
			if user == "alice" && bytes.Contains(b, []byte("secret work")) {
				t.Errorf("%s as alice shows the hidden entity's lease: %s", path, b)
			}
		}
	}

	// The included TKT-2 is held and readable, and still has no `_lease`:
	// only the GET's own entity and list rows get one.
	_, b := env.get(t, "alice", "/api/v1/tickets/TKT-1?include=blocks")
	var e v1.Entity
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	inc, ok := e.Included["TKT-2"]
	if !ok {
		t.Fatalf("fixture: TKT-2 not included: %s", b)
	}
	if inc.Lease != nil {
		t.Fatalf("an included entity carries _lease: %+v", inc.Lease)
	}
}

// countingLeaseStore counts lease store Get calls.
type countingLeaseStore struct {
	lease.Store
	gets atomic.Int64
}

func (c *countingLeaseStore) Get(ctx context.Context, ids []string, now time.Time) (map[string]lease.Lease, error) {
	c.gets.Add(1)
	return c.Store.Get(ctx, ids, now)
}

// AC6: a list page costs one lease store read, at 10 rows and at 50.
func TestLease_ListCostIsOneReadPerPage(t *testing.T) {
	counter := &countingLeaseStore{}
	var svc *lease.Service
	env := newLiveEnv(t, func(app *App) {
		for i := range 60 {
			seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%03d", i), Type: "ticket",
				Properties: map[string]any{"title": "t"}})
		}
		svc = newLeaseServiceOver(t, app, nil, acl.NopACL{}, lease.SystemClock{}, func(s lease.Store) lease.Store {
			counter.Store = s
			return counter
		})
		if err := SetLeases(app, svc); err != nil {
			t.Fatal(err)
		}
	})
	env.claimAll(t, svc)
	for _, n := range []int{10, 50} {
		counter.gets.Store(0)
		code, b := env.get(t, "alice", fmt.Sprintf("/api/v1/tickets?per_page=%d", n))
		if code != http.StatusOK || bytes.Count(b, []byte(`"_lease"`)) != n {
			t.Fatalf("list of %d = %d, %d leases", n, code, bytes.Count(b, []byte(`"_lease"`)))
		}
		if got := counter.gets.Load(); got != 1 {
			t.Errorf("list of %d rows read the lease store %d times, want 1", n, got)
		}
	}
}

// claimAll holds every ticket the first list page of 50 shows, across
// several users to stay under the per-identity cap.
func (e *liveEnv) claimAll(t *testing.T, svc LeaseService) {
	t.Helper()
	for i := range 50 {
		ctx := principal.With(context.Background(),
			principal.Principal{User: fmt.Sprintf("u%d", i/lease.MaxPerIdentity), Tool: principal.ToolDataEntry})
		if _, err := svc.Claim(ctx, lease.ClaimRequest{Addr: fmt.Sprintf("TKT-%03d", i)}); err != nil {
			t.Fatal(err)
		}
	}
}

// AC7: a claim, a status change, a release and an expiry each send exactly
// one live:changed; a plain renew sends none.
func TestLive_LeaseChangesFrameOnce(t *testing.T) {
	env := newLeaseEnv(t)
	c, conn := env.open(t, "carol")
	env.subscribe(t, "carol", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"})
	liveQuiet(t, c)

	once := func(step string) {
		t.Helper()
		if got := changedKeys(t, c, 1); got[0] != "detail" {
			t.Fatalf("%s: changed keys = %v", step, got)
		}
		liveQuiet(t, c)
	}
	renew := func(body string) {
		t.Helper()
		if resp, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/renew", body, nil); resp.StatusCode != http.StatusOK {
			t.Fatalf("renew = %d %s", resp.StatusCode, b)
		}
	}

	cl := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"ttl_seconds":30}`)
	once("claim")
	renew(fmt.Sprintf(`{"lease_id":%q,"ttl_seconds":40}`, cl.LeaseID))
	liveQuiet(t, c)
	renew(fmt.Sprintf(`{"lease_id":%q,"status":"now busy"}`, cl.LeaseID))
	once("status change")
	if resp, b := env.do(t, "alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(cl.LeaseID), nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d %s", resp.StatusCode, b)
	}
	once("release")

	env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"ttl_seconds":30}`)
	once("second claim")
	// The exact expiry comes from the store: expires_at on the wire is
	// whole seconds, up to a second earlier.
	exp, ok, err := env.expiry.Store.NextExpiry(context.Background(), env.clock.Now())
	if err != nil || !ok {
		t.Fatalf("next expiry = %v, %v, %v", exp, ok, err)
	}
	// The watcher arms its timer only for a lease live when it starts, so
	// the clock holds just before the expiry until the watcher has read the
	// next expiry, and then moves past it. The timer fires 50ms later and
	// finds the lease lapsed, however late the watcher got to run.
	env.clock.freezeAt(exp.Add(-50 * time.Millisecond))
	pastExpiry := func() { env.clock.shiftTo(exp.Add(time.Millisecond)) }
	env.expiry.then.Store(&pastExpiry)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = env.leases.Watch(ctx)
	}()
	t.Cleanup(func() { cancel(); <-done })
	once("expiry")
}

// AC8: a lease on an entity the subscriber cannot read sends no frame.
func TestLive_LeaseOnHiddenEntitySendsNoFrame(t *testing.T) {
	env := newLeaseEnv(t)
	c, conn := env.open(t, "alice")
	env.subscribe(t, "alice", conn,
		liveSubEntry{"detail", "/api/v1/tickets/TKT-1"},
		liveSubEntry{"list", "/api/v1/tickets"},
		liveSubEntry{"view", "/api/v1/_views/ticket/TKT-1"},
		liveSubEntry{"hidden", "/api/v1/features/FEAT-1"},
		liveSubEntry{"hidden-list", "/api/v1/features"},
	)

	cl := env.claim(t, "dave", "/api/v1/features/FEAT-1", `{"status":"secret"}`)
	if resp, b := env.do(t, "dave", http.MethodPost, "/api/v1/_leases/renew",
		fmt.Sprintf(`{"lease_id":%q,"status":"still secret"}`, cl.LeaseID), nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("renew = %d %s", resp.StatusCode, b)
	}
	if resp, _ := env.do(t, "dave", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(cl.LeaseID), nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("release = %d", resp.StatusCode)
	}
	liveQuiet(t, c)

	// The stream is still live: a visible claim frames.
	env.claim(t, "bob", "/api/v1/tickets/TKT-1", "")
	requireChanged(t, c, "detail", "list")
}

// syncBuffer is a bytes.Buffer safe for a logger in server goroutines and a
// reader in the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// AC10: the lease id appears in no GET, list, SSE frame or log line, on
// the success paths and the renew and release error paths.
func TestLease_IDNeverLeaks(t *testing.T) {
	var logs syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	env := newLeaseEnv(t)
	stream, conn := env.open(t, "carol")
	env.subscribe(t, "carol", conn, liveSubEntry{"detail", "/api/v1/tickets/TKT-1"}, liveSubEntry{"list", "/api/v1/tickets"})

	cl := env.claim(t, "alice", "/api/v1/tickets/TKT-1", `{"status":"s"}`)
	id := cl.LeaseID
	var seen []string
	for _, rq := range []struct{ user, method, path, body string }{
		{"carol", http.MethodGet, "/api/v1/tickets/TKT-1", ""},
		{"alice", http.MethodGet, "/api/v1/tickets/TKT-1", ""},
		{"alice", http.MethodGet, "/api/v1/tickets", ""},
		{"alice", http.MethodGet, "/api/v1/_leases", ""},
		{"bob", http.MethodPost, "/api/v1/_leases/renew", leaseIDBody(id)},
		{"bob", http.MethodPost, "/api/v1/_leases/renew", fmt.Sprintf(`{"lease_id":%q,"ttl_seconds":1}`, id)},
		{"bob", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(id)},
		{"alice", http.MethodPost, "/api/v1/_leases/renew", fmt.Sprintf(`{"lease_id":%q,"status":"t"}`, id)},
		{"alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(id)},
		{"alice", http.MethodPost, "/api/v1/_leases/renew", leaseIDBody(id)},
		{"alice", http.MethodPost, "/api/v1/_leases/release", leaseIDBody(id + "x")},
	} {
		_, b := env.do(t, rq.user, rq.method, rq.path, rq.body, nil)
		seen = append(seen, string(b))
	}
	deadline := time.After(quietWindow)
collect:
	for {
		select {
		case f, ok := <-stream.frames:
			if !ok {
				break collect
			}
			seen = append(seen, f.event+" "+f.data)
		case <-deadline:
			break collect
		}
	}
	seen = append(seen, logs.String())
	for _, s := range seen {
		if strings.Contains(s, id) {
			t.Fatalf("the lease id leaked: %s", s)
		}
	}
	if !strings.Contains(logs.String(), "msg=request") {
		t.Fatal("fixture: no request log line was captured; the test would not cover the access log")
	}
}

// AC11: a cookie-bearing cross-origin request is refused on every lease
// route. An origin-less, cookie-less request, the shape a script or agent
// sends with a token, reaches renew, release and the list: /_leases is
// non-browser exempt like the data routes.
func TestLease_CrossOriginRefused(t *testing.T) {
	app := newSecuredTestApp(t)
	installLeases(app, stubLeases{})
	h := app.NewRouter()
	send := func(method, path string, hdr map[string]string) *httptest.ResponseRecorder {
		var body io.Reader = http.NoBody
		if method == http.MethodPost {
			body = strings.NewReader(leaseIDBody(strings.Repeat("a", 32)))
		}
		r := httptest.NewRequest(method, path, body)
		r.Host = "127.0.0.1:8080"
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	browser := map[string]string{"Origin": "https://evil.example", "Cookie": "session=abc"}
	for _, rq := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/tickets/TKT-001/_lease"},
		{http.MethodPost, "/api/v1/_leases/renew"},
		{http.MethodPost, "/api/v1/_leases/release"},
		{http.MethodGet, "/api/v1/_leases"},
	} {
		w := send(rq.method, rq.path, browser)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "origin") {
			t.Errorf("cross-origin %s %s = %d %s, want 403", rq.method, rq.path, w.Code, w.Body)
		}
		if w := send(rq.method, rq.path, map[string]string{"Cookie": "session=abc"}); w.Code != http.StatusForbidden {
			t.Errorf("origin-less cookie %s %s = %d %s, want 403", rq.method, rq.path, w.Code, w.Body)
		}
	}
	for _, rq := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/_leases/renew", http.StatusOK},
		{http.MethodPost, "/api/v1/_leases/release", http.StatusNoContent},
		{http.MethodGet, "/api/v1/_leases", http.StatusOK},
	} {
		if w := send(rq.method, rq.path, nil); w.Code != rq.want {
			t.Errorf("origin-less %s %s = %d %s, want %d", rq.method, rq.path, w.Code, w.Body, rq.want)
		}
		if !isCSRFExempt(httptest.NewRequest(rq.method, rq.path, http.NoBody)) {
			t.Errorf("%s is not CSRF-exempt for a non-browser request", rq.path)
		}
	}
}
