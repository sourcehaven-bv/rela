package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// relationRuleSchema: a closed ticket belongs to a category, enforced on
// write (TKT-EBARYBI).
const relationRuleSchema = `
version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title: {type: string}
      status: {type: string}
  category:
    label: Category
    id_prefix: "C-"
    properties:
      title: {type: string}
relations:
  belongs-to:
    from: [ticket]
    to: [category]
validations:
  - name: closed-has-category
    description: A closed ticket belongs to a category.
    entity_type: ticket
    enforce: write
    when: ["status=closed"]
    then_condition: "related(entity, 'belongs-to')"
`

func newRelationRuleApp(t *testing.T) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relationRuleSchema))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &dataentryconfig.Config{
		App:   dataentryconfig.AppConfig{Name: "Relation rule"},
		Forms: map[string]dataentryconfig.Form{}, Lists: map[string]dataentryconfig.List{},
		Views: map[string]dataentryconfig.ViewConfig{}, Kanbans: map[string]dataentryconfig.Kanban{},
	}
	app := newAppFromParts(cfg, meta, newFixture())
	app.broker = newEventBroker()
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "t", "status": "closed"}})
	seedEntity(app, &entity.Entity{ID: "C-001", Type: "category", Properties: map[string]any{"title": "c"}})
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-001"))
	return app
}

// TestRelationRule_RefusedRelationWritesAre422 pins that every relation
// write path answers a relation-rule refusal with the 422 an entity write
// gets, names the rule, and leaves the edge in place.
func TestRelationRule_RefusedRelationWritesAre422(t *testing.T) {
	tests := []struct {
		name string
		do   func(app *App) *httptest.ResponseRecorder
	}{
		{"per-relation DELETE", func(app *App) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete,
				"/api/v1/tickets/TKT-001/relations/belongs-to/C-001", http.NoBody)
			app.write.handleV1DeleteRelation(rec, req, "ticket", "TKT-001", "belongs-to", "C-001")
			return rec
		}},
		{"PATCH relations remove", func(app *App) *httptest.ResponseRecorder {
			return patch(t, app, "tickets", "TKT-001",
				`{"relations":{"belongs-to":{"remove":[{"type":"category","id":"C-001"}]}}}`)
		}},
		{"PATCH relations full linkage", func(app *App) *httptest.ResponseRecorder {
			return patch(t, app, "tickets", "TKT-001", `{"relations":{"belongs-to":{"data":[]}}}`)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newRelationRuleApp(t)
			rec := tc.do(app)
			body := rec.Body.String()
			if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(body, "validation_failed") {
				t.Fatalf("got %d %s, want 422 validation_failed", rec.Code, body)
			}
			if !strings.Contains(body, "closed-has-category") {
				t.Errorf("body does not name the rule: %s", body)
			}
			if got := outgoingByType(app, "TKT-001", "belongs-to"); len(got) != 1 {
				t.Errorf("edges = %v, want the edge kept", got)
			}
		})
	}
}

// TestRelationRule_PatchJudgesTheStateItLeaves pins that a PATCH changing
// properties and relations together is judged on the state it leaves, and
// that a refused PATCH writes neither part.
func TestRelationRule_PatchJudgesTheStateItLeaves(t *testing.T) {
	const add = `{"properties":{"status":"closed"},` +
		`"relations":{"belongs-to":{"add":[{"type":"category","id":"C-001"}]}}}`
	tests := []struct {
		name      string
		id, body  string
		extra     bool // TKT-001 also belongs to C-002
		refused   bool
		wantEdges int
		wantTitle string
	}{
		{name: "status and the needed edge together", id: "TKT-002", body: add, wantEdges: 1},
		{
			name: "status without the edge", id: "TKT-002", body: `{"properties":{"status":"closed"}}`,
			refused: true,
		},
		{
			name: "property change and removing the last edge", id: "TKT-001",
			body: `{"properties":{"title":"new"},` +
				`"relations":{"belongs-to":{"remove":[{"type":"category","id":"C-001"}]}}}`,
			refused: true, wantEdges: 1, wantTitle: "t",
		},
		{
			name: "removing every edge of an unbounded relation at once", id: "TKT-001", extra: true,
			body:    `{"relations":{"belongs-to":{"data":[]}}}`,
			refused: true, wantEdges: 2,
		},
		{
			name: "removing one of two edges", id: "TKT-001", extra: true,
			body:      `{"relations":{"belongs-to":{"remove":[{"type":"category","id":"C-001"}]}}}`,
			wantEdges: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newRelationRuleApp(t)
			seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket",
				Properties: map[string]any{"title": "t", "status": "open"}})
			seedEntity(app, &entity.Entity{ID: "C-002", Type: "category", Properties: map[string]any{"title": "d"}})
			if tc.extra {
				seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "C-002"))
			}
			rec := patch(t, app, "tickets", tc.id, tc.body)
			want := http.StatusOK
			if tc.refused {
				want = http.StatusUnprocessableEntity
			}
			if rec.Code != want {
				t.Fatalf("got %d %s, want %d", rec.Code, rec.Body, want)
			}
			if got := outgoingByType(app, tc.id, "belongs-to"); len(got) != tc.wantEdges {
				t.Errorf("edges = %v, want %d", got, tc.wantEdges)
			}
			if tc.wantTitle != "" {
				if got := storedTitle(t, app, tc.id); got != tc.wantTitle {
					t.Errorf("title = %v, want %q kept", got, tc.wantTitle)
				}
			}
		})
	}
}

func storedTitle(t *testing.T, app *App, id string) any {
	t.Helper()
	for _, e := range entitiesByType(app, "ticket") {
		if e.ID == id {
			return e.Properties["title"]
		}
	}
	t.Fatalf("%s not stored", id)
	return nil
}
