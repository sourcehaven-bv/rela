package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// searchAs performs GET /api/v1/_search?q=<q> with the readGate +
// acl.Request attached, mirroring listEntitiesAs for the search view.
func searchAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative, q string) (v1.ListResponse, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q="+q, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	var resp v1.ListResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode search response: %v\nbody: %s", err, rec.Body)
		}
	}
	return resp, rec
}

// TestACLSearch_TypeLevelGrant pins TKT-BA8BSX AC1 + AC3: a role with
// `read: [ticket]` sees matching tickets through /_search and nothing
// else — the raw body carries no hidden ID, title, or property value.
// A principal with no grants at all gets the empty shape.
func TestACLSearch_TypeLevelGrant(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha rocket"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "alpha lander"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{
		"title": "alpha hidden-feature-title", "status": "secret-status-value",
	}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	resp, rec := searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 2 || resp.Meta.Total != 2 {
		t.Errorf("got %d hits (total %d), want the 2 tickets", len(resp.Data), resp.Meta.Total)
	}
	for _, leak := range []string{"FEAT-001", "hidden-feature-title", "secret-status-value"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("hidden value %q leaked into the search body", leak)
		}
	}

	// bob has no assignment → no roles → every type DenyAll.
	resp, rec = searchAs(principalCtx("bob"), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search as bob: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 || resp.Meta.Total != 0 {
		t.Errorf("denied principal: got %d hits (total %d), want 0", len(resp.Data), resp.Meta.Total)
	}
}

// TestACLSearch_RoleRelationInheritance pins TKT-BA8BSX AC2: the
// editor-of / belongs-to world — search returns the PRJ-42 ticket and
// never the PRJ-9 one, even though both match the text.
func TestACLSearch_RoleRelationInheritance(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha visible"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "alpha covert-ticket-title"}})
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

	resp, rec := searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Fatalf("expected exactly [TKT-001], got %+v", resp.Data)
	}
	for _, leak := range []string{"TKT-002", "covert-ticket-title"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("hidden value %q leaked into the search body", leak)
		}
	}
}

// TestACLSearch_VisibleHitRelatedToHidden pins TKT-BA8BSX AC3b
// (RR-QO01XY): a VISIBLE search hit that relates to a HIDDEN entity
// must not expose the hidden entity's ID or title through any field
// of its serialized body. This is the serializer-level half of the
// no-leak guarantee — the conformance suite can't see it because it
// asserts on search.Hit, not on the wire shape. The invariant rests
// on handleV1Search keeping includeRelations=false; if that flips,
// this test fails before a leak ships.
func TestACLSearch_VisibleHitRelatedToHidden(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha visible"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-SECRET", Type: "project", Properties: map[string]any{"title": "ultra-hidden-project"}})
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-SECRET"))

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	resp, rec := searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Fatalf("expected the visible ticket, got %+v", resp.Data)
	}
	for _, leak := range []string{"PRJ-SECRET", "ultra-hidden-project"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("related hidden entity value %q leaked through a visible hit", leak)
		}
	}
}

// TestACLSearch_DenyAllShortCircuit pins TKT-BA8BSX AC4 (RR-599CLE):
// an all-effective-DenyAll scope returns before the search backend is
// touched (no timing probe), and — the positive control — a single
// granted type invokes the backend exactly once.
func TestACLSearch_DenyAllShortCircuit(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha"}})

	var calls []string
	app.searcher = recordingSearcher{log: &calls}
	rebindVisibleSearcher(t, app)

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// bob: no roles → empty scope → short-circuit before the backend.
	_, rec := searchAs(principalCtx("bob"), t, app, d, "anything")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search as bob: %d %s", rec.Code, rec.Body)
	}
	if len(calls) != 0 {
		t.Errorf("search backend invoked %d times on the DenyAll path, want 0 (calls: %v)", len(calls), calls)
	}

	// alice: one granted type → the backend runs exactly once.
	_, rec = searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search as alice: %d %s", rec.Code, rec.Body)
	}
	if got := len(calls); got != 1 {
		t.Errorf("search backend invoked %d times for a granted search, want exactly 1 (calls: %v)", got, calls)
	}
}

// failingMatchingIDsStore makes every MatchingIDs call fail with a
// synthetic backend error, simulating an ACL scope-evaluation failure
// inside the visible-search pipeline.
type failingMatchingIDsStore struct {
	store.Store
	err error
}

func (s failingMatchingIDsStore) MatchingIDs(context.Context, store.GraphQuery, []string) (map[string]bool, error) {
	return nil, s.err
}

// typedHitSearcher yields fully-typed hits so the visible wrapper has
// something to probe MatchingIDs with.
type typedHitSearcher struct{ hits []search.Hit }

func (s typedHitSearcher) Search(context.Context, search.Query) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		for _, h := range s.hits {
			if !yield(h, nil) {
				return
			}
		}
	}
}

// aclSearchQueryVerdictWorld builds the editor-of world where alice's
// ticket verdict is a composed Query (not AllowAll), so the visible
// wrapper must call MatchingIDs.
func aclSearchQueryVerdictWorld(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d
	return d
}

