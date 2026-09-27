package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// traversalScopeSchema is the TKT-CXQEV0 case: a view on features filtered
// by the tickets that implement them, walking `implements` backwards.
const traversalScopeSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  feature:
    label: Feature
    plural: features
    id_prefix: FEAT
    properties:
      title: {type: string, required: true}
    query_scopes:
      busy: "related(entity, 'implementedBy', { status = 'in-progress' })"
      idle: "not related(entity, 'implementedBy', { status = 'in-progress' })"
      reported: "related(entity, {'implementedBy', 'reportedBy'})"
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: TKT
    properties:
      title: {type: string, required: true}
      status: {type: string}
  user:
    label: User
    plural: users
    id_prefix: USR
    properties:
      title: {type: string}
relations:
  implements: {label: implements, from: [ticket], to: [feature], inverse: implementedBy}
  editor-of: {label: editor of, from: [user], to: [ticket]}
  reports: {label: reports, from: [user], to: [ticket], inverse: reportedBy}
`

// newTraversalScopeApp seeds three features. FEAT-1 and FEAT-3 each have an
// in-progress ticket; FEAT-2's ticket is done. bob is editor of TKT-1 only,
// which is his sole route to reading a ticket.
func newTraversalScopeApp(t *testing.T, schema string) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	cfg := &dataentryconfig.Config{
		Forms:   make(map[string]dataentryconfig.Form),
		Lists:   make(map[string]dataentryconfig.List),
		Views:   make(map[string]dataentryconfig.ViewConfig),
		Kanbans: make(map[string]dataentryconfig.Kanban),
	}
	app := newAppFromParts(cfg, meta, newFixture())
	if err := app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)); err != nil {
		t.Fatalf("wire query scopes: %v", err)
	}
	for _, e := range []*entity.Entity{
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "one"}},
		{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "two"}},
		{ID: "FEAT-3", Type: "feature", Properties: map[string]any{"title": "three"}},
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "a", "status": "in-progress"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "b", "status": "done"}},
		{ID: "TKT-3", Type: "ticket", Properties: map[string]any{"title": "c", "status": "in-progress"}},
		{ID: "bob", Type: "user"},
	} {
		seedEntity(app, e)
	}
	for _, r := range []*entity.Relation{
		{From: "TKT-1", Type: "implements", To: "FEAT-1"},
		{From: "TKT-2", Type: "implements", To: "FEAT-2"},
		{From: "TKT-3", Type: "implements", To: "FEAT-3"},
		{From: "bob", Type: "editor-of", To: "TKT-1"},
		{From: "bob", Type: "reports", To: "TKT-1"},
	} {
		seedRelation(app, r)
	}
	return app
}

func traversalScopePolicy(t *testing.T, app *App) *acl.Declarative {
	t.Helper()
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":      {Read: []string{"feature", "ticket", "user"}},
			"features": {Read: []string{"feature", "user"}},
			"editor":   {Read: []string{"ticket"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		Assignments:   map[string]string{"alice": "all", "bob": "features", "carol": "features"},
	}, app.store)
	app.acl = d
	return d
}

func traversalScopeIDs(t *testing.T, app *App, d *acl.Declarative, user, scope string) []string {
	t.Helper()
	resp, rec := listEntitiesAs(principalCtx(user), t, app, d, "feature", "features", "query_scope="+scope)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", user, scope, rec.Code, rec.Body)
	}
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	return ids
}

// TestQueryScopeTraversal_EndToEnd drives an incoming traversal through the
// HTTP list endpoint under ACL. A ticket the principal cannot read is a
// nonexistent one: it neither makes its feature match `busy` nor stops it
// matching `idle`. The requests interleave principals on one app, so an
// answer cached across principals would show up as a wrong row.
func TestQueryScopeTraversal_EndToEnd(t *testing.T) {
	app := newTraversalScopeApp(t, traversalScopeSchema)
	d := traversalScopePolicy(t, app)

	steps := []struct {
		user, scope string
		want        []string
	}{
		{"alice", "busy", []string{"FEAT-1", "FEAT-3"}},
		{"bob", "busy", []string{"FEAT-1"}},
		{"carol", "busy", nil},
		{"alice", "busy", []string{"FEAT-1", "FEAT-3"}},
		{"bob", "idle", []string{"FEAT-2", "FEAT-3"}},
		{"carol", "idle", []string{"FEAT-1", "FEAT-2", "FEAT-3"}},
		{"alice", "idle", []string{"FEAT-2"}},
	}
	for _, s := range steps {
		got := traversalScopeIDs(t, app, d, s.user, s.scope)
		if strings.Join(got, ",") != strings.Join(s.want, ",") {
			t.Errorf("%s %s = %v, want %v", s.user, s.scope, got, s.want)
		}
	}
}

// With no ACL configured the traversal runs ungated.
func TestQueryScopeTraversal_NoACL(t *testing.T) {
	app := newTraversalScopeApp(t, traversalScopeSchema)
	got := scopeListIDsFor(t, app, "feature", "features", "query_scope=busy")
	if strings.Join(got, ",") != "FEAT-1,FEAT-3" {
		t.Errorf("busy = %v, want [FEAT-1 FEAT-3]", got)
	}
}

// The next-action count-zero hint applies the type's DEFAULT scope, so a
// default using related() runs its traversal there too, under the caller's
// gate (RR-3QEVFT).
func TestQueryScopeTraversal_CountZeroHintAppliesTheDefault(t *testing.T) {
	schema := strings.Replace(traversalScopeSchema,
		`busy: "related(`, `default: "related(`, 1)
	app := newTraversalScopeApp(t, schema)
	d := traversalScopePolicy(t, app)

	for _, tc := range []struct {
		user string
		zero bool
	}{
		{"alice", false},
		{"bob", false},
		// carol reads no ticket, so no feature passes the default scope.
		{"carol", true},
	} {
		zero, err := app.countIsZero(gateCtxFor(principalCtx(tc.user), t, d), "feature", false)
		if err != nil {
			t.Fatalf("%s: countIsZero: %v", tc.user, err)
		}
		if zero != tc.zero {
			t.Errorf("%s: countIsZero = %v, want %v", tc.user, zero, tc.zero)
		}
	}
}

func scopeListIDsFor(t *testing.T, app *App, typeName, plural, rawQuery string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural+"?"+rawQuery, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, typeName, plural)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	return idsFromListBody(t, rec)
}

// A chain the gate cannot place for THIS principal is refused as a named
// 422, not a generic 500. bob reads tickets through editor-of, which occupies
// the ticket's inbound slot the second incoming hop needs; alice reads them
// plainly, so the same scope works for her.
func TestQueryScopeTraversal_UnsupportedForPrincipalIs422(t *testing.T) {
	app := newTraversalScopeApp(t, traversalScopeSchema)
	d := traversalScopePolicy(t, app)

	if got := traversalScopeIDs(t, app, d, "alice", "reported"); strings.Join(got, ",") != "FEAT-1" {
		t.Errorf("alice reported = %v, want [FEAT-1]", got)
	}
	_, rec := listEntitiesAs(principalCtx("bob"), t, app, d, "feature", "features", "query_scope=reported")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "query_scope_unsupported") {
		t.Fatalf("bob reported: %d %s; want 422 query_scope_unsupported", rec.Code, rec.Body)
	}
}
