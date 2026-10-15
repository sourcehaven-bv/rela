package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// incomingRelMeta builds a metamodel where a persoon points at a taak via
// `verantwoordelijk_voor`, which declares an inverse `verantwoordelijken`.
// This is the fixture for the TKT-ODHV2D incoming-relation-column mechanism.
func incomingRelMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"persoon": {
				Label:    "Persoon",
				Plural:   "persoons",
				IDPrefix: "PERS",
				Properties: map[string]metamodel.PropertyDef{
					"title":   {Type: "string", Required: true},
					"segment": {Type: "string"},
				},
			},
			"taak": {
				Label:    "Taak",
				Plural:   "taaks",
				IDPrefix: "TASK",
				Properties: map[string]metamodel.PropertyDef{
					"title":   {Type: "string", Required: true},
					"segment": {Type: "string"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"verantwoordelijk_voor": {
				Label:   "verantwoordelijk voor",
				From:    []string{"persoon"},
				To:      []string{"taak"},
				Inverse: &metamodel.InverseDef{ID: "verantwoordelijken"},
			},
		},
	}
}

// incomingRelConfig defines a taak list with an incoming relation column and a
// persoon list with an outgoing relation column, so both directions are tested.
func incomingRelConfig() *Config {
	return &Config{
		App: AppConfig{Name: "Incoming Rel Test"},
		Lists: map[string]List{
			"taaks": {
				EntityType: "taak",
				Title:      "Taken",
				Columns: []ListColumn{
					{Property: "title", Label: "Title"},
					{Relation: "verantwoordelijk_voor", Direction: "incoming", Label: "Verantwoordelijken"},
				},
			},
			"persoons": {
				EntityType: "persoon",
				Title:      "Personen",
				Columns: []ListColumn{
					{Property: "title", Label: "Title"},
					{Relation: "verantwoordelijk_voor", Label: "Taken"},
				},
			},
		},
	}
}

// newIncomingRelApp wires an App over incomingRelMeta with the given entities
// and relations seeded into the store.
func newIncomingRelApp(t *testing.T, entities []*entity.Entity, relations []*entity.Relation) *App {
	t.Helper()
	meta := incomingRelMeta()
	cfg := incomingRelConfig()

	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(ctx.CacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, ctx))
	f := &fixture{entities: entities, relations: relations}
	seedFromFixture(svc.Store(), f)

	app := &App{fieldResolver: NopFieldVerdictResolver{}}
	app.live = testLiveHub(app)
	rebindApp(app, fs, ctx, svc)
	app.schema.Publish(&Schema{
		Cfg:        cfg,
		Meta:       meta,
		OpenAPIGen: openapi.New(meta, openapi.Config{Title: cfg.App.Name}),
	})
	return app
}

// listWithIncludesResponse mirrors the anonymous response shape emitted by
// handleV1ListEntities when includes are requested.
type listWithIncludesResponse struct {
	Data     []v1.Entity          `json:"data"`
	Meta     v1.ListMeta          `json:"meta"`
	Included map[string]v1.Entity `json:"included"`
}

func decodeListWithIncludes(t *testing.T, w *httptest.ResponseRecorder) listWithIncludesResponse {
	t.Helper()
	var resp listWithIncludesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list response: %v\nbody: %s", err, w.Body.String())
	}
	return resp
}

func findRow(t *testing.T, rows []v1.Entity, id string) v1.Entity {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("row %q not found in %d rows", id, len(rows))
	return v1.Entity{}
}

