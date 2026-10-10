package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// testLeaseGate adapts the gated reader to lease.Gate, as appbuild's
// leaseGate does, so the service answers not-found where show_entity does.
type testLeaseGate struct{ rows GraphReader }

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

// leaseClock is a settable clock.
type leaseClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *leaseClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *leaseClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *leaseClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newTestLeases builds a real lease service: kvlease over an in-memory KV,
// gated through rows, authorized by auth.
func newTestLeases(t *testing.T, rows GraphReader, auth lease.WriteAuthorizer, clock lease.Clock) *lease.Service {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	if err != nil {
		t.Fatalf("rooted fs: %v", err)
	}
	backend, err := kvlease.New(state.NewFSKV(rfs), filelock.ProcessPrivate{})
	if err != nil {
		t.Fatalf("kvlease: %v", err)
	}
	svc, err := lease.NewService(lease.Deps{
		Store: backend, Gate: testLeaseGate{rows: rows}, Authorizer: auth, Clock: clock, Hub: lease.NewHub(),
	})
	if err != nil {
		t.Fatalf("lease.NewService: %v", err)
	}
	return svc
}

// leaseEnv is a policy project with a lease service and one client session
// per principal.
type leaseEnv struct {
	st     store.Store
	leases *lease.Service
	clock  *leaseClock
	deps   Deps
	d      *acl.Declarative
}

var (
	leaseAlice = principal.Principal{User: "alice", Tool: principal.ToolMCP}
	leaseBob   = principal.Principal{User: "bob", Tool: principal.ToolMCP}
)

// newLeaseEnv builds a project under a policy: alice and bob (group
// `editors`) read and update tickets and read notes; bob (group `noters`)
// also updates notes; nobody reads features.
func newLeaseEnv(t *testing.T) *leaseEnv {
	t.Helper()
	manual := func(label, prefix string) metamodel.EntityDef {
		return metamodel.EntityDef{
			Label: label, IDPrefix: prefix, IDType: metamodel.IDTypeManual, DisplayProperty: "title",
			Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
		}
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  manual("Ticket", "TKT"),
			"note":    manual("Note", "NOTE"),
			"feature": manual("Feature", "FEAT"),
			"person":  manual("Person", ""),
			"group":   manual("Group", ""),
		},
		Relations: map[string]metamodel.RelationDef{
			"member-of": {Label: "member of", From: []string{"person"}, To: []string{"group"}},
		},
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		newEntity("TKT-1", "ticket", "a ticket"),
		newEntity("TKT-2", "ticket", "another ticket"),
		newEntity("NOTE-1", "note", "a note"),
		newEntity("FEAT-1", "feature", "hidden feature"),
		newEntity("alice", "person", "Alice"),
		newEntity("bob", "person", "Bob"),
		newEntity("editors", "group", "Editors"),
		newEntity("noters", "group", "Noters"),
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "alice", Type: "member-of", To: "editors"},
		{From: "bob", Type: "member-of", To: "editors"},
		{From: "bob", Type: "member-of", To: "noters"},
	} {
		if _, err := st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket", "note"}, Update: []string{"ticket"}},
			"noter":  {Read: []string{"note"}, Update: []string{"note"}},
		},
		Assignments: map[string]string{"editors": "editor", "noters": "noter"},
	}, aclstore.NewGraph(st), st)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	reads := svc.GatedReads()
	clock := &leaseClock{now: time.Now()}
	leases := newTestLeases(t, reads.Reader, svc.ACL(), clock)
	deps := Deps{
		Store:         reads.Reader,
		Traversals:    reads.Traversals,
		Meta:          meta,
		Tracer:        reads.Tracer,
		Searcher:      reads.Searcher,
		Validator:     reads.Validator,
		EntityManager: svc.EntityManager(),
		Config:        svc.Config(),
		Watcher:       nopWatcher{},
		Watch:         WatchDeps{Changes: st, Rebind: reads.Rebind},
		ProjectRoot:   t.TempDir(),
		Attachments:   testAttachmentDeps(t, svc, meta),
		World:         store.TrivialScope(),
		Families:      store.TrivialScope(),
		Leases:        leases,
	}
	return &leaseEnv{st: st, leases: leases, clock: clock, deps: deps, d: d}
}

