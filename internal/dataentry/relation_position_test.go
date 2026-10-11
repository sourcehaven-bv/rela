package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// patchPosition sends a relation PATCH for REC-001 --has-step--> target.
func patchPosition(t *testing.T, app *App, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/recipes/REC-001/relations/has-step/"+target,
		bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateRelation(rec, req, "recipe", "REC-001", "has-step", target)
	return rec
}

// recipeStepOrder reads REC-001's steps the way the entity page does.
func recipeStepOrder(t *testing.T, app *App) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/REC-001/relations/has-step", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetRelationType(rec, req, "recipe", "REC-001", "has-step")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET relations: %d %s", rec.Code, rec.Body)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return relIDs(rows)
}

func TestRelationPosition_Moves(t *testing.T) {
	// Fixture order: STP-Z=1, STP-X=2, STP-M=3, STP-Y=missing.
	app := newOrderableRelationsTestApp(t, metamodel.OrderableOutgoing)
	seedOrderableFixture(t, app)

	steps := []struct {
		target, body string
		want         []string
	}{
		{"STP-Y", `{"position":{"before":"STP-X"}}`, []string{"STP-Z", "STP-Y", "STP-X", "STP-M"}},
		{"STP-Z", `{"position":{"after":"STP-M"}}`, []string{"STP-Y", "STP-X", "STP-M", "STP-Z"}},
		{"STP-M", `{"position":{"step":-1}}`, []string{"STP-Y", "STP-M", "STP-X", "STP-Z"}},
	}
	for _, s := range steps {
		rec := patchPosition(t, app, s.target, s.body)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s %s: got %d, want 204: %s", s.target, s.body, rec.Code, rec.Body)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("a position PATCH must not echo meta: %s", rec.Body)
		}
		if got := recipeStepOrder(t, app); !slices.Equal(got, s.want) {
			t.Fatalf("after %s %s: order = %v, want %v", s.target, s.body, got, s.want)
		}
	}
}

func TestRelationPosition_BadRequests(t *testing.T) {
	tests := []struct {
		name string
		mode metamodel.OrderableMode
		body string
		code string
	}{
		{"with meta", metamodel.OrderableOutgoing, `{"position":{"step":1},"meta":{"x":1}}`, "order_position_invalid"},
		{"before and after", metamodel.OrderableOutgoing, `{"position":{"before":"STP-X","after":"STP-M"}}`, "order_position_invalid"},
		{"empty", metamodel.OrderableOutgoing, `{"position":{}}`, "order_position_invalid"},
		{"step of two", metamodel.OrderableOutgoing, `{"position":{"step":2}}`, "order_position_invalid"},
		{"relative to itself", metamodel.OrderableOutgoing, `{"position":{"after":"STP-Z"}}`, "order_position_invalid"},
		{"not orderable", metamodel.OrderableNone, `{"position":{"step":1}}`, "relation_not_orderable"},
		{"incoming only", metamodel.OrderableIncoming, `{"position":{"step":1}}`, "relation_not_orderable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrderableRelationsTestApp(t, tt.mode)
			seedOrderableFixture(t, app)
			rec := patchPosition(t, app, "STP-Z", tt.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.code) {
				t.Errorf("got %d %s, want 400 %s", rec.Code, rec.Body, tt.code)
			}
		})
	}
}

// A sibling that does not exist, and an entity that exists but is not a
// sibling, answer one and the same 404.
func TestRelationPosition_NotASiblingIsTheUniform404(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableOutgoing)
	seedOrderableFixture(t, app)
	seedEntity(app, &entity.Entity{ID: "STP-LONE", Type: "step", Properties: map[string]any{"title": "lone"}})

	missing := patchPosition(t, app, "STP-Z", `{"position":{"after":"STP-NOPE"}}`)
	lone := patchPosition(t, app, "STP-Z", `{"position":{"after":"STP-LONE"}}`)
	if missing.Code != http.StatusNotFound || lone.Code != http.StatusNotFound {
		t.Fatalf("codes = %d, %d; want 404, 404", missing.Code, lone.Code)
	}
	if missing.Body.String() != lone.Body.String() {
		t.Errorf("bodies differ:\nmissing: %s\nlone:    %s", missing.Body, lone.Body)
	}
}

