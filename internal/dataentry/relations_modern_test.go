package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newRelationsTestApp builds an App with relation types that exercise
// the modern wire format: tagged (closed-schema meta + content body),
// belongs-to (no meta, no content), assesses (required meta).
func newRelationsTestApp(t *testing.T) *App {
	t.Helper()

	weight := metamodel.PropertyDef{Type: "integer"}
	addedBy := metamodel.PropertyDef{Type: "string"}
	rationale := metamodel.PropertyDef{Type: "string", Required: true}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
			},
			"label": {
				Label:    "Label",
				IDPrefix: "L-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"category": {
				Label:    "Category",
				IDPrefix: "C-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"reviewer": {
				Label:    "Reviewer",
				IDPrefix: "R-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"tagged": {
				Label:   "tagged",
				From:    []string{"ticket"},
				To:      []string{"label"},
				Content: true,
				Properties: map[string]metamodel.PropertyDef{
					"weight":   weight,
					"added_by": addedBy,
				},
			},
			"belongs-to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"category"},
			},
			"assessed-by": {
				Label: "assessed by",
				From:  []string{"ticket"},
				To:    []string{"reviewer"},
				Properties: map[string]metamodel.PropertyDef{
					"rationale": rationale,
				},
			},
		},
	}

	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Relations Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}

	app := newAppFromParts(cfg, meta, newFixture())
	app.broker = newEventBroker()

	// Seed a baseline ticket and a few targets.
	seedEntity(app, &entity.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Sample ticket",
			"status": "open",
		},
	})
	seedEntity(app, &entity.Entity{ID: "L-001", Type: "label", Properties: map[string]interface{}{"title": "bug"}})
	seedEntity(app, &entity.Entity{ID: "L-002", Type: "label", Properties: map[string]interface{}{"title": "ui"}})
	seedEntity(app, &entity.Entity{ID: "L-003", Type: "label", Properties: map[string]interface{}{"title": "perf"}})
	seedEntity(app, &entity.Entity{ID: "C-001", Type: "category", Properties: map[string]interface{}{"title": "Backend"}})
	seedEntity(app, &entity.Entity{ID: "R-001", Type: "reviewer", Properties: map[string]interface{}{"title": "Alice"}})
	return app
}

// patch issues a PATCH against the app's handler and returns the recorder.
//
//nolint:unparam // plural is "tickets" everywhere today; kept generic so future tests can target other types without churn
func patch(t *testing.T, app *App, plural, id, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/"+plural+"/"+id, bytes.NewReader([]byte(body)))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "ticket", plural, id)
	return rec
}

func decodeV1(t *testing.T, body []byte) v1.Entity {
	t.Helper()
	var got v1.Entity
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, body)
	}
	return got
}

// outgoingByType returns the current outgoing edges of the given type for entityID.
//
//nolint:unparam // entityID is "TKT-001" everywhere today; kept generic for future tests
func outgoingByType(app *App, entityID, relType string) []*entity.Relation {
	out := []*entity.Relation{}
	for r, err := range app.store.ListRelations(context.Background(), store.RelationQuery{
		EntityID:  entityID,
		Direction: store.DirectionOutgoing,
		Type:      relType,
	}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// AC1: add new edge with meta.
func TestModern_AC1_AddEdgeWithMeta(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"weight":5}}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 {
		t.Fatalf("len=%d, want 1; %v", len(rels), rels)
	}
	if got := rels[0].Properties["weight"]; got != float64(5) {
		t.Errorf("weight=%v (%T), want 5", got, got)
	}
}

// AC2: upsert meta on existing edge (no duplicate created).
func TestModern_AC2_UpsertMeta(t *testing.T) {
	app := newRelationsTestApp(t)
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", "L-001",
		&store.RelationData{Properties: map[string]interface{}{"weight": float64(5)}}); err != nil {
		t.Fatal(err)
	}
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"weight":7}}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 {
		t.Fatalf("len=%d, want 1 (no duplicates)", len(rels))
	}
	if got := rels[0].Properties["weight"]; got != float64(7) {
		t.Errorf("weight=%v, want 7", got)
	}
}

// AC3: meta_unset clears keys.
func TestModern_AC3_MetaUnset(t *testing.T) {
	app := newRelationsTestApp(t)
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", "L-001",
		&store.RelationData{Properties: map[string]interface{}{
			"weight":   float64(5),
			"added_by": "alice",
		}}); err != nil {
		t.Fatal(err)
	}
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta_unset":["weight"]}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 {
		t.Fatalf("len=%d, want 1", len(rels))
	}
	if _, has := rels[0].Properties["weight"]; has {
		t.Errorf("weight should be cleared, props=%v", rels[0].Properties)
	}
	if got := rels[0].Properties["added_by"]; got != "alice" {
		t.Errorf("added_by=%v, want alice (preserved)", got)
	}
}

