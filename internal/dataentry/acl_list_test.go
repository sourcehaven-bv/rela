package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestACLList_TypeLevelReadGrant pins TKT-VMD8 AC1: a role with
// `read: [ticket]` sees every ticket via the list endpoint and an
// empty list for any type without a grant.
func TestACLList_TypeLevelReadGrant(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "T2"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 2 || resp.Meta.Total != 2 {
		t.Errorf("tickets: got %d entities (total %d), want 2", len(resp.Data), resp.Meta.Total)
	}

	resp, rec = listEntitiesAs(aliceCtx(), t, app, d, "feature", "features", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET features: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 || resp.Meta.Total != 0 {
		t.Errorf("features (no grant): got %d entities (total %d), want 0", len(resp.Data), resp.Meta.Total)
	}
}

// TestACLList_RoleRelationInheritance pins TKT-VMD8 AC2: an
// `editor-of` relation confers read on tickets reachable through
// `belongs-to` containment — the list returns only the visible subset.
func TestACLList_RoleRelationInheritance(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Visible"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-002", "belongs-to", "PRJ-9"))

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Errorf("expected exactly [TKT-001], got %+v", resp.Data)
	}
	if resp.Meta.Total != 1 {
		t.Errorf("meta.total = %d, want 1", resp.Meta.Total)
	}
}

// TestACLList_PaginationLeakSurfaces pins TKT-VMD8 AC3: with 5
// visible + 5 hidden tickets and per_page=3&page=2, every pagination
// surface reflects the post-filter count of 5 — and the hidden total
// of 10 appears nowhere in the response (RR-KNGC + RR-VDTW).
func TestACLList_PaginationLeakSurfaces(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("TKT-V%02d", i)
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
		seedRelation(app, entity.NewRelation(id, "belongs-to", "PRJ-42"))
	}
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("TKT-H%02d", i)
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
		seedRelation(app, entity.NewRelation(id, "belongs-to", "PRJ-9"))
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "per_page=3&page=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets: %d %s", rec.Code, rec.Body)
	}

	if len(resp.Data) != 2 {
		t.Errorf("data.length = %d, want 2", len(resp.Data))
	}
	if resp.Meta.Total != 5 {
		t.Errorf("meta.total = %d, want 5", resp.Meta.Total)
	}
	if resp.Meta.HasMore {
		t.Error("meta.has_more = true, want false")
	}
	if got := rec.Header().Get("X-Total-Count"); got != "5" {
		t.Errorf("X-Total-Count = %q, want 5", got)
	}
	if got := rec.Header().Get("X-Page"); got != "2" {
		t.Errorf("X-Page = %q, want 2", got)
	}
	if got := rec.Header().Get("X-Per-Page"); got != "3" {
		t.Errorf("X-Per-Page = %q, want 3", got)
	}
	link := rec.Header().Get("Link")
	if strings.Contains(link, `rel="next"`) {
		t.Errorf(`Link carries rel="next" beyond the last visible page: %s`, link)
	}
	if !strings.Contains(link, `page=2&per_page=3>; rel="last"`) {
		t.Errorf(`Link rel="last" should point at page=2: %s`, link)
	}

	// The hidden total (10) must not be derivable from any surface:
	// not the body, not a header. "1" alone is fine; the literal "10"
	// anywhere is the leak.
	if strings.Contains(rec.Body.String(), "10") {
		t.Errorf("body mentions the hidden total 10: %s", rec.Body)
	}
	for name, vals := range rec.Header() {
		for _, v := range vals {
			if strings.Contains(v, "10") {
				t.Errorf("header %s mentions the hidden total 10: %s", name, v)
			}
		}
	}
}

// TestACLList_DenyAllShape pins TKT-VMD8 AC4: a principal with no
// read grant gets the empty-list shape — zero data, zero totals,
// zeroed pagination headers, and `_actions.create == false` (the
// policy-load invariant guarantees no write grant exists either).
func TestACLList_DenyAllShape(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"feature"}}},
		Assignments: map[string]string{"mallory": "viewer"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(principalCtx("mallory"), t, app, d, "ticket", "tickets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets (deny): %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 || resp.Meta.Total != 0 {
		t.Errorf("deny-all: got %d entities (total %d), want 0", len(resp.Data), resp.Meta.Total)
	}
	if got := rec.Header().Get("X-Total-Count"); got != "0" {
		t.Errorf("X-Total-Count = %q, want 0", got)
	}
	if create, ok := resp.Actions["create"]; !ok || create {
		t.Errorf("_actions.create = %v (present=%v), want false", create, ok)
	}
}

