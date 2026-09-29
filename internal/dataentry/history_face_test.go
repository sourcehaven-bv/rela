package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// History, restore and purge act on the addressed face's lineage
// (BUG-4SYAA6). A lineage is keyed by (id, face), so every surface takes an
// `ID@face` address and reads or changes that face only.

// facedSnapshot is a version-1 snapshot of ticket TKT-1 captured at face.
func facedSnapshot(face entity.Face, title string) store.VersionSnapshot {
	s := snapshot("ticket", "body of "+title, map[string]any{"title": title})
	s.Face = face
	return s
}

// facedHistory is the canned history of TKT-1: the draft face's version 1
// says "Travel", the published face's says "Published v1".
func facedHistory() historyStore {
	return historyStore{versions: map[string][]store.VersionSnapshot{
		"TKT-1@draft":     {facedSnapshot("draft", "Travel")},
		"TKT-1@published": {facedSnapshot("published", "Published v1")},
	}}
}

// facedHistoryApp is TKT-1 at `draft` and `published` on a ticket type that
// declares faces, with facedHistory installed and the entitymanager wired
// with policy, so a restore is authorized by the real write path.
func facedHistoryApp(t *testing.T, policy func(st store.Store) *acl.Declarative) (*App, *acl.Declarative) {
	t.Helper()
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"ticket": {
			Label:         "Ticket",
			IDPrefix:      "TKT-",
			Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
			PropertyOrder: []string{"title"},
			Faces:         map[string]metamodel.FaceDef{"draft": {}, "published": {}, "review": {}},
		},
	}}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	st := bootstrap.Store()
	for face, title := range map[entity.Face]string{"draft": "Travel v2", "published": "Published v2"} {
		if err := st.CreateEntity(context.Background(), &entity.Entity{
			ID: "TKT-1", Type: "ticket", Face: face, Properties: map[string]any{"title": title},
		}); err != nil {
			t.Fatalf("seed %s: %v", face, err)
		}
	}
	d := policy(st)
	svc := appbuildtest.New(meta,
		appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d),
	)
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	app.versions = facedHistory()
	return app, d
}

// historyAs runs one history request as the principal on ctx.
func historyAs(
	ctx context.Context, t *testing.T, app *App, d *acl.Declarative, method, path string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/_history/ticket/"+path, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)
	return rec
}

// titleAt returns the stored title of TKT-1 at face, or "" when the face
// has no row.
func titleAt(t *testing.T, app *App, face entity.Face) string {
	t.Helper()
	e, err := app.store.GetEntityState(context.Background(), "TKT-1", face)
	if err != nil {
		return ""
	}
	return e.GetString("title")
}

func TestFacedHistory_TimelineIsTheAddressedFace(t *testing.T) {
	app, d := facedHistoryApp(t, faceEditors(t, "bob"))
	bob := principalCtx("bob")

	for _, face := range []entity.Face{"draft", "published"} {
		rec := historyAs(bob, t, app, d, http.MethodGet, "TKT-1@"+face.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("%s timeline = %d (%s)", face, rec.Code, rec.Body)
		}
		var body struct {
			Face     string `json:"face"`
			Versions []any  `json:"versions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Face != face.String() || len(body.Versions) != 1 {
			t.Errorf("%s timeline = face %q with %d versions, want its own single version",
				face, body.Face, len(body.Versions))
		}
	}
}

// TestFacedHistory_BareIDIsNotFound: a bare id names no face of a faced
// type in the default world, so its history is the uniform 404, as its GET
// is. Serving the zero face's empty lineage would look like "no versions".
func TestFacedHistory_BareIDIsNotFound(t *testing.T) {
	app, d := facedHistoryApp(t, faceEditors(t, "bob"))
	for _, path := range []string{"TKT-1", "TKT-1/1"} {
		if rec := historyAs(principalCtx("bob"), t, app, d, http.MethodGet, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (%s)", path, rec.Code, rec.Body)
		}
	}
	if rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1/1/restore"); rec.Code != http.StatusNotFound {
		t.Errorf("restore of a bare faced id = %d, want 404 (%s)", rec.Code, rec.Body)
	}
}

func TestFacedHistory_RestoreChangesTheAddressedFaceOnly(t *testing.T) {
	app, d := facedHistoryApp(t, faceEditors(t, "bob"))

	rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d (%s)", rec.Code, rec.Body)
	}
	if got := titleAt(t, app, "draft"); got != "Travel" {
		t.Errorf("draft title = %q, want the restored %q", got, "Travel")
	}
	if got := titleAt(t, app, "published"); got != "Published v2" {
		t.Errorf("published title = %q; a restore of draft must not touch it", got)
	}
}

// TestFacedHistory_RestoreRecreatesADeletedFace: the snapshot records its
// face (TKT-7R0ABK), so a deleted face comes back at that face.
func TestFacedHistory_RestoreRecreatesADeletedFace(t *testing.T) {
	app, d := facedHistoryApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: allTicketFaces, Update: allTicketFaces, Create: allTicketFaces,
				Permissions: []string{string(acl.PermHistoryRead)},
			}},
			Assignments: map[string]string{"bob": "editor"},
		}, st)
	})
	if _, err := app.store.DeleteEntityState(context.Background(), "TKT-1", "draft"); err != nil {
		t.Fatalf("delete draft: %v", err)
	}

	rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore of a deleted face = %d (%s)", rec.Code, rec.Body)
	}
	if got := titleAt(t, app, "draft"); got != "Travel" {
		t.Errorf("draft title = %q, want the face recreated from its snapshot", got)
	}
	if got := titleAt(t, app, "published"); got != "Published v2" {
		t.Errorf("published title = %q; recreating draft must not touch it", got)
	}
}

// TestFacedHistory_RestoreIsAuthorizedOnTypeAtFace: restore is an update of
// `ticket@face`. A face the principal may read but not update is a 403; a
// face it may not read is the uniform 404.
func TestFacedHistory_RestoreIsAuthorizedOnTypeAtFace(t *testing.T) {
	app, d := facedHistoryApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"published-editor": {Read: allTicketFaces, Update: []string{"ticket@published"}},
				"published-reader": {Read: []string{"ticket@published"}, Update: []string{"ticket@published"}},
			},
			Assignments: map[string]string{"alice": "published-editor", "carol": "published-reader"},
		}, st)
	})
	for _, tc := range []struct {
		name string
		user string
		path string
		want int
	}{
		{"update-denied face", "alice", "TKT-1@draft/1/restore", http.StatusForbidden},
		{"hidden face", "carol", "TKT-1@draft/1/restore", http.StatusNotFound},
		{"hidden face timeline", "carol", "TKT-1@draft", http.StatusNotFound},
		{"hidden face snapshot", "carol", "TKT-1@draft/1", http.StatusNotFound},
		{"update-granted face", "alice", "TKT-1@published/1/restore", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodGet
			if strings.HasSuffix(tc.path, "/restore") {
				method = http.MethodPost
			}
			if rec := historyAs(principalCtx(tc.user), t, app, d, method, tc.path); rec.Code != tc.want {
				t.Errorf("= %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
	if got := titleAt(t, app, "draft"); got != "Travel v2" {
		t.Errorf("draft changed under denied restores: %q", got)
	}
	if got := titleAt(t, app, "published"); got != "Published v1" {
		t.Errorf("published title = %q, want the restored value", got)
	}
}