// TestACLSearch_ScopeErrorMapping pins TKT-BA8BSX AC7 + AC7b for the
// visibility-failure class: a MatchingIDs error surfaces as 500
// acl_query_failed with the constant detail — the raw backend string
// (which can name tables/columns) never reaches the wire — and the
// executeQuery error wraps errACLListQuery so the _position consumer
// keeps the writeGateError mapping.
func TestACLSearch_ScopeErrorMapping(t *testing.T) {
	app := newTestAppV1(t)
	d := aclSearchQueryVerdictWorld(t, app)

	const synthetic = `pq: relation "secret_internal_acl_table" does not exist`
	failing := failingMatchingIDsStore{Store: app.store, err: errors.New(synthetic)}
	app.searcher = typedHitSearcher{hits: []search.Hit{{ID: "TKT-001", Type: "ticket", Title: "alpha"}}}
	v, err := search.NewVisible(app.searcher, failing)
	if err != nil {
		t.Fatalf("NewVisible: %v", err)
	}
	app.visibleSearcher = v

	// Wire-level mapping (AC7).
	_, rec := searchAs(aliceCtx(), t, app, d, "alpha")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on scope failure, got %d (body: %s)", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "acl_query_failed") {
		t.Errorf("expected acl_query_failed code, body: %s", body)
	}
	if !strings.Contains(body, "check server logs") {
		t.Errorf("expected constant detail, body: %s", body)
	}
	if strings.Contains(body, "secret_internal_acl_table") || strings.Contains(body, "pq:") {
		t.Errorf("raw backend error leaked to the wire: %s", body)
	}

	// Sentinel wrapping (AC7b): the _position consumer relies on it.
	_, qErr := app.executeQuery(gateCtxFor(aliceCtx(), t, d), "alpha")
	if !errors.Is(qErr, errACLListQuery) {
		t.Errorf("executeQuery scope error = %v, want errACLListQuery wrap", qErr)
	}
}

// TestACLSearch_CanceledScopeStaysSilent pins the AC7b cancel branch:
// a context.Canceled surfacing through the visibility pipeline writes
// NO response body on /_position?scope=search (client is gone — same
// behavior readableSubset had before it was retired).
func TestACLSearch_CanceledScopeStaysSilent(t *testing.T) {
	app := newTestAppV1(t)
	d := aclSearchQueryVerdictWorld(t, app)

	failing := failingMatchingIDsStore{Store: app.store, err: context.Canceled}
	app.searcher = typedHitSearcher{hits: []search.Hit{{ID: "TKT-001", Type: "ticket", Title: "alpha"}}}
	v, err := search.NewVisible(app.searcher, failing)
	if err != nil {
		t.Fatalf("NewVisible: %v", err)
	}
	app.visibleSearcher = v

	req := httptest.NewRequest(http.MethodGet,
		positionURL(t, "TKT-001", ScopeDescriptor{Source: "search", Q: "alpha"}), http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("canceled scope error must stay silent, got body: %s", rec.Body)
	}
}

// TestACLSearch_BackendErrorMapping pins the OTHER error class of AC7:
// a plain search-backend failure (not scope evaluation) maps to 500
// search_failed — still constant-detail, still no echo — and does NOT
// wrap errACLListQuery.
func TestACLSearch_BackendErrorMapping(t *testing.T) {
	app := newTestAppV1(t)
	const synthetic = "bleve: index file /var/secret/path corrupt"
	app.searcher = &fakeSearcher{err: errors.New(synthetic)}
	rebindVisibleSearcher(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=alpha", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on backend failure, got %d (body: %s)", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "search_failed") {
		t.Errorf("expected search_failed code, body: %s", body)
	}
	if strings.Contains(body, "/var/secret/path") {
		t.Errorf("raw backend error leaked to the wire: %s", body)
	}

	_, qErr := app.executeQuery(context.Background(), "alpha")
	if qErr == nil || errors.Is(qErr, errACLListQuery) {
		t.Errorf("backend error = %v, want non-nil and NOT errACLListQuery", qErr)
	}
}

// TestACLSearchRegression_NopACL pins TKT-BA8BSX AC9: with no ACL
// configured, /_search behaves exactly as before the gate existed —
// every matching entity comes back, INCLUDING entities whose type is
// not in the metamodel (permissive storage; the nop gate's wildcard
// scope is what keeps them visible — RR-SOU82P).
func TestACLSearchRegression_NopACL(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "alpha rocket"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "alpha feature"}})
	seedEntity(app, &entity.Entity{ID: "GHOST-001", Type: "ghost", Properties: map[string]any{"title": "alpha off-metamodel"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=alpha", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	var resp v1.ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body)
	}
	got := make(map[string]bool, len(resp.Data))
	for _, e := range resp.Data {
		got[e.ID] = true
	}
	for _, id := range []string{"TKT-001", "FEAT-001", "GHOST-001"} {
		if !got[id] {
			t.Errorf("NopACL search lost %s (off-metamodel types must stay visible without ACL)", id)
		}
	}
	if resp.Meta.Total != 3 || resp.Meta.Page != 1 || resp.Meta.PerPage != 3 {
		t.Errorf("meta = %+v, want {Total:3 Page:1 PerPage:3}", resp.Meta)
	}
}
