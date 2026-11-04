package dataentry

import (
	"context"
	"errors"
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
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility/visibilitytest"
)

// newTypeContractApp has one relation of each shape between ticket and
// category, each with an inverse name, and a tag type none of them allows.
// TKT-1 -> C-1 is seeded on every relation.
func newTypeContractApp(t *testing.T, aud audit.Audit, opts ...appbuildtest.Option) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title: {type: string}
  category:
    label: Category
    plural: categories
    id_prefix: "C-"
    properties:
      title: {type: string}
  tag:
    label: Tag
    id_prefix: "G-"
    properties:
      title: {type: string}
relations:
  free:
    from: [ticket]
    to: [category]
    inverse: freed-by
  single:
    from: [ticket]
    to: [category]
    max_outgoing: 1
    inverse: single-of
  owns:
    from: [ticket]
    to: [category]
    max_incoming: 1
    inverse: owned-by
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	cfg := &dataentryconfig.Config{
		App:   dataentryconfig.AppConfig{Name: "Type Contract Test", Description: "x"},
		Forms: map[string]dataentryconfig.Form{}, Lists: map[string]dataentryconfig.List{},
		Views: map[string]dataentryconfig.ViewConfig{}, Kanbans: map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), append([]appbuildtest.Option{appbuildtest.WithAudit(aud)}, opts...)...)
	app.broker = newEventBroker()
	for id, typ := range map[string]string{"TKT-1": "ticket", "C-1": "category", "G-1": "tag"} {
		seedEntity(app, &entity.Entity{ID: id, Type: typ, Properties: map[string]any{"title": "before"}})
	}
	for _, rel := range []string{"free", "single", "owns"} {
		seedRelation(app, entity.NewRelation("TKT-1", rel, "C-1"))
	}
	return app
}

// allEdges lists every stored edge as from>type>to, sorted.
func allEdges(t *testing.T, app *App) []string {
	t.Helper()
	var out []string
	for r, err := range app.store.ListRelations(t.Context(), store.RelationQuery{}) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		out = append(out, r.From+">"+r.Type+">"+r.To)
	}
	slices.Sort(out)
	return out
}

// writeAudited lists the audited write operations, ignoring nothing: a
// refused request must leave none.
func writeAudited(aud *audit.Memory) []string {
	var out []string
	for _, r := range aud.Records() {
		out = append(out, r.Op)
	}
	return out
}

// TestPatchRelations_RejectsDisallowedEndpointTypes pins DEC-0D8UZX3 on PATCH
// (GitHub #1992): an edge whose source or target type the relation does not
// allow is refused with 422 and a code naming the wrong end of the canonical
// edge, for canonical and inverse names and for bounded and unbounded
// relations. The refusal comes before any write: the entity's properties,
// every edge and the audit log are unchanged.
func TestPatchRelations_RejectsDisallowedEndpointTypes(t *testing.T) {
	const (
		src = metamodel.CodeSourceTypeNotAllowed
		tgt = metamodel.CodeTargetTypeNotAllowed
	)
	tests := []struct {
		name             string
		typ, plural, id  string // the path entity
		bodyKey, wrapper string
		wantCode         string
	}{
		// The peer has the wrong type. Under a canonical name the peer is
		// the target; under an inverse name it is the source.
		{"unbounded canonical, wrong peer", "ticket", "tickets", "TKT-1",
			"free", `{"data":[{"type":"tag","id":"G-1"}]}`, tgt},
		{"max_outgoing canonical, wrong peer", "ticket", "tickets", "TKT-1",
			"single", `{"data":[{"type":"tag","id":"G-1"}]}`, tgt},
		{"max_incoming canonical, wrong peer", "ticket", "tickets", "TKT-1",
			"owns", `{"data":[{"type":"tag","id":"G-1"}]}`, tgt},
		{"unbounded canonical delta, wrong peer", "ticket", "tickets", "TKT-1",
			"free", `{"add":[{"type":"tag","id":"G-1"}]}`, tgt},
		{"mixed full linkage, one wrong peer", "ticket", "tickets", "TKT-1",
			"free", `{"data":[{"type":"category","id":"C-1"},{"type":"tag","id":"G-1"}]}`, tgt},
		{"unbounded inverse, wrong peer", "category", "categories", "C-1",
			"freed-by", `{"data":[{"type":"tag","id":"G-1"}]}`, src},
		{"max_outgoing inverse, wrong peer", "category", "categories", "C-1",
			"single-of", `{"data":[{"type":"tag","id":"G-1"}]}`, src},
		{"max_incoming inverse, wrong peer", "category", "categories", "C-1",
			"owned-by", `{"data":[{"type":"tag","id":"G-1"}]}`, src},
		// The path entity has the wrong type. Under a canonical name it is
		// the source; under an inverse name it is the target.
		{"unbounded canonical, wrong path entity", "tag", "tags", "G-1",
			"free", `{"data":[{"type":"category","id":"C-1"}]}`, src},
		{"max_outgoing canonical, wrong path entity", "tag", "tags", "G-1",
			"single", `{"data":[{"type":"category","id":"C-1"}]}`, src},
		{"max_incoming canonical, wrong path entity", "tag", "tags", "G-1",
			"owns", `{"data":[{"type":"category","id":"C-1"}]}`, src},
		{"unbounded inverse, wrong path entity", "tag", "tags", "G-1",
			"freed-by", `{"data":[{"type":"ticket","id":"TKT-1"}]}`, tgt},
		{"max_outgoing inverse, wrong path entity", "tag", "tags", "G-1",
			"single-of", `{"data":[{"type":"ticket","id":"TKT-1"}]}`, tgt},
		{"max_incoming inverse, wrong path entity", "tag", "tags", "G-1",
			"owned-by", `{"add":[{"type":"ticket","id":"TKT-1"}]}`, tgt},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aud := audit.NewMemory()
			app := newTypeContractApp(t, aud)
			before := allEdges(t, app)

			body := `{"properties":{"title":"after"},"relations":{"` + tc.bodyKey + `":` + tc.wrapper + `}}`
			rec := patchEntity(t, app, tc.typ, tc.plural, tc.id, body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("PATCH = %d %s, want 422", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "/errors/"+tc.wantCode) {
				t.Errorf("body = %s, want code %s", rec.Body, tc.wantCode)
			}
			if got := allEdges(t, app); !slices.Equal(got, before) {
				t.Errorf("edges = %v, want %v unchanged", got, before)
			}
			e, err := app.store.GetEntity(t.Context(), entity.Ref{ID: tc.id})
			if err != nil {
				t.Fatalf("GetEntity %s: %v", tc.id, err)
			}
			if e.Properties["title"] != "before" {
				t.Errorf("title = %v, want the entity untouched", e.Properties["title"])
			}
			if got := writeAudited(aud); len(got) != 0 {
				t.Errorf("audited %v, want nothing", got)
			}
		})
	}
}

