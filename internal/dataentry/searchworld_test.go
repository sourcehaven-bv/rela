package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Cross-type search under a world (BUG-SMPOZB).
//
// `/api/v1/_search` backs the command palette, the search page and the entity
// picker. It was refused by worldCapablePath, so `app.default_world` never
// reached it and a faced entity with no default face could not be found,
// although the list for its type showed it.

// seedSearchWorld seeds the faced tickets of seedFacedTickets plus TKT-ONLY,
// which has a published face and no default face at all: the BUG-SMPOZB shape.
// With configured set, `app.default_world` names `published`.
func seedSearchWorld(t *testing.T, configured bool) *App {
	t.Helper()
	app := newTestAppV1(t)
	seedFacedTickets(t, app)
	if err := app.store.CreateEntity(context.Background(), &entity.Entity{
		ID: "TKT-ONLY", Type: "ticket", Face: entity.Face("published"),
		Properties: map[string]any{"title": "narwhal"},
	}); err != nil {
		t.Fatalf("seed published-only ticket: %v", err)
	}
	if configured {
		state := *app.State()
		cfg := *state.Cfg
		cfg.App.DefaultWorld = "published"
		state.Cfg = &cfg
		app.schema.Publish(&state)
	}
	return app
}

func sortedSearchIDs(t *testing.T, app *App, path string) []string {
	t.Helper()
	ids := searchIDs(t, app, path)
	slices.Sort(ids)
	return ids
}

func TestSearch_ResolvesThroughTheWorld(t *testing.T) {
	tests := []struct {
		name       string
		configured bool
		path       string
		want       []string
	}{
		{
			name: "default world finds the published-only entity", configured: true,
			path: "/api/v1/_search?q=narwhal", want: []string{"TKT-ONLY"},
		},
		{
			name: "default world matches the published text", configured: true,
			path: "/api/v1/_search?q=walrus", want: []string{"TKT-PUB"},
		},
		{
			name: "default world does not match draft text", configured: true,
			path: "/api/v1/_search?q=sardine", want: nil,
		},
		{
			name: "default world scopes a type-only query", configured: true,
			path: "/api/v1/_search?q=type:ticket", want: []string{"TKT-ONLY", "TKT-PUB"},
		},
		{
			// No type and no ACL: the wildcard listing in visibleListByTypes.
			name: "default world scopes a property-only query", configured: true,
			path: "/api/v1/_search?q=prop:title=narwhal", want: []string{"TKT-ONLY"},
		},
		{
			name: "explicit world without a configured default",
			path: "/api/v1/_search?q=narwhal&world=published", want: []string{"TKT-ONLY"},
		},
		{
			name: "explicit default world reaches the draft faces", configured: true,
			path: "/api/v1/_search?q=sardine&world=default", want: []string{"TKT-DRAFT", "TKT-PUB"},
		},
		{
			name: "no world configured keeps the default world",
			path: "/api/v1/_search?q=sardine", want: []string{"TKT-DRAFT", "TKT-PUB"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := seedSearchWorld(t, tc.configured)
			got := sortedSearchIDs(t, app, tc.path)
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// A hit is served as the face the world resolved, so the row carries the
// same `_world` provenance a list row does.
func TestSearch_HitCarriesTheResolvedFace(t *testing.T) {
	app := seedSearchWorld(t, true)
	rows := listRows(t, app, "/api/v1/_search?q=walrus")
	if len(rows) != 1 {
		t.Fatalf("want one row; got %v", rows)
	}
	if title := rows[0]["properties"].(map[string]any)["title"]; title != "walrus onboarding" {
		t.Errorf("title = %v, want the published face's", title)
	}
	w, ok := rows[0]["_world"].(map[string]any)
	if !ok || w["face"] != "published" {
		t.Errorf("_world = %v, want face published", rows[0]["_world"])
	}
}

// A denied handle carries the zero scope, which is the default world. Both
// executeQuery branches must therefore check the denial rather than search
// with that scope.
func TestSearch_DeniedWorldFindsNothing(t *testing.T) {
	app := seedSearchWorld(t, false)
	for _, q := range []string{"sardine", "type:ticket"} {
		t.Run(q, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q="+q, http.NoBody)
			ctx := withWorld(withReadGate(aliceCtx(), nopReadGate{}),
				worldHandle{name: "published", denied: true})
			rec := httptest.NewRecorder()
			app.handleV1Search(rec, req.WithContext(ctx))
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
			var resp struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(resp.Data) != 0 {
				t.Errorf("a denied world must find nothing; got %v", resp.Data)
			}
		})
	}
}