// TestACLList_DenyAllSearchShortCircuit pins TKT-VMD8 AC5 (RR-X56H):
// the DenyAll verdict resolves BEFORE the search backend runs, so a
// denied principal cannot probe backend latency (or induce load)
// through ?q=. Asserted via a recording searcher: zero calls.
func TestACLList_DenyAllSearchShortCircuit(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha"}})

	var calls []string
	app.searcher = recordingSearcher{log: &calls}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"feature"}}},
		Assignments: map[string]string{"mallory": "viewer"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(principalCtx("mallory"), t, app, d, "ticket", "tickets", "q=alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets (deny, q=): %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 {
		t.Errorf("deny-all with q=: got %d entities, want 0", len(resp.Data))
	}
	if len(calls) != 0 {
		t.Errorf("search backend invoked %d times on DenyAll path, want 0 (calls: %v)", len(calls), calls)
	}
}

// TestACLList_SearchAfterACLOrdering pins TKT-VMD8 AC9 (RR-WX77 +
// RR-3IO2): on the composed-Query path the ACL GraphQuery executes
// BEFORE the free-text searcher, so search only ever intersects the
// visible subset. Call order is recorded by wrapping both the store
// and the searcher with the same log.
func TestACLList_SearchAfterACLOrdering(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha one"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))

	var calls []string
	app.store = orderRecordingStore{Store: app.store, log: &calls}
	app.searcher = recordingSearcher{log: &calls, hits: []string{"TKT-001"}}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "q=alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tickets: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Errorf("expected [TKT-001], got %+v", resp.Data)
	}

	gqIdx, searchIdx := -1, -1
	for i, c := range calls {
		switch c {
		case "graphquery":
			if gqIdx == -1 {
				gqIdx = i
			}
		case "search":
			if searchIdx == -1 {
				searchIdx = i
			}
		}
	}
	if gqIdx == -1 || searchIdx == -1 {
		t.Fatalf("expected both graphquery and search calls, got %v", calls)
	}
	if gqIdx > searchIdx {
		t.Errorf("search ran before the ACL GraphQuery: %v", calls)
	}
}

// TestACLList_QueryErrorMapping pins the errACLListQuery routing: a
// failing ACL GraphQuery surfaces as 500 acl_query_failed via
// writeGateError, not as the misleading search_failed shape.
func TestACLList_QueryErrorMapping(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
	}, app.store)
	app.acl = d

	// The gate context is built from the REAL store (so the ACL
	// resolver works), then the app's store is swapped for a failing
	// one — only the list-path GraphQuery sees the failure.
	ctx := gateCtxFor(aliceCtx(), t, d)
	app.store = failingGraphQueryStore{Store: app.store}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "acl_query_failed") {
		t.Errorf("body missing acl_query_failed: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "search_failed") {
		t.Errorf("ACL failure mislabeled as search_failed: %s", rec.Body)
	}
}

// TestACLPosition_SearchScopeGated pins the CRIT finding from the
// TKT-VMD8 code review: `_position` with `source=search` runs
// executeQuery, which is itself ungated — without the readableSubset
// filter a denied principal reads hidden cardinality from `total`
// and harvests hidden {id, type} pairs from prev/next.
func TestACLPosition_SearchScopeGated(t *testing.T) {
	app := newTestAppV1(t)
	// Both entities match the free-text query "alpha"; only the
	// ticket is readable by alice.
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha ticket"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "alpha feature"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	scope := url.QueryEscape(`{"source":"search","q":"alpha"}`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_position?id=TKT-001&scope="+scope, http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("position: %d %s", rec.Code, rec.Body)
	}
	var pos V1Position
	if err := json.Unmarshal(rec.Body.Bytes(), &pos); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pos.Total != 1 {
		t.Errorf("total = %d, want 1 (hidden FEAT-001 must not count)", pos.Total)
	}
	if pos.Prev != nil || pos.Next != nil {
		t.Errorf("prev/next leak hidden neighbors: prev=%+v next=%+v", pos.Prev, pos.Next)
	}

	// The hidden id itself must 404 out of the scope entirely.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/_position?id=FEAT-001&scope="+scope, http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec = httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("hidden id in search scope: got %d, want 404 not_in_scope", rec.Code)
	}
}