// TestListIncomingRelationColumn verifies the shared wire mechanism from
// TKT-ODHV2D: incoming edges reach the list wire under the relation's inverse
// key, outgoing edges are unaffected, and multiple sources are all present.
func TestListIncomingRelationColumn(t *testing.T) {
	const (
		taakID   = "TASK-VKZ2"
		persAID  = "PERS-JV"
		persBID  = "PERS-AB"
		otherTID = "TASK-OTHER"
	)

	entities := []*entity.Entity{
		{ID: taakID, Type: "taak", Properties: map[string]any{"title": "Ship it"}},
		{ID: otherTID, Type: "taak", Properties: map[string]any{"title": "Other"}},
		{ID: persAID, Type: "persoon", Properties: map[string]any{"title": "Jeroen Vloothuis"}},
		{ID: persBID, Type: "persoon", Properties: map[string]any{"title": "Alice B"}},
	}

	t.Run("single incoming source keyed under inverse", func(t *testing.T) {
		rels := []*entity.Relation{
			entity.NewRelation(persAID, "verantwoordelijk_voor", taakID),
		}
		app := newIncomingRelApp(t, entities, rels)

		w := doRequest(t, app, http.MethodGet, "/api/v1/taaks?include=*")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		resp := decodeListWithIncludes(t, w)

		row := findRow(t, resp.Data, taakID)
		// Incoming edge lives under the inverse key, NOT the relation type.
		got := row.Relations["verantwoordelijken"]
		if len(got) != 1 || got[0] != persAID {
			t.Fatalf("row.Relations[verantwoordelijken] = %v, want [%s]", got, persAID)
		}
		if _, collides := row.Relations["verantwoordelijk_voor"]; collides {
			t.Fatalf("outgoing key verantwoordelijk_voor must not appear on a taak row: %v", row.Relations)
		}
		// The include map carries the source persoon so the SPA can resolve
		// the ID to a title.
		inc, ok := resp.Included[persAID]
		if !ok {
			t.Fatalf("included map missing source %s; included=%v", persAID, resp.Included)
		}
		if inc.Title != "Jeroen Vloothuis" {
			t.Fatalf("included[%s]._title = %q, want %q", persAID, inc.Title, "Jeroen Vloothuis")
		}
	})

	t.Run("multiple incoming sources both present", func(t *testing.T) {
		rels := []*entity.Relation{
			entity.NewRelation(persAID, "verantwoordelijk_voor", taakID),
			entity.NewRelation(persBID, "verantwoordelijk_voor", taakID),
		}
		app := newIncomingRelApp(t, entities, rels)

		w := doRequest(t, app, http.MethodGet, "/api/v1/taaks?include=*")
		resp := decodeListWithIncludes(t, w)
		row := findRow(t, resp.Data, taakID)

		got := row.Relations["verantwoordelijken"]
		if len(got) != 2 {
			t.Fatalf("want 2 incoming sources, got %v", got)
		}
		seen := map[string]bool{}
		for _, id := range got {
			seen[id] = true
		}
		if !seen[persAID] || !seen[persBID] {
			t.Fatalf("incoming sources = %v, want both %s and %s", got, persAID, persBID)
		}
	})

	t.Run("outgoing column unchanged for the source entity", func(t *testing.T) {
		rels := []*entity.Relation{
			entity.NewRelation(persAID, "verantwoordelijk_voor", taakID),
		}
		app := newIncomingRelApp(t, entities, rels)

		w := doRequest(t, app, http.MethodGet, "/api/v1/persoons?include=*")
		resp := decodeListWithIncludes(t, w)
		row := findRow(t, resp.Data, persAID)

		// Outgoing edge keyed under the relation type, exactly as before.
		got := row.Relations["verantwoordelijk_voor"]
		if len(got) != 1 || got[0] != taakID {
			t.Fatalf("row.Relations[verantwoordelijk_voor] = %v, want [%s]", got, taakID)
		}
		if _, leaked := row.Relations["verantwoordelijken"]; leaked {
			t.Fatalf("inverse key must not appear on a persoon row: %v", row.Relations)
		}
	})
}