// A body that names a relation the path entity's type cannot be on, but
// writes no edge (data: [] or only remove), creates nothing to refuse.
func TestPatchRelations_EmptyWrapperOnDisallowedTypeIsNotRefused(t *testing.T) {
	for name, wrapper := range map[string]string{
		"empty data":  `{"data":[]}`,
		"only remove": `{"remove":[{"type":"category","id":"C-1"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			app := newTypeContractApp(t, audit.Nop{})
			rec := patchEntity(t, app, "tag", "tags", "G-1", `{"relations":{"free":`+wrapper+`}}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH = %d %s, want 200", rec.Code, rec.Body)
			}
		})
	}
}

// TestCreateEntity_RejectsDisallowedEndpointTypes pins DEC-0D8UZX3 on POST:
// the refusal comes before the entity is created, so no unlinked entity is
// left behind and nothing is audited.
func TestCreateEntity_RejectsDisallowedEndpointTypes(t *testing.T) {
	tests := []struct {
		name, typ, plural, rels, wantCode string
	}{
		{"canonical, wrong peer", "ticket", "tickets",
			`{"free":{"data":[{"type":"tag","id":"G-1"}]}}`, metamodel.CodeTargetTypeNotAllowed},
		{"inverse, wrong peer", "category", "categories",
			`{"owned-by":{"data":[{"type":"tag","id":"G-1"}]}}`, metamodel.CodeSourceTypeNotAllowed},
		{"canonical, wrong new entity", "tag", "tags",
			`{"single":{"data":[{"type":"category","id":"C-1"}]}}`, metamodel.CodeSourceTypeNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aud := audit.NewMemory()
			app := newTypeContractApp(t, aud)
			before := len(entitiesByType(app, tc.typ))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/"+tc.plural,
				strings.NewReader(`{"properties":{"title":"new"},"relations":`+tc.rels+`}`))
			rec := httptest.NewRecorder()
			app.write.handleV1CreateEntity(rec, req, tc.typ, tc.plural)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("POST = %d %s, want 422", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "/errors/"+tc.wantCode) {
				t.Errorf("body = %s, want code %s", rec.Body, tc.wantCode)
			}
			if got := len(entitiesByType(app, tc.typ)); got != before {
				t.Errorf("%d %s entities, want %d: the refused create wrote one", got, tc.typ, before)
			}
			if got := writeAudited(aud); len(got) != 0 {
				t.Errorf("audited %v, want nothing", got)
			}
		})
	}
}

