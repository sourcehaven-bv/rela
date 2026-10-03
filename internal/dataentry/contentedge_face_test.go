package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A content-scoped edge is served only with the face that owns it
// (BUG-ISJHML). POL-1's draft cites FEAT-DRAFT and its published face cites
// FEAT-PUB; the identity-scoped `implements` edge to FEAT-1 belongs to every
// face. facedApp's default world is `published`, so a bare id serves the
// published face of POL-1.
//
// Two readers see the same thing: one who may read every face, and one
// granted only `policy@published`. For the second, a draft edge on a
// published page is a disclosure of draft content, not only a mixed-face
// page.

type contentEdgeReader struct {
	name string
	read []string
	// draft reports whether this reader may read POL-1@draft.
	draft bool
}

var contentEdgeReaders = []contentEdgeReader{
	{"all faces", []string{"*", "policy@draft", "policy@published", "world:published"}, true},
	{"policy@published only", []string{"policy@published", "feature", "world:published"}, false},
}

// contentEdgeApp builds facedApp for reader, seeds one content edge per face
// plus an identity edge, and routes every request as that reader.
func contentEdgeApp(t *testing.T, reader contentEdgeReader) (*App, *acl.Declarative) {
	t.Helper()
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: reader.read}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	ctx := context.Background()
	for _, id := range []string{"FEAT-DRAFT", "FEAT-PUB"} {
		if err := app.store.CreateEntity(ctx, &entity.Entity{
			ID: id, Type: "feature", Properties: map[string]any{"title": id},
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	for _, e := range []struct {
		typ, to string
		tail    entity.Face
	}{
		{"cites", "FEAT-DRAFT", "draft"},
		{"cites", "FEAT-PUB", "published"},
		{"implements", "FEAT-1", ""},
	} {
		if _, err := app.store.CreateRelation(ctx, entity.RelationKey{From: "POL-1", FromFace: e.tail, Type: e.typ, To: e.to}, &store.RelationData{}); err != nil {
			t.Fatalf("seed POL-1 %s %s: %v", e.typ, e.to, err)
		}
	}
	return app, d
}

// routedBody GETs path through the router and returns the decoded object.
func routedBody(t *testing.T, app *App, path string) map[string]any {
	t.Helper()
	body, ok := routedJSON(t, app, path).(map[string]any)
	if !ok {
		t.Fatalf("GET %s: body is not an object", path)
	}
	return body
}

// routedJSON GETs path through the router and returns the decoded body.
func routedJSON(t *testing.T, app *App, path string) any {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
	}
	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET %s: decode: %v", path, err)
	}
	return body
}

// mentions reports whether v, re-encoded, names id anywhere.
func mentions(t *testing.T, v any, id string) bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(b), `"`+id+`"`)
}

func TestContentEdges_ViewSectionsServeOnlyTheOwningFace(t *testing.T) {
	for _, reader := range contentEdgeReaders {
		t.Run(reader.name, func(t *testing.T) {
			app, _ := contentEdgeApp(t, reader)

			for _, path := range []string{
				"/api/v1/_views/policy/POL-1@published",
				"/api/v1/_views/policy/POL-1", // the world resolves it to published
			} {
				sections := routedBody(t, app, path)["sections"]
				if mentions(t, sections, "FEAT-DRAFT") {
					t.Errorf("%s: sections carry the DRAFT face's content edge: %v", path, sections)
				}
				for _, id := range []string{"FEAT-PUB", "FEAT-1"} {
					if !mentions(t, sections, id) {
						t.Errorf("%s: sections dropped %s, an edge the published face owns", path, id)
					}
				}
			}

			if reader.draft {
				sections := routedBody(t, app, "/api/v1/_views/policy/POL-1@draft")["sections"]
				if mentions(t, sections, "FEAT-PUB") {
					t.Errorf("the draft page carries the PUBLISHED face's content edge: %v", sections)
				}
				for _, id := range []string{"FEAT-DRAFT", "FEAT-1"} {
					if !mentions(t, sections, id) {
						t.Errorf("the draft page dropped %s, an edge the draft owns", id)
					}
				}
			}

			// Incoming, on the faceless target in the published world: the
			// source is served at its published face, so only that face's
			// edge and the identity edge name it.
			for _, tc := range []struct {
				target string
				want   bool
			}{
				{"FEAT-DRAFT", false},
				{"FEAT-PUB", true},
				{"FEAT-1", true},
			} {
				sections := routedBody(t, app, "/api/v1/_views/feature/"+tc.target)["sections"]
				if got := mentions(t, sections, "POL-1"); got != tc.want {
					t.Errorf("%s page shows POL-1 as incoming neighbor = %v, want %v: %v",
						tc.target, got, tc.want, sections)
				}
			}
		})
	}
}

