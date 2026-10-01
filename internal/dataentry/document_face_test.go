package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// facedDocumentApp is facedApp with documents whose Lua script prints the
// entry id it was handed and the title of the entity it reads through
// rela.get_entity(rela.document.entry_id), the way an operator's report reads
// "the entity". The test observes which face the render saw.
//
// Roles: alice holds `policy@published` only; bob reads every policy face;
// carol reads features only.
func facedDocumentApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"published-reader": {Read: []string{"policy@published", "feature", "world:published"}},
				"editor":           {Read: []string{"policy", "feature"}},
				"feature-reader":   {Read: []string{"feature"}},
			},
			Assignments: map[string]string{"alice": "published-reader", "bob": "editor", "carol": "feature-reader"},
		}, st)
	})

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	const body = `print("ENTRY: " .. rela.document.entry_id)
local e = rela.get_entity(rela.document.entry_id)
if e then print("TITLE: " .. e.properties.title) else print("NO ENTITY") end
`
	if err := os.WriteFile(filepath.Join(root, "scripts", "docs", "report.lua"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := func() lua.WriteDeps {
		wd := app.luaWriteDeps()
		wd.ProjectRoot = root
		return wd
	}
	app.documents = newDocumentService(app.store, app.kv, root, script.NewEngine(), deps, nil)

	s := app.State()
	meta := *s.Meta
	meta.Transforms = map[string]metamodel.TransformDef{
		"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
	}
	cfg := *s.Cfg
	cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"policy_report":  {EntityType: "policy", Script: "docs/report.lua"},
		"policy_cmd":     {EntityType: "policy", Command: []string{"cat", "{in}"}},
		"feature_report": {EntityType: "feature", Script: "docs/report.lua"},
	}
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: &meta,
		StyleMap: s.StyleMap, StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	return app, d
}

func getDocumentAs(t *testing.T, app *App, d *acl.Declarative, user, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_documents/"+path, http.NoBody).
		WithContext(gateCtxFor(principalCtx(user), t, d))
	app.handleV1Documents(rec, req)
	return rec
}

// assertUniform404 fails unless denied is a 404 with exactly the problem
// shape of missing, the answer for an entity that does not exist.
func assertUniform404(t *testing.T, denied, missing *httptest.ResponseRecorder) {
	t.Helper()
	if denied.Code != http.StatusNotFound {
		t.Fatalf("denied = %d, want 404 (%s)", denied.Code, denied.Body)
	}
	if missing.Code != http.StatusNotFound {
		t.Fatalf("control: missing = %d, want 404 (%s)", missing.Code, missing.Body)
	}
	if got, want := problemShape(t, denied.Body.Bytes()), problemShape(t, missing.Body.Bytes()); got != want {
		t.Errorf("a denial is distinguishable from a missing entity:\n denied=%s\n missing=%s", got, want)
	}
}