// TestCreateRelation_RejectsDisallowedEndpointTypes pins DEC-0D8UZX3 on the
// per-edge POST: the manager's refusal answers 422 with the same codes.
func TestCreateRelation_RejectsDisallowedEndpointTypes(t *testing.T) {
	tests := []struct {
		name, typ, id, relType, body, wantCode string
	}{
		{"outgoing, wrong target", "ticket", "TKT-1", "free",
			`{"id":"G-1"}`, metamodel.CodeTargetTypeNotAllowed},
		{"outgoing bounded, wrong target", "ticket", "TKT-1", "owns",
			`{"id":"G-1"}`, metamodel.CodeTargetTypeNotAllowed},
		{"incoming, wrong source", "category", "C-1", "free",
			`{"id":"G-1","direction":"incoming"}`, metamodel.CodeSourceTypeNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aud := audit.NewMemory()
			app := newTypeContractApp(t, aud)
			before := allEdges(t, app)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/x/"+tc.id+"/relations/"+tc.relType,
				strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			app.write.handleV1CreateRelation(rec, req, tc.typ, tc.id, tc.relType)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("POST = %d %s, want 422", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "/errors/"+tc.wantCode) {
				t.Errorf("body = %s, want code %s", rec.Body, tc.wantCode)
			}
			if got := allEdges(t, app); !slices.Equal(got, before) {
				t.Errorf("edges = %v, want %v unchanged", got, before)
			}
			if got := writeAudited(aud); len(got) != 0 {
				t.Errorf("audited %v, want nothing", got)
			}
		})
	}
}

// typeHidingGate is a read gate that hides every entity of one type and
// admits the rest.
type typeHidingGate struct {
	fakeGate
	hidden string
}

func (g typeHidingGate) PermitsRead(_ context.Context, typ, _ string) (bool, error) {
	return typ != g.hidden, nil
}

func (g typeHidingGate) ReadableFacesMany(_ context.Context, typ string, ids []string) (acl.FaceVerdicts, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = typ != g.hidden
	}
	return visibilitytest.IDVerdicts(m, nil)
}

func (g typeHidingGate) ReadQuery(_ context.Context, typ string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: typ != g.hidden, DenyAll: typ == g.hidden}
}

// TestRelationWrite_HiddenDisallowedPeerAnswersAsMissing pins that a peer the
// caller may not read is never type-checked: G-1, a tag no relation allows,
// is hidden, and every write surface answers it exactly as it answers an id
// that does not exist. The answer also comes before any write, so the PATCH
// leaves the title alone and the POST creates no entity.
func TestRelationWrite_HiddenDisallowedPeerAnswersAsMissing(t *testing.T) {
	type outcome struct {
		code     int
		body     string
		edges    []string
		titles   []string
		audited  []string
		tickets  int
		category int
	}
	surfaces := []struct {
		name string
		send func(app *App, gated func(*http.Request) *http.Request, peer string) *httptest.ResponseRecorder
	}{
		{"PATCH canonical", func(app *App, gated func(*http.Request) *http.Request, peer string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-1", strings.NewReader(
				`{"properties":{"title":"after"},"relations":{"free":{"add":[{"type":"category","id":"`+peer+`"}]}}}`))
			rec := httptest.NewRecorder()
			app.write.handleV1UpdateEntity(rec, gated(req), "ticket", "tickets", "TKT-1")
			return rec
		}},
		{"PATCH inverse", func(app *App, gated func(*http.Request) *http.Request, peer string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/categories/C-1", strings.NewReader(
				`{"properties":{"title":"after"},"relations":{"owned-by":{"data":[{"type":"ticket","id":"`+peer+`"}]}}}`))
			rec := httptest.NewRecorder()
			app.write.handleV1UpdateEntity(rec, gated(req), "category", "categories", "C-1")
			return rec
		}},
		{"POST create", func(app *App, gated func(*http.Request) *http.Request, peer string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(
				`{"properties":{"title":"new"},"relations":{"free":{"data":[{"type":"category","id":"`+peer+`"}]}}}`))
			rec := httptest.NewRecorder()
			app.write.handleV1CreateEntity(rec, gated(req), "ticket", "tickets")
			return rec
		}},
		{"per-edge POST incoming", func(app *App, gated func(*http.Request) *http.Request, peer string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/categories/C-1/relations/free", strings.NewReader(
				`{"id":"`+peer+`","direction":"incoming"}`))
			rec := httptest.NewRecorder()
			app.write.handleV1CreateRelation(rec, gated(req), "category", "C-1", "free")
			return rec
		}},
	}
	run := func(
		t *testing.T, send func(*App, func(*http.Request) *http.Request, string) *httptest.ResponseRecorder, peer string,
	) outcome {
		t.Helper()
		aud := audit.NewMemory()
		app := newTypeContractApp(t, aud)
		gated := func(r *http.Request) *http.Request {
			return r.WithContext(withReadGate(r.Context(), typeHidingGate{hidden: "tag"}))
		}
		rec := send(app, gated, peer)
		var titles []string
		for _, id := range []string{"TKT-1", "C-1"} {
			e, err := app.store.GetEntity(t.Context(), entity.Ref{ID: id})
			if err != nil {
				t.Fatalf("GetEntity %s: %v", id, err)
			}
			titles = append(titles, e.Properties["title"].(string))
		}
		return outcome{
			code: rec.Code, body: strings.ReplaceAll(rec.Body.String(), peer, "PEER"),
			edges: allEdges(t, app), titles: titles, audited: writeAudited(aud),
			tickets: len(entitiesByType(app, "ticket")), category: len(entitiesByType(app, "category")),
		}
	}
	for _, sf := range surfaces {
		t.Run(sf.name, func(t *testing.T) {
			hidden := run(t, sf.send, "G-1")
			missing := run(t, sf.send, "G-404")
			if hidden.code < 400 {
				t.Fatalf("hidden peer: status %d %s, want a refusal", hidden.code, hidden.body)
			}
			if strings.Contains(hidden.body, "type_not_allowed") || strings.Contains(hidden.body, "tag") {
				t.Errorf("hidden peer's type leaked: %s", hidden.body)
			}
			if hidden.code != missing.code || hidden.body != missing.body {
				t.Errorf("hidden peer answered %d %s, missing peer %d %s; want the same",
					hidden.code, hidden.body, missing.code, missing.body)
			}
			if !slices.Equal(hidden.titles, []string{"before", "before"}) {
				t.Errorf("titles = %v, want both untouched", hidden.titles)
			}
			if hidden.tickets != 1 || hidden.category != 1 {
				t.Errorf("%d tickets, %d categories, want the seeded 1 and 1", hidden.tickets, hidden.category)
			}
			if len(hidden.audited) != 0 {
				t.Errorf("audited %v, want nothing", hidden.audited)
			}
			if !slices.Equal(hidden.edges, missing.edges) {
				t.Errorf("edges %v, want %v", hidden.edges, missing.edges)
			}
		})
	}
}