func TestContentEdges_WorldNeighborsServeOnlyTheOwningFace(t *testing.T) {
	for _, reader := range contentEdgeReaders {
		t.Run(reader.name, func(t *testing.T) {
			app, _ := contentEdgeApp(t, reader)

			// The single-entity GET and its include block.
			body := routedBody(t, app, "/api/v1/policys/POL-1@published?include=*")
			if mentions(t, body["relations"], "FEAT-DRAFT") || mentions(t, body["included"], "FEAT-DRAFT") {
				t.Errorf("the published face serves the draft's content edge: %v / %v",
					body["relations"], body["included"])
			}
			if !mentions(t, body["relations"], "FEAT-PUB") || !mentions(t, body["relations"], "FEAT-1") {
				t.Errorf("the published face lost an edge it owns: %v", body["relations"])
			}

			// Incoming neighbors on the target's page and include block.
			for _, tc := range []struct {
				target string
				want   bool
			}{
				{"FEAT-DRAFT", false},
				{"FEAT-PUB", true},
				{"FEAT-1", true},
			} {
				body := routedBody(t, app, "/api/v1/features/"+tc.target+"?include=*")
				if got := mentions(t, body["included"], "POL-1"); got != tc.want {
					t.Errorf("%s includes POL-1 = %v, want %v: %v", tc.target, got, tc.want, body["included"])
				}
			}

			// The relations routes, outgoing on the published face.
			for _, tc := range []struct{ path, absent string }{
				{"/api/v1/policys/POL-1@published/relations", "FEAT-DRAFT"},
				{"/api/v1/policys/POL-1@published/relations/cites", "FEAT-DRAFT"},
			} {
				if body := routedJSON(t, app, tc.path); mentions(t, body, tc.absent) {
					t.Errorf("GET %s names %s: %v", tc.path, tc.absent, body)
				}
			}
			// Incoming on the draft edge's target: an edge editor, so the
			// edge is served by ACL alone, named by its face, and read-only
			// for a role without relation writes.
			for _, path := range []string{
				"/api/v1/features/FEAT-DRAFT/relations",
				"/api/v1/features/FEAT-DRAFT/relations/cites?direction=incoming",
			} {
				body := routedJSON(t, app, path)
				if got := mentions(t, body, "POL-1"); got != reader.draft {
					t.Errorf("GET %s names POL-1 = %v, want %v: %v", path, got, reader.draft, body)
				}
				raw, _ := json.Marshal(body)
				if reader.draft && (!mentions(t, body, "draft") || !strings.Contains(string(raw), `"editable":false`)) {
					t.Errorf("GET %s: want POL-1 at face draft, not editable: %v", path, body)
				}
			}

			// List rows carry incoming edges keyed by the inverse name.
			rows := map[string]any{}
			for _, row := range routedBody(t, app, "/api/v1/features")["data"].([]any) {
				r := row.(map[string]any)
				rows[r["id"].(string)] = r["relations"]
			}
			for _, tc := range []struct {
				target string
				want   bool
			}{
				{"FEAT-DRAFT", false},
				{"FEAT-PUB", true},
				{"FEAT-1", true},
			} {
				if got := mentions(t, rows[tc.target], "POL-1"); got != tc.want {
					t.Errorf("list row %s names POL-1 = %v, want %v: %v", tc.target, got, tc.want, rows[tc.target])
				}
			}
		})
	}
}