// TestAnchoredDocument_FaceGate pins BUG-8J3LSB: the document route applies
// the face gate after the row gate, so a `policy@published` reader cannot
// render a document over the draft, and a denied face answers exactly what
// a missing entity answers.
func TestAnchoredDocument_FaceGate(t *testing.T) {
	app, d := facedDocumentApp(t)

	t.Run("denied face is the uniform 404", func(t *testing.T) {
		denied := getDocumentAs(t, app, d, "alice", "policy_report/POL-1@draft")
		if strings.Contains(denied.Body.String(), "DRAFT TEXT") {
			t.Fatalf("a policy@published reader rendered the draft: %s", denied.Body)
		}
		assertUniform404(t, denied, getDocumentAs(t, app, d, "alice", "policy_report/POL-9@draft"))
	})

	for _, tc := range []struct {
		name, user, doc, addr string
		want                  []string
		notWant               string
	}{
		{"granted face renders its own content", "alice", "policy_report", "POL-1@published",
			[]string{"ENTRY: POL-1@published", "TITLE: PUBLISHED TEXT"}, "DRAFT TEXT"},
		{"all-faces reader renders the draft", "bob", "policy_report", "POL-1@draft",
			[]string{"ENTRY: POL-1@draft", "TITLE: DRAFT TEXT"}, "PUBLISHED TEXT"},
		{"all-faces reader renders the published face", "bob", "policy_report", "POL-1@published",
			[]string{"ENTRY: POL-1@published", "TITLE: PUBLISHED TEXT"}, "DRAFT TEXT"},
		{"faceless type keeps a bare entry id", "carol", "feature_report", "FEAT-1",
			[]string{`ENTRY: FEAT-1\nTITLE: f`}, "@"},
		{"command render reads the addressed face", "bob", "policy_cmd", "POL-1@draft",
			[]string{"DRAFT TEXT"}, "PUBLISHED TEXT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.doc == "policy_cmd" {
				requireCmdexecSandbox(t)
			}
			rec := getDocumentAs(t, app, d, tc.user, tc.doc+"/"+tc.addr)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s/%s as %s = %d, want 200 (%s)", tc.doc, tc.addr, tc.user, rec.Code, rec.Body)
			}
			for _, w := range tc.want {
				if !strings.Contains(rec.Body.String(), w) {
					t.Errorf("want %q in %s", w, rec.Body)
				}
			}
			if strings.Contains(rec.Body.String(), tc.notWant) {
				t.Errorf("rendered the wrong face: %q in %s", tc.notWant, rec.Body)
			}
		})
	}

	// The type-mismatch 400 names the entity's real type, so it may only be
	// answered to a principal who can read that row.
	t.Run("type mismatch on an unreadable row is the uniform 404", func(t *testing.T) {
		assertUniform404(t,
			getDocumentAs(t, app, d, "carol", "feature_report/POL-1@published"),
			getDocumentAs(t, app, d, "carol", "feature_report/FEAT-9"))
	})

	t.Run("type mismatch on a readable row stays a 400", func(t *testing.T) {
		rec := getDocumentAs(t, app, d, "bob", "feature_report/POL-1@published")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("= %d, want 400 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("bare id of a faced type with no world is a 404", func(t *testing.T) {
		rec := getDocumentAs(t, app, d, "bob", "policy_report/POL-1")
		if rec.Code != http.StatusNotFound {
			t.Errorf("bare id = %d, want 404 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("render cache does not serve a denied face", func(t *testing.T) {
		requireCmdexecSandbox(t)
		if rec := getDocumentAs(t, app, d, "bob", "policy_cmd/POL-1@draft"); rec.Code != http.StatusOK {
			t.Fatalf("control: bob = %d (%s)", rec.Code, rec.Body)
		}
		assertUniform404(t,
			getDocumentAs(t, app, d, "alice", "policy_cmd/POL-1@draft"),
			getDocumentAs(t, app, d, "alice", "policy_cmd/POL-9@draft"))
	})
}

// TestAnchoredDocument_BareIDRouted routes requests through the real router.
// Every route binds the default world, so a bare id resolves to the face that
// world serves (published here). The addressed face renders too.
func TestAnchoredDocument_BareIDRouted(t *testing.T) {
	app, d := facedDocumentApp(t)
	app.acl = d
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_documents/"+path, http.NoBody))
		return rec
	}
	if rec := get("policy_report/POL-1"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "TITLE: PUBLISHED TEXT") {

		t.Errorf("bare id = %d, want 200 with the published title (%s)", rec.Code, rec.Body)
	}
	if rec := get("policy_report/POL-9"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown bare id = %d, want 404", rec.Code)
	}
	rec := get("policy_report/POL-1@published")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "TITLE: PUBLISHED TEXT") {
		t.Errorf("addressed face = %d, want 200 with the published title (%s)", rec.Code, rec.Body)
	}
	assertUniform404(t, get("policy_report/POL-1@draft"), get("policy_report/POL-9@draft"))
}

// TestAnchoredDocumentExport_FaceGate: the `_export` form of an anchored
// document resolves through the same gate as the render.
func TestAnchoredDocumentExport_FaceGate(t *testing.T) {
	requireCmdexecSandbox(t)
	app, d := facedDocumentApp(t)

	assertUniform404(t,
		getDocumentAs(t, app, d, "alice", "policy_report/POL-1@draft/_export?transform=copy"),
		getDocumentAs(t, app, d, "alice", "policy_report/POL-9@draft/_export?transform=copy"))

	rec := getDocumentAs(t, app, d, "bob", "policy_report/POL-1@draft/_export?transform=copy")
	if rec.Code != http.StatusOK {
		t.Fatalf("bob export = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "TITLE: DRAFT TEXT") {
		t.Errorf("export rendered the wrong face: %s", rec.Body)
	}
}

// requireCmdexecSandbox skips a command-render case on a host where the
// confined runner refuses to execute (no sandbox mechanism available).
func requireCmdexecSandbox(t *testing.T) {
	t.Helper()
	runner, err := cmdexec.New(10*time.Second, 1024)
	if err == nil {
		_, _, err = runner.Run(context.Background(), []string{"true"}, nil, true)
	}
	if err != nil {
		t.Skipf("command execution unavailable on this host: %v", err)
	}
}
