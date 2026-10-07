package dataentry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