// bound stamps p on a context and binds its Request on e's evaluator, as a
// transport edge does before it calls the lease service.
func (e *leaseEnv) bound(t *testing.T, p principal.Principal) context.Context {
	t.Helper()
	return acltest.Bind(principal.With(context.Background(), p), t, e.d)
}

// session connects a client that runs every call as p, with an acl.Request
// attached up front as remote MCP does.
func (e *leaseEnv) session(t *testing.T, p principal.Principal) *mcpgo.ClientSession {
	t.Helper()
	srv, err := NewServer(e.deps, "test", WithPrincipal(p))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.mcp.AddReceivingMiddleware(func(next mcpgo.MethodHandler) mcpgo.MethodHandler {
		return func(ctx context.Context, method string, req mcpgo.Request) (mcpgo.Result, error) {
			r, err := acl.OpenRequest(ctx, e.d, p)
			if err != nil {
				return nil, err
			}
			return next(acl.WithRequest(ctx, r), method, req)
		}
	})
	return connect(t, srv)
}

// claim claims id through cs and returns the decoded result.
func claim(t *testing.T, cs *mcpgo.ClientSession, args string) claimResult {
	t.Helper()
	text, isError := callToolOn(t, cs, "claim_entity", args)
	if isError {
		t.Fatalf("claim_entity %s: %s", args, text)
	}
	var out claimResult
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if len(out.LeaseID) != 32 || out.Token == 0 || out.ExpiresAt == "" {
		t.Fatalf("claim result %+v", out)
	}
	return out
}

// AC1: a claim on a hidden or nonexistent id answers byte for byte what
// show_entity answers.
func TestClaimEntity_NotFoundMatchesShowEntity(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	cs := e.session(t, leaseAlice)
	for _, id := range []string{"FEAT-1", "FEAT-404", "TKT-404", "TKT-1@draft", "FEAT-1@draft", "TKT-1@", "@x"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			show, showErr := callToolOn(t, cs, "show_entity", `{"id":"`+id+`"}`)
			got, claimErr := callToolOn(t, cs, "claim_entity", `{"id":"`+id+`"}`)
			if !showErr || !claimErr {
				t.Fatalf("want errors, got show=%v claim=%v", showErr, claimErr)
			}
			if got != show {
				t.Fatalf("claim text %q, show_entity text %q", got, show)
			}
		})
	}
}

// AC2 and AC3: read without update is forbidden and names no holder; a
// held entity answers `lease held:` with the holder.
func TestClaimEntity_ForbiddenAndHeld(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	alice, bob := e.session(t, leaseAlice), e.session(t, leaseBob)
	claim(t, bob, `{"id":"NOTE-1","label":"bot-b","status":"editing the note"}`)
	first := claim(t, alice, `{"id":"TKT-1","label":"bot-a","status":"fixing the title"}`)

	text, isError := callToolOn(t, alice, "claim_entity", `{"id":"NOTE-1"}`)
	if !isError || !strings.HasPrefix(text, "forbidden:") {
		t.Fatalf("read-only claim = %q, want forbidden", text)
	}
	for _, leak := range []string{"bob", "bot-b", "editing", "lease held", "holder"} {
		if strings.Contains(text, leak) {
			t.Errorf("forbidden text %q contains %q", text, leak)
		}
	}

	text, isError = callToolOn(t, bob, "claim_entity", `{"id":"TKT-1"}`)
	body, ok := strings.CutPrefix(text, "lease held: ")
	if !isError || !ok {
		t.Fatalf("claim on a held entity = %q", text)
	}
	var held struct {
		Holder    map[string]string `json:"holder"`
		Since     string            `json:"since"`
		ExpiresAt string            `json:"expires_at"`
		Status    string            `json:"status"`
	}
	if err := json.Unmarshal([]byte(body), &held); err != nil {
		t.Fatalf("decode held body %q: %v", body, err)
	}
	wantHolder := map[string]string{"user": "alice", "tool": principal.ToolMCP, "label": "bot-a"}
	if len(held.Holder) != len(wantHolder) {
		t.Errorf("holder = %v, want %v", held.Holder, wantHolder)
	}
	for k, v := range wantHolder {
		if held.Holder[k] != v {
			t.Errorf("holder[%s] = %q, want %q", k, held.Holder[k], v)
		}
	}
	if held.Since == "" || held.ExpiresAt != first.ExpiresAt || held.Status != "fixing the title" {
		t.Errorf("held = %+v", held)
	}
	if strings.Contains(text, first.LeaseID) {
		t.Errorf("held text carries the lease id")
	}
}