// An incoming relation filter counts an edge only when the face the world
// serves for its source owns it, and only when the reader may read that
// face: otherwise the matched rows reveal what the draft cites.
func TestContentEdges_IncomingRelationFilterMatchesTheServedFace(t *testing.T) {
	for _, reader := range contentEdgeReaders {
		t.Run(reader.name, func(t *testing.T) {
			app, d := contentEdgeApp(t, reader)
			ctx := withWorld(gateCtxFor(principal.With(context.Background(),
				principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, d),
				worldHandle{name: "published", scope: policyPublishedScope()})
			pub, err := app.store.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: "published"})
			if err != nil {
				t.Fatal(err)
			}
			title := app.Meta().DisplayTitle(pub.ID, pub.Type, pub.Properties)
			var rows []*entity.Entity
			for _, id := range []string{"FEAT-DRAFT", "FEAT-PUB", "FEAT-1"} {
				rows = append(rows, &entity.Entity{ID: id, Type: "feature"})
			}
			matched, err := matchRelationFilterMany(ctx, app.Services(), app.visibleReader, rows,
				"cites", dataentryconfig.DirectionIncoming, title)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range rows {
				if matched[r.ID] {
					got = append(got, r.ID)
				}
			}
			if want := []string{"FEAT-PUB"}; !slices.Equal(got, want) {
				t.Errorf("incoming cites filter on %q matched %v, want %v", title, got, want)
			}

			// In a world that serves the draft, the draft's edge counts, but
			// only for a reader who may read the draft face.
			draftCtx := withWorld(ctx, worldHandle{name: "drafts", scope: store.NewWorldScope(
				map[string]store.TypeResolution{
					"policy": {Chain: []entity.Face{"draft"}, Fallback: store.FallbackExclude},
				})})
			matched, err = matchRelationFilterMany(draftCtx, app.Services(), app.visibleReader, rows,
				"cites", dataentryconfig.DirectionIncoming, title)
			if err != nil {
				t.Fatal(err)
			}
			got = got[:0]
			for _, r := range rows {
				if matched[r.ID] {
					got = append(got, r.ID)
				}
			}
			var want []string
			if reader.draft {
				want = []string{"FEAT-DRAFT"}
			}
			if !slices.Equal(got, want) {
				t.Errorf("draft world: incoming cites filter matched %v, want %v", got, want)
			}
		})
	}
}

// On a surface that reads neighbors at the zero coordinate, an incoming
// content edge is kept only when its source is served there: a faceless
// source, or the row itself for a self-edge.
func TestIncomingOwnedAtZero(t *testing.T) {
	app, _ := contentEdgeApp(t, contentEdgeReaders[0])
	row := &entity.Entity{ID: "POL-1", Type: "policy", Face: "published"}
	selfOwn := &entity.Relation{From: "POL-1", FromFace: "published", Type: "cites", To: "POL-1"}
	selfOther := &entity.Relation{From: "POL-1", FromFace: "draft", Type: "cites", To: "POL-1"}
	faced := &entity.Relation{From: "POL-2", FromFace: "published", Type: "cites", To: "POL-1"}
	faceless := &entity.Relation{From: "FEAT-1", Type: "cites", To: "POL-1"}
	identity := &entity.Relation{From: "POL-2", FromFace: "draft", Type: "implements", To: "POL-1"}

	got := incomingOwnedAtZero(app.Meta(), []*entity.Relation{selfOwn, selfOther, faced, faceless, identity}, row)
	if want := []*entity.Relation{selfOwn, faceless, identity}; !slices.Equal(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

// The surfaces that read edges by bare id: a list relation filter, a view
// table's relation column, and the export renderer.
func TestContentEdges_BareIdSurfacesServeOnlyTheOwningFace(t *testing.T) {
	app, d := contentEdgeApp(t, contentEdgeReaders[0])
	ctx := gateCtxFor(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, d)
	pub, err := app.store.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: "published"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("relation filter", func(t *testing.T) {
		for _, tc := range []struct {
			want    string
			matched bool
		}{
			{"FEAT-DRAFT", false},
			{"FEAT-PUB", true},
		} {
			got, err := matchRelationFilterMany(ctx, app.Services(), app.visibleReader, []*entity.Entity{pub},
				"cites", dataentryconfig.DirectionOutgoing, tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if got["POL-1"] != tc.matched {
				t.Errorf("filter[cites]=%s matched the published face = %v, want %v",
					tc.want, got["POL-1"], tc.matched)
			}
		}
	})

	t.Run("view table relation column", func(t *testing.T) {
		cols := []dataentryconfig.ListColumn{{Relation: "cites"}}
		targets, _ := app.views.relationColumnTargets(ctx, app.Services(), app.views.schema(),
			cols, []*entity.Entity{pub})
		if got := targets["POL-1"][0]; !slices.Equal(got, []string{"FEAT-PUB"}) {
			t.Errorf("published row's cites column = %v, want [FEAT-PUB]", got)
		}
	})

	t.Run("export", func(t *testing.T) {
		var names []string
		groups, err := app.export.entityRelationGroups(ctx, pub)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range groups {
			names = append(names, g.Neighbors...)
		}
		if slices.Contains(names, "FEAT-DRAFT") || !slices.Contains(names, "FEAT-PUB") {
			t.Errorf("export of the published face lists %v, want FEAT-PUB and not FEAT-DRAFT", names)
		}
	})
}
