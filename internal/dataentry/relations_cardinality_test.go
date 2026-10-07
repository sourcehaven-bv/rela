package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newBoundedRelationsApp is newRelationsTestApp with belongs-to limited to one
// category per ticket (TKT-65LVAK).
func newBoundedRelationsApp(t *testing.T) *App {
	t.Helper()
	app := newRelationsTestApp(t)
	one := 1
	meta := app.State().Meta
	def := meta.Relations["belongs-to"]
	def.MaxOutgoing = &one
	meta.Relations["belongs-to"] = def
	seedEntity(app, &entity.Entity{ID: "C-002", Type: "category", Properties: map[string]any{"title": "Frontend"}})
	return app
}

func belongsTo(t *testing.T, app *App) []string {
	t.Helper()
	var out []string
	for _, r := range outgoingByType(app, "TKT-001", "belongs-to") {
		out = append(out, r.To)
	}
	return out
}

func TestPatchRelations_FullLinkageRepointsSingleValued(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))

	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"data":[{"type":"category","id":"C-002"}]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, body %s", rec.Code, rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "C-002" {
		t.Errorf("edges = %v, want [C-002]", got)
	}
}

// A delta naming the old edge in remove and the new one in add re-points
// the edge in one PATCH, as `data` does.
func TestPatchRelations_DeltaRepointsSingleValued(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))

	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"add":[{"type":"category","id":"C-002"}],"remove":[{"type":"category","id":"C-001"}]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, body %s", rec.Code, rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "C-002" {
		t.Errorf("edges = %v, want [C-002]", got)
	}
}

func TestPatchRelations_AddOverMaxOutgoingIs422(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))

	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"add":[{"type":"category","id":"C-002"}]}}}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cardinality_exceeded") {
		t.Fatalf("PATCH = %d %s, want 422 cardinality_exceeded", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "max_outgoing") {
		t.Errorf("body does not name the bound: %s", rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "C-001" {
		t.Errorf("edges = %v, want [C-001]", got)
	}
}

func TestPatchRelations_FullLinkageOverBoundIs422BeforeWriting(t *testing.T) {
	app := newBoundedRelationsApp(t)
	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"data":[{"type":"category","id":"C-001"},{"type":"category","id":"C-002"}]}}}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cardinality_exceeded") {
		t.Fatalf("PATCH = %d %s, want 422 cardinality_exceeded", rec.Code, rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 0 {
		t.Errorf("edges = %v, want none", got)
	}
}

func TestPatchRelations_RepointToMissingTargetKeepsEdge(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))
	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"data":[{"type":"category","id":"C-404"}]}}}`)
	if rec.Code < 400 {
		t.Fatalf("PATCH = %d, want a refusal", rec.Code)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "C-001" {
		t.Errorf("edges = %v, want [C-001]", got)
	}
}

// A re-point to a target the type allowlist does not admit (both entities
// exist) is a soft condition (DEC-HWZHA): it is written, with a warning.
func TestPatchRelations_RepointToDisallowedTypeWarns(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))
	rec := patch(t, app, "tickets", "TKT-001",
		`{"relations":{"belongs-to":{"data":[{"type":"label","id":"L-001"}]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s, want 200 with a warning", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "target_type_not_allowed") {
		t.Errorf("body has no type warning: %s", rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "L-001" {
		t.Errorf("edges = %v, want [L-001]", got)
	}
}

// POST of one relation answers a refused create with 422
// cardinality_exceeded, not the generic relation_failed.
func TestCreateRelation_OverMaxOutgoingIs422(t *testing.T) {
	app := newBoundedRelationsApp(t)
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/TKT-001/relations/belongs-to",
		strings.NewReader(`{"id":"C-002"}`))
	rec := httptest.NewRecorder()
	app.write.handleV1CreateRelation(rec, req, "ticket", "TKT-001", "belongs-to")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cardinality_exceeded") {
		t.Fatalf("POST = %d %s, want 422 cardinality_exceeded", rec.Code, rec.Body)
	}
	if got := belongsTo(t, app); len(got) != 1 || got[0] != "C-001" {
		t.Errorf("edges = %v, want [C-001]", got)
	}
}

// newCardinalityApp is parsed, not built literally, so the inverse names
// resolve as body keys. `owns` gives a category one owning ticket
// (max_incoming: 1); `pair` gives a ticket two categories.
func newCardinalityApp(t *testing.T, opts ...appbuildtest.Option) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title: {type: string, required: true}
  category:
    label: Category
    id_prefix: "C-"
    properties:
      title: {type: string, required: true}
  tag:
    label: Tag
    id_prefix: "G-"
    properties:
      title: {type: string, required: true}
relations:
  single:
    from: [ticket]
    to: [category]
    max_outgoing: 1
  owns:
    from: [ticket]
    to: [category]
    max_incoming: 1
    inverse: owned-by
  pair:
    from: [ticket]
    to: [category]
    max_outgoing: 2
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	cfg := &dataentryconfig.Config{
		App:   dataentryconfig.AppConfig{Name: "Cardinality Test", Description: "x"},
		Forms: map[string]dataentryconfig.Form{}, Lists: map[string]dataentryconfig.List{},
		Views: map[string]dataentryconfig.ViewConfig{}, Kanbans: map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), opts...)
	app.broker = newEventBroker()
	seedEntity(app, &entity.Entity{ID: "G-1", Type: "tag", Properties: map[string]any{"title": "G-1"}})
	for _, id := range []string{"TKT-1", "TKT-2"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}
	for _, id := range []string{"C-1", "C-2", "C-3", "C-4"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "category", Properties: map[string]any{"title": id}})
	}
	return app
}