// AC4: every way a lease ends answers renew with one text; release always
// answers released.
func TestRenewLease_LostIsOneAnswer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		end  func(t *testing.T, e *leaseEnv)
	}{
		{"released", nil},
		{"expired", func(_ *testing.T, e *leaseEnv) { e.clock.advance(lease.DefaultTTL + time.Second) }},
		{"revoked", func(t *testing.T, e *leaseEnv) {
			t.Helper()
			k := entity.RelationKey{From: "alice", Type: "member-of", To: "editors"}
			if err := e.st.DeleteRelation(context.Background(), k); err != nil {
				t.Fatal(err)
			}
		}},
		{"deleted", func(t *testing.T, e *leaseEnv) {
			t.Helper()
			deleteWithHook(t, e, "TKT-1")
		}},
		{"deleted and recreated", func(t *testing.T, e *leaseEnv) {
			t.Helper()
			deleteWithHook(t, e, "TKT-1")
			if err := e.st.CreateEntity(context.Background(), newEntity("TKT-1", "ticket", "a new ticket")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newLeaseEnv(t)
			cs := e.session(t, leaseAlice)
			c := claim(t, cs, `{"id":"TKT-1"}`)
			if tc.end == nil {
				releaseOK(t, cs, c.LeaseID)
			} else {
				tc.end(t, e)
			}
			text, isError := callToolOn(t, cs, "renew_lease", `{"lease_id":"`+c.LeaseID+`"}`)
			if !isError || text != lease.LostText {
				t.Fatalf("renew = %q, want %q", text, lease.LostText)
			}
			releaseOK(t, cs, c.LeaseID)
			if show, _ := callToolOn(t, cs, "show_entity", `{"id":"TKT-1"}`); strings.Contains(show, `"lease"`) {
				t.Fatalf("show_entity after the lease ended = %s", show)
			}
		})
	}

	t.Run("release of another's lease or an unknown id", func(t *testing.T) {
		t.Parallel()
		e := newLeaseEnv(t)
		alice, bob := e.session(t, leaseAlice), e.session(t, leaseBob)
		c := claim(t, alice, `{"id":"TKT-1"}`)
		releaseOK(t, bob, c.LeaseID)
		releaseOK(t, bob, strings.Repeat("0", 32))
		if _, err := e.leases.Renew(e.bound(t, leaseAlice), c.LeaseID, 0, nil); err != nil {
			t.Fatalf("bob's release ended alice's lease: %v", err)
		}
	})
}

