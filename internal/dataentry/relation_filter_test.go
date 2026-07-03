package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// relationFilterMeta is a small metamodel exercising both relation
// directions: taak--verantwoordelijk_voor(incoming)-->persoon and
// taak--belongs_to(outgoing)-->project.
func relationFilterMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:         "Taak",
				IDPrefix:      "TAAK-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
				PropertyOrder: []string{"title"},
			},
			"persoon": {
				Label:         "Persoon",
				IDPrefix:      "PER-",
				Properties:    map[string]metamodel.PropertyDef{"name": {Type: "string", Required: true}},
				PropertyOrder: []string{"name"},
			},
			"project": {
				Label:         "Project",
				IDPrefix:      "PRJ-",
				Properties:    map[string]metamodel.PropertyDef{"name": {Type: "string", Required: true}},
				PropertyOrder: []string{"name"},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			// persoon is responsible FOR taak: edge points persoon -> taak,
			// so from a taak's perspective the persoon is the incoming source.
			"verantwoordelijk_voor": {
				Label: "verantwoordelijk voor",
				From:  []string{"persoon"},
				To:    []string{"taak"},
			},
			"belongs_to": {
				Label: "belongs to",
				From:  []string{"taak"},
				To:    []string{"project"},
			},
		},
	}
}

// relationFilterApp builds an App whose taak list carries relation filter
// controls (incoming verantwoordelijk_voor + outgoing belongs_to) and seeds a
// small graph. Returns the app.
func relationFilterApp(t *testing.T) *App {
	t.Helper()
	meta := relationFilterMeta()
	cfg := &dataentryconfig.Config{
		App: dataentryconfig.AppConfig{Name: "Test"},
		Lists: map[string]dataentryconfig.List{
			"taken": {
				EntityType: "taak",
				Title:      "Taken",
				FilterControls: []dataentryconfig.FilterControl{
					{Relation: "verantwoordelijk_voor", Direction: dataentryconfig.DirectionIncoming},
					{Relation: "belongs_to"}, // default outgoing
				},
			},
		},
	}

	f := newFixture()
	// People
	f.AddNode(&entity.Entity{ID: "PER-001", Type: "persoon", Properties: map[string]interface{}{"name": "Jeroen Vloothuis"}})
	f.AddNode(&entity.Entity{ID: "PER-002", Type: "persoon", Properties: map[string]interface{}{"name": "Alice"}})
	// Projects
	f.AddNode(&entity.Entity{ID: "PRJ-001", Type: "project", Properties: map[string]interface{}{"name": "Apollo"}})
	f.AddNode(&entity.Entity{ID: "PRJ-002", Type: "project", Properties: map[string]interface{}{"name": "Gemini"}})
	// Tasks (seeded out of title order to prove sort wins)
	f.AddNode(&entity.Entity{ID: "TAAK-002", Type: "taak", Properties: map[string]interface{}{"title": "B task"}})
	f.AddNode(&entity.Entity{ID: "TAAK-001", Type: "taak", Properties: map[string]interface{}{"title": "A task"}})
	f.AddNode(&entity.Entity{ID: "TAAK-003", Type: "taak", Properties: map[string]interface{}{"title": "C task"}})
	f.AddNode(&entity.Entity{ID: "TAAK-004", Type: "taak", Properties: map[string]interface{}{"title": "D task"}})

	// Jeroen is responsible for TAAK-001 and TAAK-003; Alice for TAAK-002.
	f.AddEdge(entity.NewRelation("PER-001", "verantwoordelijk_voor", "TAAK-001"))
	f.AddEdge(entity.NewRelation("PER-001", "verantwoordelijk_voor", "TAAK-003"))
	f.AddEdge(entity.NewRelation("PER-002", "verantwoordelijk_voor", "TAAK-002"))
	// TAAK-004 has no responsible person.

	// belongs_to (outgoing) targets.
	f.AddEdge(entity.NewRelation("TAAK-001", "belongs_to", "PRJ-001")) // Apollo
	f.AddEdge(entity.NewRelation("TAAK-002", "belongs_to", "PRJ-002")) // Gemini
	f.AddEdge(entity.NewRelation("TAAK-003", "belongs_to", "PRJ-001")) // Apollo

	return newAppFromParts(cfg, meta, f)
}