func patchEntity(t *testing.T, app *App, typ, plural, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/"+plural+"/"+id, strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, typ, plural, id)
	return rec
}

func edgesOf(t *testing.T, app *App, q store.RelationQuery) []string {
	t.Helper()
	var out []string
	for r, err := range app.store.ListRelations(t.Context(), q) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		out = append(out, r.From+">"+r.To)
	}
	slices.Sort(out)
	return out
}

// A max_incoming: 1 edge is re-pointed from the target's side, through the
// inverse body key, with `data` and with `add` and `remove`.
func TestPatchRelations_RepointsMaxIncomingFromTarget(t *testing.T) {
	for name, body := range map[string]string{
		"data":  `{"relations":{"owned-by":{"data":[{"type":"ticket","id":"TKT-2"}]}}}`,
		"delta": `{"relations":{"owned-by":{"add":[{"type":"ticket","id":"TKT-2"}],"remove":[{"type":"ticket","id":"TKT-1"}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			app := newCardinalityApp(t)
			seedRelation(app, entity.NewRelation("TKT-1", "owns", "C-1"))
			rec := patchEntity(t, app, "category", "categories", "C-1", body)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
			}
			if got := edgesOf(t, app, store.RelationQuery{To: "C-1", Type: "owns"}); !slices.Equal(got, []string{"TKT-2>C-1"}) {
				t.Errorf("edges = %v, want [TKT-2>C-1]", got)
			}
		})
	}
}

// Adding a second owner from the target's side is refused, and the error
// path names the inverse key the request used.
func TestPatchRelations_AddOverMaxIncomingNamesInverseKey(t *testing.T) {
	app := newCardinalityApp(t)
	seedRelation(app, entity.NewRelation("TKT-1", "owns", "C-1"))
	rec := patchEntity(t, app, "category", "categories", "C-1",
		`{"relations":{"owned-by":{"add":[{"type":"ticket","id":"TKT-2"}]}}}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "cardinality_exceeded") {
		t.Fatalf("PATCH = %d %s, want 422 cardinality_exceeded", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "/relations/owned-by") {
		t.Errorf("error path does not name the inverse key: %s", rec.Body)
	}
}

// Two new targets replace two old ones on a max_outgoing: 2 relation.
func TestPatchRelations_FullLinkageReplacesTwoOfTwo(t *testing.T) {
	app := newCardinalityApp(t)
	seedRelation(app, entity.NewRelation("TKT-1", "pair", "C-1"))
	seedRelation(app, entity.NewRelation("TKT-1", "pair", "C-2"))
	rec := patchEntity(t, app, "ticket", "tickets", "TKT-1",
		`{"relations":{"pair":{"data":[{"type":"category","id":"C-3"},{"type":"category","id":"C-4"}]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	if got := edgesOf(t, app, store.RelationQuery{From: "TKT-1", Type: "pair"}); !slices.Equal(got, []string{"TKT-1>C-3", "TKT-1>C-4"}) {
		t.Errorf("edges = %v, want [TKT-1>C-3 TKT-1>C-4]", got)
	}
}

// denyRelationDeleteACL allows everything but deleting a relation.
type denyRelationDeleteACL struct{ acl.NopACL }

func (denyRelationDeleteACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if _, rel := req.Subject.(acl.RelationSubject); rel && req.Op == acl.OpDelete {
		return acl.Decision{Allow: false, RuleKind: "test", Reason: "no relation deletes"}
	}
	return acl.Decision{Allow: true}
}

// A re-point to a disallowed type whose remove is denied writes nothing: the
// soft-condition fallback runs only after every remove is authorized.
func TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing(t *testing.T) {
	app := newCardinalityApp(t, appbuildtest.WithACL(denyRelationDeleteACL{}))
	seedRelation(app, entity.NewRelation("TKT-1", "single", "C-1"))
	rec := patchEntity(t, app, "ticket", "tickets", "TKT-1",
		`{"relations":{"single":{"data":[{"type":"tag","id":"G-1"}]}}}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PATCH = %d %s, want 403", rec.Code, rec.Body)
	}
	if got := edgesOf(t, app, store.RelationQuery{From: "TKT-1", Type: "single"}); !slices.Equal(got, []string{"TKT-1>C-1"}) {
		t.Errorf("edges = %v, want [TKT-1>C-1] unchanged", got)
	}
}

// The fallback writes only the disallowed create around the manager; the
// allowed one in the same wrapper still goes through it.
func TestPatchRelations_DisallowedTypeFallbackKeepsAllowedCreateOnManager(t *testing.T) {
	aud := &audit.Memory{}
	app := newCardinalityApp(t, appbuildtest.WithAudit(aud))
	rec := patchEntity(t, app, "ticket", "tickets", "TKT-1",
		`{"relations":{"pair":{"data":[{"type":"tag","id":"G-1"},{"type":"category","id":"C-1"}]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	if got := edgesOf(t, app, store.RelationQuery{From: "TKT-1", Type: "pair"}); !slices.Equal(got, []string{"TKT-1>C-1", "TKT-1>G-1"}) {
		t.Errorf("edges = %v, want [TKT-1>C-1 TKT-1>G-1]", got)
	}
	var created []string
	for _, r := range aud.Records() {
		if r.Op == audit.OpCreateRelation && r.Subject != nil {
			created = append(created, r.Subject.ToID)
		}
	}
	if !slices.Equal(created, []string{"C-1"}) {
		t.Errorf("audited creates = %v, want [C-1]: only the allowed edge goes through the manager", created)
	}
}