// deleteWithHook deletes id from the store and runs the lease hook, as the
// entity manager's rename/delete fan-out does on a real delete.
func deleteWithHook(t *testing.T, e *leaseEnv, id string) {
	t.Helper()
	ctx := context.Background()
	if _, err := e.st.DeleteFamily(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if err := e.leases.EntityDeleted(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func releaseOK(t *testing.T, cs *mcpgo.ClientSession, leaseID string) {
	t.Helper()
	text, isError := callToolOn(t, cs, "release_lease", `{"lease_id":"`+leaseID+`"}`)
	if isError || text != "{\n  \"released\": true\n}" {
		t.Fatalf("release_lease = %q (error %v)", text, isError)
	}
}

func TestRenewLease_StatusAndTTL(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	cs := e.session(t, leaseAlice)
	c := claim(t, cs, `{"id":"TKT-1","status":"first"}`)
	status := func() string {
		t.Helper()
		got, err := e.leases.GetReadable(context.Background(), []string{"TKT-1"})
		if err != nil {
			t.Fatal(err)
		}
		return got["TKT-1"].Status
	}
	steps := []struct {
		args, want string
	}{
		{`"ttl_seconds":60`, "first"},
		{`"status":"second"`, "second"},
		{`"status":""`, ""},
	}
	// The steps share one lease and run in order, so they form one
	// subtest. The padded lease id checks that it is trimmed.
	t.Run("status steps", func(t *testing.T) {
		t.Parallel()
		for _, s := range steps {
			text, isError := callToolOn(t, cs, "renew_lease", `{"lease_id":" `+c.LeaseID+` ",`+s.args+`}`)
			if isError {
				t.Fatalf("renew %s: %s", s.args, text)
			}
			var r renewResult
			if err := json.Unmarshal([]byte(text), &r); err != nil || r.Token != c.Token ||
				strings.Contains(text, "lease_id") {

				t.Fatalf("renew result %q (%v)", text, err)
			}
			if got := status(); got != s.want {
				t.Fatalf("after renew %s status = %q, want %q", s.args, got, s.want)
			}
		}
	})
	t.Run("ttl null is the default", func(t *testing.T) {
		t.Parallel()
		text, isError := callToolOn(t, cs, "renew_lease", `{"lease_id":"`+c.LeaseID+`","ttl_seconds":null}`)
		if isError {
			t.Fatalf("renew with null ttl: %s", text)
		}
		var r renewResult
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			t.Fatal(err)
		}
		if want := leaseTime(e.clock.Now().Add(lease.DefaultTTL)); r.ExpiresAt != want {
			t.Fatalf("expires_at = %s, want %s", r.ExpiresAt, want)
		}
	})
	for _, bad := range []string{
		`"ttl_seconds":10`, `"ttl_seconds":0`, `"ttl_seconds":7200`, `"ttl_seconds":"60"`,
		`"ttl_seconds":60.5`, `"status":5`,
	} {
		t.Run("bad "+bad, func(t *testing.T) {
			t.Parallel()
			if text, isError := callToolOn(t, cs, "renew_lease", `{"lease_id":"`+c.LeaseID+`",`+bad+`}`); !isError {
				t.Errorf("renew %s = %q, want an error", bad, text)
			}
		})
	}
	t.Run("malformed lease id", func(t *testing.T) {
		t.Parallel()
		if text, isError := callToolOn(t, cs, "renew_lease", `{"lease_id":"not-a-lease"}`); !isError ||
			!strings.Contains(text, "malformed lease id") {

			t.Errorf("malformed lease id = %q", text)
		}
	})
}

// AC5 and AC7: show_entity carries the lease without expiry or lease id;
// list_leases lists own leases without the lease id.
func TestShowEntity_CarriesLease(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	alice, bob := e.session(t, leaseAlice), e.session(t, leaseBob)
	if show, _ := callToolOn(t, bob, "show_entity", `{"id":"TKT-1"}`); strings.Contains(show, `"lease"`) {
		t.Fatalf("unheld entity shows a lease: %s", show)
	}
	c := claim(t, alice, `{"id":"TKT-1","label":"bot-a","status":"on it"}`)

	show, isError := callToolOn(t, bob, "show_entity", `{"id":"TKT-1"}`)
	if isError {
		t.Fatal(show)
	}
	var got struct {
		Lease map[string]any `json:"lease"`
	}
	if err := json.Unmarshal([]byte(show), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"user": "alice", "tool": principal.ToolMCP, "label": "bot-a", "status": "on it"}
	for k, v := range want {
		if got.Lease[k] != v {
			t.Errorf("lease[%s] = %v, want %v", k, got.Lease[k], v)
		}
	}
	if _, ok := got.Lease["since"]; !ok || len(got.Lease) != len(want)+1 {
		t.Errorf("lease = %v, want exactly user, tool, label, since, status", got.Lease)
	}
	if strings.Contains(show, c.LeaseID) || strings.Contains(show, "expires") {
		t.Errorf("show_entity leaks the lease id or expiry: %s", show)
	}

	list, isError := callToolOn(t, alice, "list_leases", `{}`)
	if isError || strings.Contains(list, c.LeaseID) || strings.Contains(list, "lease_id") {
		t.Fatalf("list_leases = %s", list)
	}
	var rows []ownLeaseJSON
	if err := json.Unmarshal([]byte(list), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "TKT-1" || rows[0].ExpiresAt != c.ExpiresAt || rows[0].Label != "bot-a" ||
		rows[0].Status != "on it" {

		t.Fatalf("list_leases rows = %+v", rows)
	}
	if list, _ := callToolOn(t, bob, "list_leases", `{}`); list != "[]" {
		t.Fatalf("bob's list_leases = %s, want []", list)
	}
}

// AC6: a watch on show_entity returns when the lease is released, although
// no store event fires.
func TestWatch_WakesOnLeaseRelease(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	alice, bob := e.session(t, leaseAlice), e.session(t, leaseBob)
	c := claim(t, alice, `{"id":"TKT-1"}`)
	target := `{"tool":"show_entity","args":{"id":"TKT-1"}}`
	base := mustWatch(t, bob, `{"targets":[`+target+`]}`)
	later(t, 300*time.Millisecond, func() error {
		return e.leases.Release(e.bound(t, leaseAlice), c.LeaseID)
	})
	got := mustWatch(t, bob, watchArgs(base.Cursor, 10, target))
	if got.TimedOut || len(got.Changed) != 1 {
		t.Fatalf("watch after release = %+v, want changed [0]", got)
	}
}

// A wait ends on every lease change show_entity shows, and only on those.
func TestWatch_LeaseChanges(t *testing.T) {
	t.Parallel()
	target := `{"tool":"show_entity","args":{"id":"TKT-1"}}`
	cases := []struct {
		name string
		// setup runs before the baseline and returns the change to make
		// while the watch waits.
		setup       func(t *testing.T, e *leaseEnv) func() error
		wantChanged bool
	}{
		{"claim ends the wait", func(_ *testing.T, e *leaseEnv) func() error {
			return func() error {
				_, err := e.leases.Claim(e.bound(t, leaseAlice),
					lease.ClaimRequest{Addr: "TKT-1"})
				return err
			}
		}, true},
		{"status renew ends the wait", func(t *testing.T, e *leaseEnv) func() error {
			t.Helper()
			c := claimDirect(t, e)
			status := "new status"
			return func() error {
				_, err := e.leases.Renew(e.bound(t, leaseAlice), c.LeaseID, 0, &status)
				return err
			}
		}, true},
		{"plain renew changes nothing", func(t *testing.T, e *leaseEnv) func() error {
			t.Helper()
			c := claimDirect(t, e)
			return func() error {
				_, err := e.leases.Renew(e.bound(t, leaseAlice), c.LeaseID, 0, nil)
				return err
			}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newLeaseEnv(t)
			bob := e.session(t, leaseBob)
			change := tc.setup(t, e)
			base := mustWatch(t, bob, `{"targets":[`+target+`]}`)
			later(t, 300*time.Millisecond, change)
			timeout := 10.0
			if !tc.wantChanged {
				timeout = 2
			}
			got := mustWatch(t, bob, watchArgs(base.Cursor, timeout, target))
			if tc.wantChanged && (got.TimedOut || len(got.Changed) != 1) {
				t.Fatalf("watch = %+v, want changed [0]", got)
			}
			if !tc.wantChanged && (!got.TimedOut || len(got.Changed) != 0) {
				t.Fatalf("watch = %+v, want a timeout", got)
			}
		})
	}
}

// claimDirect claims TKT-1 as alice through the service.
func claimDirect(t *testing.T, e *leaseEnv) lease.Claimed {
	t.Helper()
	c, err := e.leases.Claim(e.bound(t, leaseAlice), lease.ClaimRequest{Addr: "TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// An expiry seen by the lease service's Watch ends a wait, although no
// store event and no lease call happens.
func TestWatch_WakesOnLeaseExpiry(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	bob := e.session(t, leaseBob)
	c := claimDirect(t, e)
	// Watch arms its expiry timer one second out. The clock moves past the
	// expiry before the timer fires, so the replay it causes sees no lease.
	e.clock.set(c.Lease.Expires.Add(-time.Second))
	target := `{"tool":"show_entity","args":{"id":"TKT-1"}}`
	base := mustWatch(t, bob, `{"targets":[`+target+`]}`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.leases.Watch(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("lease Watch: %v", err)
		}
	})
	later(t, 200*time.Millisecond, func() error {
		e.clock.advance(2 * time.Second)
		return nil
	})
	got := mustWatch(t, bob, watchArgs(base.Cursor, 10, target))
	if got.TimedOut || len(got.Changed) != 1 {
		t.Fatalf("watch after expiry = %+v, want changed [0]", got)
	}
}

// Without a lease service a waiting watch still wakes on a store write; the
// nil lease channel in its select never fires.
func TestWatch_NilLeasesWakesOnStoreWrite(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	without := e.deps
	without.Leases = nil
	srv, err := NewServer(without, "test", WithPrincipal(leaseAlice))
	if err != nil {
		t.Fatal(err)
	}
	cs := connect(t, srv)
	target := `{"tool":"show_entity","args":{"id":"TKT-1"}}`
	base := mustWatch(t, cs, `{"targets":[`+target+`]}`)
	later(t, 300*time.Millisecond, func() error {
		return setTitle(context.Background(), e.st, "TKT-1", "ticket", "retitled")
	})
	got := mustWatch(t, cs, watchArgs(base.Cursor, 10, target))
	if got.TimedOut || len(got.Changed) != 1 {
		t.Fatalf("watch after a store write = %+v, want changed [0]", got)
	}
}

// AC7: no log line carries a lease id, on the success paths and on the error
// paths of renew and release.
//
//nolint:paralleltest // replaces the default logger
func TestLeaseTools_LeaseIDNeverLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&lockedWriter{w: &buf}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	e := newLeaseEnv(t)
	cs := e.session(t, leaseAlice)
	c := claim(t, cs, `{"id":"TKT-1","status":"secret status"}`)
	callToolOn(t, cs, "show_entity", `{"id":"TKT-1"}`)
	callToolOn(t, cs, "list_leases", `{}`)
	callToolOn(t, cs, "renew_lease", `{"lease_id":"`+c.LeaseID+`","ttl_seconds":5}`)
	// Revoking alice makes the renew release the lease, which the service
	// logs at debug.
	k := entity.RelationKey{From: "alice", Type: "member-of", To: "editors"}
	if err := e.st.DeleteRelation(context.Background(), k); err != nil {
		t.Fatal(err)
	}
	callToolOn(t, cs, "renew_lease", `{"lease_id":"`+c.LeaseID+`"}`)
	callToolOn(t, cs, "release_lease", `{"lease_id":"`+c.LeaseID+`"}`)

	// A failing service: the handler logs the failure, never the lease id.
	failing := e.deps
	failing.Leases = failingLeases{LeaseService: e.leases}
	srv, err := NewServer(failing, "test", WithPrincipal(leaseAlice))
	if err != nil {
		t.Fatal(err)
	}
	fcs := connect(t, srv)
	for _, tool := range []string{"renew_lease", "release_lease"} {
		text, isError := callToolOn(t, fcs, tool, `{"lease_id":"`+c.LeaseID+`"}`)
		if !isError || text != lease.FailedText {
			t.Fatalf("%s on a failing service = %q", tool, text)
		}
	}

	logs := buf.String()
	if !strings.Contains(logs, "store unavailable") || !strings.Contains(logs, "user=alice") {
		t.Fatalf("the failure was not logged with the user: %s", logs)
	}
	for _, secret := range []string{c.LeaseID, "secret status"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("logs carry %q:\n%s", secret, logs)
		}
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// failingLeases fails renew and release as a broken backend would.
type failingLeases struct{ LeaseService }

var errLeaseStore = errors.New("store unavailable")

func (failingLeases) Renew(context.Context, string, time.Duration, *string) (lease.Lease, error) {
	return lease.Lease{}, errLeaseStore
}

func (failingLeases) Release(context.Context, string) error { return errLeaseStore }

// AC9: without a lease service there are no lease tools and show_entity
// carries no lease; after a reload drops the service the tools answer that
// leases are off.
func TestLeases_NilService(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	without := e.deps
	without.Leases = nil
	srv, err := NewServer(without, "test", WithPrincipal(leaseAlice))
	if err != nil {
		t.Fatal(err)
	}
	cs := connect(t, srv)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if strings.Contains(tool.Name, "lease") || tool.Name == "claim_entity" {
			t.Errorf("tool %s registered without a lease service", tool.Name)
		}
	}
	if _, claimErr := e.leases.Claim(e.bound(t, leaseAlice),
		lease.ClaimRequest{Addr: "TKT-1"}); claimErr != nil {
		t.Fatal(claimErr)
	}
	if show, _ := callToolOn(t, cs, "show_entity", `{"id":"TKT-1"}`); strings.Contains(show, `"lease"`) {
		t.Errorf("show_entity without a lease service = %s", show)
	}

	withSrv, err := NewServer(e.deps, "test", WithPrincipal(leaseAlice))
	if err != nil {
		t.Fatal(err)
	}
	if err := withSrv.ReloadDeps(without); err != nil {
		t.Fatal(err)
	}
	wcs := connect(t, withSrv)
	for _, call := range [][2]string{
		{"claim_entity", `{"id":"TKT-2"}`},
		{"renew_lease", `{"lease_id":"` + strings.Repeat("a", 32) + `"}`},
		{"release_lease", `{"lease_id":"` + strings.Repeat("a", 32) + `"}`},
		{"list_leases", `{}`},
	} {
		if text, isError := callToolOn(t, wcs, call[0], call[1]); !isError || text != lease.NotEnabledText {
			t.Errorf("%s after reload = %q", call[0], text)
		}
	}
	if show, _ := callToolOn(t, wcs, "show_entity", `{"id":"TKT-1"}`); strings.Contains(show, `"lease"`) {
		t.Errorf("show_entity after reload = %s", show)
	}
	if got := mustWatch(t, wcs, `{"targets":[{"tool":"show_entity","args":{"id":"TKT-1"}}],"timeout":0}`); got.Cursor == "" {
		t.Errorf("watch after reload = %+v", got)
	}
}

func TestClaimEntity_InputErrors(t *testing.T) {
	t.Parallel()
	e := newLeaseEnv(t)
	cs := e.session(t, leaseAlice)
	cases := []struct {
		name, args, want string
	}{
		{"ttl too short", `{"id":"TKT-1","ttl_seconds":1}`, "ttl_seconds"},
		{"ttl huge", `{"id":"TKT-1","ttl_seconds":1e300}`, "ttl_seconds"},
		{"bad label", `{"id":"TKT-1","label":"has space"}`, "label"},
		{"control in status", `{"id":"TKT-1","status":"a\nb"}`, "control characters"},
		{"missing id", `{}`, "id"},
		{"ttl zero", `{"id":"TKT-1","ttl_seconds":0}`, "ttl_seconds"},
		{"ttl fractional", `{"id":"TKT-1","ttl_seconds":60.5}`, "ttl_seconds"},
		{"label not a string", `{"id":"TKT-1","label":5}`, "label"},
		{"status not a string", `{"id":"TKT-1","status":true}`, "status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			text, isError := callToolOn(t, cs, "claim_entity", tc.args)
			if !isError || !strings.Contains(text, tc.want) {
				t.Fatalf("claim %s = %q, want an error mentioning %q", tc.args, text, tc.want)
			}
		})
	}
}

func TestLeaseFailed_Texts(t *testing.T) {
	t.Parallel()
	leaseLimitText, _ := lease.UserText(lease.ErrLimit, leaseListHint)
	ctx := principal.With(context.Background(), leaseAlice)
	invalid := fmt.Errorf("%w: label too long", lease.ErrInvalid)
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"lost", lease.ErrLost, lease.LostText},
		{"wrapped lost", fmt.Errorf("x: %w", lease.ErrLost), lease.LostText},
		{"limit", lease.ErrLimit, leaseLimitText},
		{"full", lease.ErrFull, lease.FullText},
		{"invalid", invalid, invalid.Error()},
		{"no principal", lease.ErrNoPrincipal, lease.ErrNoPrincipal.Error()},
		{"unexpected", errLeaseStore, lease.FailedText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := leaseFailed(ctx, "test", tc.err)
			if text := res.Content[0].(*mcpgo.TextContent).Text; !res.IsError || text != tc.want {
				t.Errorf("leaseFailed(%v) = %q, want %q", tc.err, text, tc.want)
			}
		})
	}
	t.Run("limit text names the shared cap", func(t *testing.T) {
		t.Parallel()
		if !strings.Contains(leaseLimitText, "32") || !strings.Contains(leaseLimitText, "every agent running as you") {
			t.Errorf("limit text %q must name the shared cap", leaseLimitText)
		}
	})
	t.Run("ambiguous address", func(t *testing.T) {
		t.Parallel()
		amb := claimFailed(ctx, "DOC-1", &lease.AmbiguousError{
			Err: &visibility.AmbiguousAddressError{ID: "DOC-1", Faces: []entity.Face{"a", "b"}},
		})
		if text := amb.Content[0].(*mcpgo.TextContent).Text; !strings.Contains(text, "DOC-1") {
			t.Errorf("ambiguous text = %q", text)
		}
	})
}