// TestListOutgoingRelationsByteIdentical is the regression guard: a taak row
// with NO incoming edges must serialize its relations map exactly as before
// the incoming-edge change (i.e. an absent/empty relations map, never a
// spurious inverse key).
func TestListOutgoingRelationsByteIdentical(t *testing.T) {
	const persID = "PERS-JV"
	entities := []*entity.Entity{
		{ID: persID, Type: "persoon", Properties: map[string]any{"title": "Jeroen"}},
	}
	// One outgoing edge from the persoon to a taak.
	taak := &entity.Entity{ID: "TASK-1", Type: "taak", Properties: map[string]any{"title": "T"}}
	entities = append(entities, taak)
	rels := []*entity.Relation{entity.NewRelation(persID, "verantwoordelijk_voor", "TASK-1")}

	app := newIncomingRelApp(t, entities, rels)
	w := doRequest(t, app, http.MethodGet, "/api/v1/persoons?include=*")
	resp := decodeListWithIncludes(t, w)
	row := findRow(t, resp.Data, persID)

	// The persoon row's relations map must contain ONLY the outgoing key.
	if len(row.Relations) != 1 {
		t.Fatalf("persoon row relations should have exactly the outgoing key, got %v", row.Relations)
	}
	got, err := json.Marshal(row.Relations)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"verantwoordelijk_voor":["TASK-1"]}`
	if string(got) != want {
		t.Fatalf("outgoing relations map = %s, want %s", got, want)
	}
}

// TestListIncludeIncomingPeer_StripsHiddenProperty pins the read path a kanban
// card's related-property field relies on (TKT-8Z2LT3): the board fetches with
// `include=*` and reads the property off the incoming peer in `included`. A
// property the reader may not see must be absent there, so the card shows
// nothing for it, while a visible property of the same peer still arrives.
func TestListIncludeIncomingPeer_StripsHiddenProperty(t *testing.T) {
	const (
		taakID = "TASK-SEG"
		persID = "PERS-SEG"
	)
	entities := []*entity.Entity{
		{ID: taakID, Type: "taak", Properties: map[string]any{"title": "Deal", "segment": "row-segment"}},
		{ID: persID, Type: "persoon", Properties: map[string]any{
			"title": "Ziekenhuis", "segment": "secret-segment",
		}},
	}
	rels := []*entity.Relation{entity.NewRelation(persID, "verantwoordelijk_voor", taakID)}
	app := newIncomingRelApp(t, entities, rels)
	// Hidden on the peer's type only. The board row's type declares a visible
	// segment too, so redacting the peer with the row type's rules would leak.
	app.fieldResolver = fieldsByTypeResolver{"persoon": {Visible: map[string]bool{"segment": false}}}

	w := doRequest(t, app, http.MethodGet, "/api/v1/taaks?include=*")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret-segment") {
		t.Fatalf("hidden property value reached the wire: %s", w.Body.String())
	}
	resp := decodeListWithIncludes(t, w)
	inc, ok := resp.Included[persID]
	if !ok {
		t.Fatalf("included map missing incoming peer %s; included=%v", persID, resp.Included)
	}
	if _, present := inc.Properties["segment"]; present {
		t.Errorf("included[%s] carries hidden property segment: %v", persID, inc.Properties)
	}
	if inc.Properties["title"] != "Ziekenhuis" {
		t.Errorf("included[%s].title = %v, want the visible value", persID, inc.Properties["title"])
	}
	if row := findRow(t, resp.Data, taakID); row.Properties["segment"] != "row-segment" {
		t.Errorf("row segment = %v, want the value visible on its own type", row.Properties["segment"])
	}
}

// fieldsByTypeResolver returns field verdicts per entity type, so a test can
// hide a property on one type and leave the same name visible on another.
// Relation verdicts are empty.
type fieldsByTypeResolver map[string]FieldVerdicts

func (r fieldsByTypeResolver) FieldVerdicts(_ context.Context, e *entity.Entity) FieldVerdicts {
	return r[e.Type]
}

func (fieldsByTypeResolver) RelationVerdicts(context.Context, *entity.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

func (fieldsByTypeResolver) UnconditionalRelationVerdicts(context.Context, *entity.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

func (r fieldsByTypeResolver) FieldVisibleOnEveryRow(_ context.Context, entityType, property string) bool {
	return r[entityType].IsVisible(property)
}
