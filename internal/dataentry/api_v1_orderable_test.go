package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newOrderableRelationsTestApp builds an App with an `orderable: <mode>`
// relation between recipe and step. Mirrors newReverseRelationsTestApp.
func newOrderableRelationsTestApp(t *testing.T, mode metamodel.OrderableMode) *App {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"recipe": {Label: "Recipe", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
			"step":   {Label: "Step", Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"has-step": {
				Label:     "has step",
				From:      []string{"recipe"},
				To:        []string{"step"},
				Orderable: mode,
			},
		},
	}
	cfg := &Config{
		App: AppConfig{Name: "Orderable Test", Description: "x"},
	}
	return newAppFromParts(cfg, meta, newFixture())
}

// seedOrderableFixture seeds four edges whose target IDs are chosen so the
// alphabetical-by-key store order DIFFERS from the orderable-by-property
// order. Without that property the assertion would pass even if the sort
// were a no-op. memstore returns alphabetical: STP-M, STP-X, STP-Y, STP-Z.
// Orderable: STP-Z=1, STP-X=2, STP-M=3, STP-Y=missing.
func seedOrderableFixture(t *testing.T, app *App, prop string) string {
	t.Helper()
	const recipeID = "REC-001"
	seedEntity(app, &entity.Entity{ID: recipeID, Type: "recipe", Properties: map[string]interface{}{"title": "Soup"}})
	type stepSeed struct {
		id    string
		order interface{}
	}
	steps := []stepSeed{
		{"STP-Z", 1.0},
		{"STP-X", 2.0},
		{"STP-M", 3.0},
		{"STP-Y", nil},
	}
	for _, s := range steps {
		seedEntity(app, &entity.Entity{ID: s.id, Type: "step", Properties: map[string]interface{}{"title": s.id}})
		props := map[string]interface{}{}
		if s.order != nil {
			props[prop] = s.order
		}
		_, err := app.store.CreateRelation(t.Context(), recipeID, "has-step", s.id, &store.RelationData{Properties: props})
		if err != nil {
			t.Fatalf("seed relation: %v", err)
		}
	}
	return recipeID
}

func TestV1EntityRelations_OutgoingOrderableSorted(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableOutgoing)
	recipeID := seedOrderableFixture(t, app, metamodel.OrderPropertyOut)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+recipeID+"/relations", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "recipe", recipeID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var grouped map[string][]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &grouped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := make([]string, 0, len(grouped["has-step"]))
	for _, r := range grouped["has-step"] {
		got = append(got, r["id"].(string))
	}
	want := []string{"STP-Z", "STP-X", "STP-M", "STP-Y"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got %q, want %q (full got=%v)", i, got[i], id, got)
		}
	}
}

func TestV1GetRelationType_OutgoingOrderableSorted(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableOutgoing)
	recipeID := seedOrderableFixture(t, app, metamodel.OrderPropertyOut)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/recipes/"+recipeID+"/relations/has-step", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetRelationType(rec, req, "recipe", recipeID, "has-step")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var edges []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &edges); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := make([]string, 0, len(edges))
	for _, e := range edges {
		got = append(got, e["id"].(string))
	}
	want := []string{"STP-Z", "STP-X", "STP-M", "STP-Y"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("position %d: got %q, want %q", i, got[i], id)
		}
	}
}

func TestV1EntityRelations_NonOrderable_NotSortedByOrderProperty(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableNone)
	recipeID := seedOrderableFixture(t, app, metamodel.OrderPropertyOut)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+recipeID+"/relations", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, "recipe", recipeID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var grouped map[string][]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &grouped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := make([]string, 0, len(grouped["has-step"]))
	for _, r := range grouped["has-step"] {
		got = append(got, r["id"].(string))
	}
	orderableSort := []string{"STP-Z", "STP-X", "STP-M", "STP-Y"}
	if equalStringSlices(got, orderableSort) {
		t.Errorf("non-orderable type was sorted by _order_out anyway: got %v", got)
	}
}

func TestV1Schema_AdvertisesOrderableFlag(t *testing.T) {
	app := newOrderableRelationsTestApp(t, metamodel.OrderableBoth)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var schema struct {
		Relations map[string]V1RelationType `json:"relations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &schema); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt, ok := schema.Relations["has-step"]
	if !ok {
		t.Fatalf("missing has-step in schema")
	}
	if rt.Orderable == nil {
		t.Fatalf("expected Orderable to be advertised")
	}
	if !rt.Orderable.Outgoing || !rt.Orderable.Incoming {
		t.Errorf("expected both sides true, got %+v", rt.Orderable)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