func TestRelationPosition_OrderFieldNotWritable(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableOutgoing)
	seedOrderableFixture(t, app)
	app.fieldResolver = fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"has-step": {Creatable: true, Removable: true, Fields: map[string]bool{metamodel.OrderPropertyOut: false}},
	}}}
	rec := patchPosition(t, app, "STP-Z", `{"position":{"step":1}}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "meta-read-only:has-step._order_out") {
		t.Errorf("body must name the meta-read-only rule: %s", rec.Body)
	}
}

// A sibling the principal cannot read answers the same 404 as one that does
// not exist, so a position is no probe for hidden entities.
func TestRelationPosition_HiddenSiblingIsTheUniform404(t *testing.T) {
	app := newTestAppV1(t)
	d := seedNeighborLeakFixture(t, app)
	rd := app.State().Meta.Relations["implements"]
	rd.Orderable = metamodel.OrderableOutgoing
	app.State().Meta.Relations["implements"] = rd

	send := func(ref string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-VIS/relations/implements/FEAT-VIS",
			strings.NewReader(`{"position":{"after":"`+ref+`"}}`))
		req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
		rec := httptest.NewRecorder()
		app.write.handleV1UpdateRelation(rec, req, "ticket", "TKT-VIS", "implements", "FEAT-VIS")
		return rec
	}
	hidden := send("FEAT-HIDDEN")
	missing := send("FEAT-NOPE")
	if hidden.Code != http.StatusNotFound || missing.Code != http.StatusNotFound {
		t.Fatalf("codes = %d, %d; want 404, 404 (hidden: %s)", hidden.Code, missing.Code, hidden.Body)
	}
	if hidden.Body.String() != missing.Body.String() {
		t.Errorf("bodies differ:\nhidden:  %s\nmissing: %s", hidden.Body, missing.Body)
	}
}

// A step counts only the siblings the principal can see, and writes no
// other edge. TKT-VIS has a hidden FEAT-HIDDEN edge before FEAT-VIS; were
// it counted, a step up would move FEAT-VIS past it and renumber it, and the
// answer would tell the principal that a hidden sibling exists.
func TestRelationPosition_StepIgnoresHiddenSiblings(t *testing.T) {
	app := newTestAppV1(t)
	d := seedNeighborLeakFixture(t, app)
	rd := app.State().Meta.Relations["implements"]
	rd.Orderable = metamodel.OrderableOutgoing
	app.State().Meta.Relations["implements"] = rd

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-VIS/relations/implements/FEAT-VIS",
		strings.NewReader(`{"position":{"step":-1}}`))
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateRelation(rec, req, "ticket", "TKT-VIS", "implements", "FEAT-VIS")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}
	for _, to := range []string{"FEAT-HIDDEN", "FEAT-VIS"} {
		rel, err := app.store.GetRelation(t.Context(),
			entity.RelationKey{From: "TKT-VIS", Type: "implements", To: to})
		if err != nil {
			t.Fatalf("get %s: %v", to, err)
		}
		if v, ok := rel.Properties[metamodel.OrderPropertyOut]; ok {
			t.Errorf("%s edge got order value %v; the step must not write it", to, v)
		}
	}
}

// A content-scoped relation lists one face's edges, so the order names its
// anchor by address, and a move on that address rewrites that face's edges
// only. POL-1's draft edges are another face's list: the densify the move
// causes must leave them alone.
func TestRelationPosition_FacedAnchor(t *testing.T) {
	app, d := publishedEditor(t)
	ctx := context.Background()
	meta := app.State().Meta
	rd := meta.Relations["cites"]
	rd.Orderable = metamodel.OrderableOutgoing
	meta.Relations["cites"] = rd
	for _, id := range []string{"FEAT-2", "FEAT-3"} {
		if err := app.store.CreateEntity(ctx, &entity.Entity{ID: id, Type: "feature",
			Properties: map[string]any{"title": id}}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	edge := func(face entity.Face, to string) entity.RelationKey {
		return entity.RelationKey{From: "POL-1", FromFace: face, Type: "cites", To: to}
	}
	for _, k := range []entity.RelationKey{
		edge("published", "FEAT-1"), edge("published", "FEAT-2"), edge("published", "FEAT-3"),
		edge("draft", "FEAT-1"), edge("draft", "FEAT-2"),
	} {
		if _, err := app.store.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatalf("seed %v: %v", k, err)
		}
	}

	anchor, err := app.store.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: "published"})
	if err != nil {
		t.Fatalf("get anchor: %v", err)
	}
	actx := asAlice(t, d, httptest.NewRequest(http.MethodGet, "/", http.NoBody)).Context()
	ordering, err := newRelationOrdering(actx, app.write.affordances, meta, anchor, "cites", false, nil)
	if err != nil {
		t.Fatalf("ordering: %v", err)
	}
	order := ordering.wire()
	if order == nil || order.Anchor != "POL-1@published" || !order.Movable {
		t.Fatalf("order = %+v, want a movable order anchored at POL-1@published", order)
	}

	rec := relationAs(t, app, d, http.MethodPatch, "policy", "policys", order.Anchor, "cites", "FEAT-3",
		`{"position":{"after":"FEAT-1"}}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("move = %d %s, want 204", rec.Code, rec.Body)
	}
	value := func(k entity.RelationKey) (float64, bool) {
		r, err := app.store.GetRelation(ctx, k)
		if err != nil {
			t.Fatalf("get %v: %v", k, err)
		}
		return metamodel.FiniteOrder(r.Properties[metamodel.OrderPropertyOut])
	}
	// FEAT-1 has no value, so no value fits after it and the face densifies.
	for i, to := range []string{"FEAT-1", "FEAT-3", "FEAT-2"} {
		if v, _ := value(edge("published", to)); v != float64(i+1) {
			t.Errorf("published %s = %v, want %d", to, v, i+1)
		}
	}
	for _, to := range []string{"FEAT-1", "FEAT-2"} {
		if v, ok := value(edge("draft", to)); ok {
			t.Errorf("draft %s got order value %v; a move on the published face must not write it", to, v)
		}
	}
}