// TestACLList_AllowAllLoadErrorSurfaces pins the errListLoad mapping:
// a mid-stream store failure on the AllowAll path surfaces as 500
// list_load_failed rather than a silently truncated 200.
func TestACLList_AllowAllLoadErrorSurfaces(t *testing.T) {
	app := newTestAppV1(t)
	app.store = failingListStore{Store: app.store}

	// No gate on ctx → nopReadGate → AllowAll path.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "list_load_failed") {
		t.Errorf("body missing list_load_failed: %s", rec.Body)
	}
}

// TestACLList_VaryHeader pins TKT-VMD8 AC10 (RR-VDTW): when a
// principal header is configured, /api/ responses carry both the
// no-store Cache-Control and `Vary: <header>` so no cache layer can
// serve principal A's filtered response to principal B.
func TestACLList_VaryHeader(t *testing.T) {
	app := newTestAppV1(t)
	app.SetPrincipalHeader("X-Forwarded-User")

	probe := app.noCacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	probe.ServeHTTP(rec, req)

	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store directive", cc)
	}
	if vary := rec.Header().Get("Vary"); vary != "X-Forwarded-User" {
		t.Errorf("Vary = %q, want X-Forwarded-User", vary)
	}

	// Without a configured header no Vary is emitted — the pre-ACL
	// wire shape stays untouched for single-user deployments.
	app2 := newTestAppV1(t)
	rec2 := httptest.NewRecorder()
	app2.noCacheMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))
	if vary := rec2.Header().Get("Vary"); vary != "" {
		t.Errorf("Vary without configured header = %q, want empty", vary)
	}
}

// ---- helpers ----

// listEntitiesAs performs a list request through handleV1ListEntities
// with the readGate + acl.Request attached, mirroring the production
// middleware. Returns the decoded response plus the raw recorder for
// header asserts.
func listEntitiesAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, rawQuery string,
) (V1ListResponse, *httptest.ResponseRecorder) {
	t.Helper()
	url := "/api/v1/" + plural
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, typeName, plural)

	var resp V1ListResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode list response: %v\nbody: %s", err, rec.Body)
		}
	}
	return resp, rec
}

// recordingSearcher logs every Search call and yields the configured
// hits. Used to pin the DenyAll short-circuit (zero calls) and the
// search-after-ACL ordering.
type recordingSearcher struct {
	log  *[]string
	hits []string
}

func (s recordingSearcher) Search(context.Context, search.Query) iter.Seq2[search.Hit, error] {
	*s.log = append(*s.log, "search")
	return func(yield func(search.Hit, error) bool) {
		for _, id := range s.hits {
			if !yield(search.Hit{ID: id}, nil) {
				return
			}
		}
	}
}

// orderRecordingStore wraps a store.Store and logs GraphQuery calls
// into the shared log so call ordering against the searcher can be
// asserted.
type orderRecordingStore struct {
	store.Store
	log *[]string
}

func (s orderRecordingStore) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	*s.log = append(*s.log, "graphquery")
	return s.Store.GraphQuery(ctx, q)
}

// failingGraphQueryStore wraps a store.Store with a GraphQuery that
// yields an immediate error — drives the errACLListQuery mapping.
type failingGraphQueryStore struct {
	store.Store
}

func (s failingGraphQueryStore) GraphQuery(context.Context, store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		yield(nil, errors.New("synthetic graph query failure"))
	}
}

// failingListStore wraps a store.Store with a ListEntities that
// yields an immediate error — drives the errListLoad mapping on the
// AllowAll path.
type failingListStore struct {
	store.Store
}

func (s failingListStore) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		yield(nil, errors.New("synthetic list failure"))
	}
}