// AC4: per-edge content upsert + clear.
func TestModern_AC4_ContentUpsertAndClear(t *testing.T) {
	app := newRelationsTestApp(t)
	body1 := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","content":"edge body"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body1)
	if rec.Code != http.StatusOK {
		t.Fatalf("first PATCH status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 || rels[0].Content != "edge body" {
		t.Fatalf("after first PATCH: %v", rels)
	}

	body2 := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","content":""}]}}}`
	rec = patch(t, app, "tickets", "TKT-001", body2)
	if rec.Code != http.StatusOK {
		t.Fatalf("second PATCH status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels = outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 || rels[0].Content != "" {
		t.Fatalf("content should be cleared, got %q", rels[0].Content)
	}
}

// AC5: data: [] removes all edges of that type.
func TestModern_AC5_EmptyDataRemovesAll(t *testing.T) {
	app := newRelationsTestApp(t)
	for _, id := range []string{"L-001", "L-002", "L-003"} {
		if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", id, nil); err != nil {
			t.Fatal(err)
		}
	}
	body := `{"relations": {"tagged": {"data": []}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rels := outgoingByType(app, "TKT-001", "tagged"); len(rels) != 0 {
		t.Errorf("len=%d, want 0; %v", len(rels), rels)
	}
}

// AC6: replacement semantics — keep, add, remove.
func TestModern_AC6_Replacement(t *testing.T) {
	app := newRelationsTestApp(t)
	for _, id := range []string{"L-001", "L-002", "L-003"} {
		if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", id, nil); err != nil {
			t.Fatal(err)
		}
	}
	seedEntity(app, &entity.Entity{ID: "L-004", Type: "label", Properties: map[string]interface{}{"title": "new"}})

	body := `{"relations": {"tagged": {"data": [
		{"type":"label","id":"L-001"},
		{"type":"label","id":"L-004"}
	]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	got := map[string]bool{}
	for _, r := range rels {
		got[r.To] = true
	}
	want := map[string]bool{"L-001": true, "L-004": true}
	if len(got) != len(want) {
		t.Fatalf("got=%v, want=%v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing %q", k)
		}
	}
}

// AC7: absent relation type leaves alone.
func TestModern_AC7_AbsentTypeLeavesAlone(t *testing.T) {
	app := newRelationsTestApp(t)
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", "L-001", nil); err != nil {
		t.Fatal(err)
	}
	seedEntity(app, &entity.Entity{ID: "C-002", Type: "category", Properties: map[string]interface{}{"title": "Frontend"}})
	body := `{"relations": {"belongs-to": {"data": [{"type":"category","id":"C-002"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rels := outgoingByType(app, "TKT-001", "tagged"); len(rels) != 1 {
		t.Errorf("tagged should be preserved, got %v", rels)
	}
}

// AC8: {"tagged": {}} returns 400.
func TestModern_AC8_DataAbsentReturns400(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// AC9: unknown relation type returns 422.
func TestModern_AC9_UnknownRelationTypeReturns422(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"nonexistent": {"data": []}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status=%d, want 422; body=%s", rec.Code, rec.Body.String())
	}
}

// AC10: target type mismatch surfaces warning, edge IS written.
func TestModern_AC10_TargetTypeMismatchWarns(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"data": [{"type":"category","id":"L-001"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (with warning); body=%s", rec.Code, rec.Body.String())
	}
	got := decodeV1(t, rec.Body.Bytes())
	found := false
	for _, w := range got.Warnings {
		if w.Code == "target_type_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected target_type_mismatch warning, got %v", got.Warnings)
	}
	if rels := outgoingByType(app, "TKT-001", "tagged"); len(rels) != 1 {
		t.Errorf("edge should still be created despite warning, got %d", len(rels))
	}
}

// AC11: target ID doesn't exist surfaces warning.
func TestModern_AC11_TargetMissingWarns(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-DOES-NOT-EXIST"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := decodeV1(t, rec.Body.Bytes())
	found := false
	for _, w := range got.Warnings {
		if w.Code == "target_not_found" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected target_not_found warning, got %v", got.Warnings)
	}
}

// AC12: unknown meta key surfaces warning.
func TestModern_AC12_UnknownMetaKeyWarns(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"unknownX":1}}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := decodeV1(t, rec.Body.Bytes())
	found := false
	for _, w := range got.Warnings {
		if w.Code == "unknown_meta_key" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown_meta_key warning, got %v", got.Warnings)
	}
	rels := outgoingByType(app, "TKT-001", "tagged")
	if len(rels) != 1 {
		t.Fatalf("len=%d, want 1", len(rels))
	}
	if rels[0].Properties["unknownX"] != float64(1) {
		t.Errorf("meta should still be persisted, got %v", rels[0].Properties)
	}
}

// AC12b: required meta unset surfaces warning.
func TestModern_AC12b_RequiredMetaUnsetWarns(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"assessed-by": {"data": [{"type":"reviewer","id":"R-001"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := decodeV1(t, rec.Body.Bytes())
	found := false
	for _, w := range got.Warnings {
		if w.Code == "required_meta_unset" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected required_meta_unset warning, got %v", got.Warnings)
	}
}

// AC12c: meta type mismatch surfaces warning.
func TestModern_AC12c_MetaTypeMismatchWarns(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"weight":"not-a-number"}}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := decodeV1(t, rec.Body.Bytes())
	found := false
	for _, w := range got.Warnings {
		if w.Code == "meta_type_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected meta_type_mismatch warning, got %v", got.Warnings)
	}
}

// AC13: content on non-content-bearing relation type returns 422.
func TestModern_AC13_ContentOnNonContentTypeReturns422(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"belongs-to": {"data": [{"type":"category","id":"C-001","content":"body"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status=%d, want 422; body=%s", rec.Code, rec.Body.String())
	}
}

// AC14: value-based no-op suppression — re-PATCHing identical state writes nothing.
func TestModern_AC14_ValueBasedNoOp(t *testing.T) {
	app := newRelationsTestApp(t)
	if _, err := app.store.CreateRelation(context.Background(), "TKT-001", "tagged", "L-001",
		&store.RelationData{Properties: map[string]interface{}{"weight": float64(5)}}); err != nil {
		t.Fatal(err)
	}

	events, cancel := app.store.Subscribe(16)
	body := `{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"weight":5}}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	relCount := 0
	done := make(chan struct{})
	go func() {
		for range events {
			relCount++
		}
		close(done)
	}()
	cancel()
	<-done

	if relCount != 0 {
		t.Errorf("expected 0 store events on no-op PATCH, got %d", relCount)
	}
}

// AC17: validation failure leaves entity AND relations untouched.
func TestModern_AC17_ValidationFailureLeavesStateUntouched(t *testing.T) {
	app := newRelationsTestApp(t)
	before, _ := app.store.GetEntity(context.Background(), "TKT-001")
	titleBefore := before.Properties["title"]

	body := `{
		"properties": {"title": "Should NOT be saved"},
		"relations": {"nonexistent": {"data": []}}
	}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422; body=%s", rec.Code, rec.Body.String())
	}

	after, _ := app.store.GetEntity(context.Background(), "TKT-001")
	if after.Properties["title"] != titleBefore {
		t.Errorf("title changed to %q, want %q (entity should be untouched on relation 422)",
			after.Properties["title"], titleBefore)
	}
}

// AC18: legacy + modern shapes mixed in one body return 400 deterministically.
func TestModern_AC18_MixedShapesReturns400Deterministically(t *testing.T) {
	app := newRelationsTestApp(t)
	bodies := []string{
		`{"relations": {"tagged": ["L-001"], "belongs-to": {"data": [{"type":"category","id":"C-001"}]}}}`,
		`{"relations": {"belongs-to": {"data": [{"type":"category","id":"C-001"}]}, "tagged": ["L-001"]}}`,
	}
	for _, body := range bodies {
		rec := patch(t, app, "tickets", "TKT-001", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s: status=%d, want 400; body=%s", body, rec.Code, rec.Body.String())
		}
	}
}

// AC19: sibling-key rejection.
func TestModern_AC19_UnknownSiblingKeyReturns400(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": {"datas": [{"type":"label","id":"L-001"}]}}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

// AC20a: non-string element in meta_unset returns 400.
func TestModern_AC20a_NonStringInMetaUnsetReturns400(t *testing.T) {
	app := newRelationsTestApp(t)
	cases := []string{
		`{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta_unset":["x", null]}]}}}`,
		`{"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta_unset":["x", 5]}]}}}`,
	}
	for _, body := range cases {
		rec := patch(t, app, "tickets", "TKT-001", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s: status=%d, want 400", body, rec.Code)
		}
	}
}

// The legacy IDs-only shape is no longer accepted. PATCH that
// submits it gets a 400 with `legacy_shape_unsupported`; no edges
// are written.
func TestModern_LegacyShape_Rejected(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{"relations": {"tagged": ["L-001", "L-002"]}}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "legacy_shape_unsupported") {
		t.Errorf("body should name legacy_shape_unsupported: %s", rec.Body.String())
	}
	if rels := outgoingByType(app, "TKT-001", "tagged"); len(rels) != 0 {
		t.Errorf("no edges should have been written, got %d", len(rels))
	}
}

// AC16: combined PATCH (properties + relations).
func TestModern_AC16_CombinedPatch(t *testing.T) {
	app := newRelationsTestApp(t)
	body := `{
		"properties": {"status": "in-progress"},
		"relations": {"tagged": {"data": [{"type":"label","id":"L-001","meta":{"weight":3}}]}}
	}`
	rec := patch(t, app, "tickets", "TKT-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := app.store.GetEntity(context.Background(), "TKT-001")
	if got.Properties["status"] != "in-progress" {
		t.Errorf("status=%v, want in-progress", got.Properties["status"])
	}
	if rels := outgoingByType(app, "TKT-001", "tagged"); len(rels) != 1 || rels[0].Properties["weight"] != float64(3) {
		t.Errorf("relations not as expected: %v", rels)
	}
}