// listTaken drives the list handler and returns the returned entity IDs.
// query is a url.Values that is safely encoded (values may contain spaces).
func listTaken(t *testing.T, app *App, query url.Values) []string {
	t.Helper()
	path := "/api/v1/taaks"
	if enc := query.Encode(); enc != "" {
		path += "?" + enc
	}
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "taak", "taaks")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status: got %d, body %s", rec.Code, rec.Body.String())
	}
	var resp v1.ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestV1ListRelationFilter(t *testing.T) {
	tests := []struct {
		name    string
		query   url.Values
		wantIDs []string
	}{
		{
			name:    "incoming relation filter keeps only matching rows",
			query:   url.Values{"filter[verantwoordelijk_voor]": {"Jeroen Vloothuis"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-001", "TAAK-003"},
		},
		{
			name:    "incoming relation filter for a different person",
			query:   url.Values{"filter[verantwoordelijk_voor]": {"Alice"}},
			wantIDs: []string{"TAAK-002"},
		},
		{
			name:    "incoming relation filter with no matches returns empty",
			query:   url.Values{"filter[verantwoordelijk_voor]": {"Nobody"}},
			wantIDs: []string{},
		},
		{
			name:    "outgoing relation filter (default direction)",
			query:   url.Values{"filter[belongs_to]": {"Apollo"}, "sort": {"title"}},
			wantIDs: []string{"TAAK-001", "TAAK-003"},
		},
		{
			name:    "outgoing relation filter different target",
			query:   url.Values{"filter[belongs_to]": {"Gemini"}},
			wantIDs: []string{"TAAK-002"},
		},
		{
			name:    "empty relation filter value is no constraint",
			query:   url.Values{"filter[verantwoordelijk_voor]": {""}, "sort": {"title"}},
			wantIDs: []string{"TAAK-001", "TAAK-002", "TAAK-003", "TAAK-004"},
		},
		{
			name: "relation + property filter combine (property still works)",
			query: url.Values{
				"filter[verantwoordelijk_voor]": {"Jeroen Vloothuis"},
				"filter[title]":                 {"A task"},
			},
			wantIDs: []string{"TAAK-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := relationFilterApp(t)
			got := listTaken(t, app, tt.query)
			if !equalStringSlices(got, tt.wantIDs) {
				t.Errorf("ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

// TestV1ListPropertyFilterUnaffected confirms a pure property filter (no
// relation involved) still behaves exactly as before the relation-filter pass.
func TestV1ListPropertyFilterUnaffected(t *testing.T) {
	app := relationFilterApp(t)
	got := listTaken(t, app, url.Values{"filter[title]": {"C task"}})
	if !equalStringSlices(got, []string{"TAAK-003"}) {
		t.Errorf("property filter ids = %v, want [TAAK-003]", got)
	}
}

// TestV1PositionRelationFilter checks that scope nav over a relation-filtered
// list yields prev/next/total consistent with the list ordering. Both go
// through scopedSortedEntities, so the relation filter must apply identically.
func TestV1PositionRelationFilter(t *testing.T) {
	app := relationFilterApp(t)

	// The list, sorted by title, of tasks Jeroen is responsible for:
	// TAAK-001 (A task), TAAK-003 (C task).
	scope := ScopeDescriptor{
		Source:  "list",
		Type:    "taak",
		Filters: map[string]string{"filter[verantwoordelijk_voor]": "Jeroen Vloothuis"},
		Sort:    "title",
	}

	// Sanity: the list handler returns exactly these two, in this order.
	gotList := listTaken(t, app, url.Values{
		"filter[verantwoordelijk_voor]": {"Jeroen Vloothuis"},
		"sort":                          {"title"},
	})
	if !equalStringSlices(gotList, []string{"TAAK-001", "TAAK-003"}) {
		t.Fatalf("precondition list ids = %v, want [TAAK-001 TAAK-003]", gotList)
	}

	t.Run("first entity", func(t *testing.T) {
		rec, pos := getPosition(t, app, "TAAK-001", scope)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d", rec.Code)
		}
		if pos.Current != 1 || pos.Total != 2 {
			t.Errorf("current/total = %d/%d, want 1/2", pos.Current, pos.Total)
		}
		if pos.Prev != nil {
			t.Errorf("prev = %v, want nil", pos.Prev)
		}
		if pos.Next == nil || pos.Next.ID != "TAAK-003" {
			t.Errorf("next = %v, want TAAK-003", pos.Next)
		}
	})

	t.Run("last entity", func(t *testing.T) {
		rec, pos := getPosition(t, app, "TAAK-003", scope)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d", rec.Code)
		}
		if pos.Current != 2 || pos.Total != 2 {
			t.Errorf("current/total = %d/%d, want 2/2", pos.Current, pos.Total)
		}
		if pos.Prev == nil || pos.Prev.ID != "TAAK-001" {
			t.Errorf("prev = %v, want TAAK-001", pos.Prev)
		}
		if pos.Next != nil {
			t.Errorf("next = %v, want nil", pos.Next)
		}
	})

	t.Run("entity filtered out is not in scope", func(t *testing.T) {
		// TAAK-002 (Alice's) is excluded by the filter, so it must 404.
		rec, _ := getPosition(t, app, "TAAK-002", scope)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rec.Code)
		}
	})
}
