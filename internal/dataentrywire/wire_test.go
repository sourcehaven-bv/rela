package dataentrywire_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// A list's condition: applies once the app is wired. Without the view
// condition compiler it would not, and the list would show every row.
func TestServices_ListConditionApplies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", `version: "1.0"
entities:
  doc:
    label: Document
    id_prefix: DOC
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)
	writeFile(t, root, "data-entry.yaml", `app:
  name: Test
lists:
  first_only:
    entity_type: doc
    condition: "entity.title == 'First'"
`)
	writeFile(t, root, "entities/docs/DOC-1.md", "---\nid: DOC-1\ntype: doc\ntitle: First\n---\n")
	writeFile(t, root, "entities/docs/DOC-2.md", "---\nid: DOC-2\ntype: doc\ntitle: Second\n---\n")

	fsys := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.At(root, fsys)
	require.NoError(t, err)
	sink, err := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
	require.NoError(t, err)
	svc, err := appbuild.New(appbuild.Config{
		FS: fsys, Paths: paths, ScriptEngine: script.NewEngine(), Audit: sink,
	}, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	resolver, err := dataentry.ResolverFromServices(svc)
	require.NoError(t, err)
	app, err := dataentry.NewApp(
		fsys, paths, svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		resolver, svc.Audit(), svc.State(), dataentry.UngatedCommandAuthorizer(), appbuild.CompiledWorlds(svc),
	)
	require.NoError(t, err)
	require.NoError(t, dataentrywire.Services(app, svc))
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: principal.SystemUser(), Tool: principal.ToolDesktop}
	})

	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/docs?list_id=first_only", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data []struct{ ID string } `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Data, 1, rec.Body.String())
	assert.Equal(t, "DOC-1", resp.Data[0].ID)
}
