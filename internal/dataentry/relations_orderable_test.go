package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func newOrderableModernTestApp(t *testing.T, mode metamodel.OrderableMode) *App {
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
	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Orderable PATCH Test"},
		Forms:      make(map[string]dataentryconfig.Form),
		Lists:      make(map[string]dataentryconfig.List),
		Views:      make(map[string]dataentryconfig.ViewConfig),
		Kanbans:    make(map[string]dataentryconfig.Kanban),
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture())
	app.broker = newEventBroker()
	seedEntity(app, &entity.Entity{ID: "REC-001", Type: "recipe", Properties: map[string]interface{}{"title": "Soup"}})
	seedEntity(app, &entity.Entity{ID: "STP-001", Type: "step", Properties: map[string]interface{}{"title": "Boil"}})
	return app
}

func patchRecipe(t *testing.T, app *App, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/recipes/REC-001", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	app.handleV1UpdateEntity(rec, req, "recipe", "recipes", "REC-001")
	return rec
}

// PATCH that sets a valid numeric _order_out is accepted (200) and persisted.
func TestOrderable_PatchSetsOrderOut(t *testing.T) {
	app := newOrderableModernTestApp(t, metamodel.OrderableOutgoing)
	if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
		&store.RelationData{Properties: map[string]interface{}{metamodel.OrderPropertyOut: 1.0}}); err != nil {
		t.Fatal(err)
	}
	body := `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001","meta":{"_order_out":2.5}}]}}}`
	rec := patchRecipe(t, app, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := app.store.GetRelation(context.Background(), "REC-001", "has-step", "STP-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties[metamodel.OrderPropertyOut] != 2.5 {
		t.Errorf("_order_out=%v, want 2.5", got.Properties[metamodel.OrderPropertyOut])
	}
}

// PATCH with a non-finite numeric value on a managed order property is a
// hard wire-format violation (400).
func TestOrderable_PatchRejectsNonFiniteOrder(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "string instead of number",
			body: `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001","meta":{"_order_out":"abc"}}]}}}`,
		},
		{
			// 1e500 overflows float64 → +Inf, which we reject.
			name: "overflows to infinity",
			body: `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001","meta":{"_order_out":1e500}}]}}}`,
		},
		{
			// boolean is not numeric.
			name: "boolean rejected",
			body: `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001","meta":{"_order_out":true}}]}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrderableModernTestApp(t, metamodel.OrderableOutgoing)
			if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
				&store.RelationData{Properties: map[string]interface{}{metamodel.OrderPropertyOut: 1.0}}); err != nil {
				t.Fatal(err)
			}
			rec := patchRecipe(t, app, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			// The "overflows to infinity" case is rejected by Go's
			// JSON decoder before our validateManagedOrderMeta runs
			// (1e500 → field_invalid_type during v1.ResourceIdentifier
			// decode). The string and boolean cases reach our check.
			// Either way the request must 400 — we accept both codes.
			var problem struct {
				Type   string `json:"type"`
				Status int    `json:"status"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Status != 400 {
				t.Errorf("problem.Status = %d, want 400", problem.Status)
			}
		})
	}
}

// patchEdge issues a per-edge PATCH (the path used by the SPA for reorder).
func patchEdge(t *testing.T, app *App, relType, targetID, body string) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/recipes/REC-001/relations/" + relType + "/" + targetID
	req := httptest.NewRequest(http.MethodPatch, url, bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	app.handleV1UpdateRelation(rec, req, "recipe", "REC-001", relType, targetID)
	return rec
}

// G2: per-edge PATCH wire validation for the incoming-side managed property.
// Mirrors TestOrderable_PatchRejectsNonFiniteOrder but exercises _order_in
// instead of _order_out. Without this test, a regression that loosens the
// incoming-side check at the per-edge handler would slip through.
func TestOrderable_PatchEdgeRejectsNonFiniteOrderIn(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"string", `{"meta":{"_order_in":"abc"}}`},
		{"boolean", `{"meta":{"_order_in":true}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrderableModernTestApp(t, metamodel.OrderableIncoming)
			if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
				&store.RelationData{Properties: map[string]interface{}{metamodel.OrderPropertyIn: 1.0}}); err != nil {
				t.Fatal(err)
			}
			rec := patchEdge(t, app, "has-step", "STP-001", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// G3: orderable:incoming mode rejects non-finite values via the same
// per-edge endpoint that drives the SPA reorder. Sanity check that
// incoming-mode wires up identically to outgoing-mode.
func TestOrderable_PatchEdgeIncomingModeAcceptsFinite(t *testing.T) {
	app := newOrderableModernTestApp(t, metamodel.OrderableIncoming)
	if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
		&store.RelationData{Properties: map[string]interface{}{metamodel.OrderPropertyIn: 1.0}}); err != nil {
		t.Fatal(err)
	}
	body := `{"meta":{"_order_in":2.5}}`
	rec := patchEdge(t, app, "has-step", "STP-001", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := app.store.GetRelation(context.Background(), "REC-001", "has-step", "STP-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties[metamodel.OrderPropertyIn] != 2.5 {
		t.Errorf("_order_in = %v, want 2.5", got.Properties[metamodel.OrderPropertyIn])
	}
}

// PATCH that omits the order property does not trigger validation
// (managed properties are optional on writes that don't touch them).
func TestOrderable_PatchWithoutOrderUntouched(t *testing.T) {
	app := newOrderableModernTestApp(t, metamodel.OrderableOutgoing)
	if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
		&store.RelationData{Properties: map[string]interface{}{metamodel.OrderPropertyOut: 1.0}}); err != nil {
		t.Fatal(err)
	}
	body := `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001"}]}}}`
	rec := patchRecipe(t, app, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got, err := app.store.GetRelation(context.Background(), "REC-001", "has-step", "STP-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties[metamodel.OrderPropertyOut] != 1.0 {
		t.Errorf("_order_out should remain 1.0, got %v", got.Properties[metamodel.OrderPropertyOut])
	}
}

// PATCH on a non-orderable type writing _order_out succeeds (the property
// is just a plain meta key) but produces an `unknown_meta_key` warning.
func TestOrderable_NonOrderableTypeStillAcceptsOrderKey(t *testing.T) {
	app := newOrderableModernTestApp(t, metamodel.OrderableNone)
	if _, err := app.store.CreateRelation(context.Background(), "REC-001", "has-step", "STP-001",
		&store.RelationData{Properties: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	body := `{"relations":{"has-step":{"data":[{"type":"step","id":"STP-001","meta":{"_order_out":1.5}}]}}}`
	rec := patchRecipe(t, app, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Body should report the unknown_meta_key warning.
	var resp struct {
		Warnings []Warning `json:"warnings"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	found := false
	for _, w := range resp.Warnings {
		if w.Code == "unknown_meta_key" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown_meta_key warning for non-orderable type; got %+v", resp.Warnings)
	}
}