// An existing edge whose type the relation does not allow is not kept by a
// full linkage that names it: the PATCH is refused and the edge stays as it
// was (DEC-0D8UZX3 has no carve-out for existing edges).
func TestPatchRelations_KeepingExistingDisallowedEdgeIs422(t *testing.T) {
	app := newTypeContractApp(t, audit.Nop{})
	seedRelation(app, entity.NewRelation("TKT-1", "free", "G-1"))
	before := allEdges(t, app)
	rec := patchEntity(t, app, "ticket", "tickets", "TKT-1",
		`{"relations":{"free":{"data":[{"type":"category","id":"C-1"},{"type":"tag","id":"G-1"}]}}}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), metamodel.CodeTargetTypeNotAllowed) {
		t.Fatalf("PATCH = %d %s, want 422 %s", rec.Code, rec.Body, metamodel.CodeTargetTypeNotAllowed)
	}
	if got := allEdges(t, app); !slices.Equal(got, before) {
		t.Errorf("edges = %v, want %v unchanged", got, before)
	}
}

// TestEndpointTypeRefusal pins the mapping of a manager refusal to the 422:
// bare or wrapped as a RelationCreateError it carries the code; any other
// error maps to nil.
func TestEndpointTypeRefusal(t *testing.T) {
	app := newTypeContractApp(t, audit.Nop{})
	key := entity.RelationKey{From: "TKT-1", Type: "free", To: "G-1"}
	_, refused := app.entityManager.CreateRelation(t.Context(), key, entity.RelationOptions{})
	if refused == nil {
		t.Fatal("manager admitted a disallowed target")
	}
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"bare", refused, metamodel.CodeTargetTypeNotAllowed},
		{"wrapped", &entitymanager.RelationCreateError{Key: key, Err: refused}, metamodel.CodeTargetTypeNotAllowed},
		{"other error", errors.New("boom"), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := endpointTypeRefusal(tc.err, "freed-by")
			if tc.wantCode == "" {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Code != tc.wantCode || got.Path != "/relations/freed-by" {
				t.Fatalf("got %+v, want code %s at /relations/freed-by", got, tc.wantCode)
			}
		})
	}
}

// A caller the create ACL denies gets the manager's 403 even when the body
// also names a disallowed edge: the endpoint-type 422 must not pre-empt the
// authorization answer.
func TestCreateEntity_DeniedCreateIs403BeforeEndpointType(t *testing.T) {
	app := newTypeContractApp(t, audit.Nop{}, appbuildtest.WithACL(acl.ReadOnlyACL{}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(
		`{"properties":{"title":"new"},"relations":{"free":{"data":[{"type":"tag","id":"G-1"}]}}}`))
	rec := httptest.NewRecorder()
	app.write.handleV1CreateEntity(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST = %d %s, want 403", rec.Code, rec.Body)
	}
}
