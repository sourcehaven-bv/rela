package dataentry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// TestConflictedRow_RoleRelationsReader pins BUG-HTSW37Q through the real
// policy on the file store. Alice reads tickets only through a role an
// owned-by edge confers. One of her tickets has a file with conflict markers.
// The list answers 200 without that row, and a GET of it answers the uniform
// 404, the same answer as an id that does not exist.
func TestConflictedRow_RoleRelationsReader(t *testing.T) {
	app := newTestAppV1(t)
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	fss, err := fsstore.New(fsstore.Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations", AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}, "feature": {Plural: "features"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = fss.Close() })
	paths := &project.Context{Root: "/"}
	rebindApp(app, fs, paths, appbuildtest.New(app.Meta(), appbuildtest.WithStore(fss), appbuildtest.WithFS(fs, paths)))
	for _, id := range []string{"TKT-001", "TKT-002"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": "t"}})
		_, err := app.store.CreateRelation(t.Context(), entity.RelationKey{From: "alice", Type: "owned-by", To: id}, nil)
		require.NoError(t, err)
	}
	conflicted := "---\nid: TKT-002\ntype: ticket\n<<<<<<< HEAD\ntitle: ours\n=======\ntitle: theirs\n>>>>>>> theirs\n---\n"
	require.NoError(t, fs.WriteFileExternal("/entities/tickets/TKT-002.md", []byte(conflicted), 0o644))

	d := mustNewACL(t, &acl.Policy{
		Roles:         map[string]acl.RoleDef{"member": {}, "owner": {Read: []string{"ticket"}}},
		Assignments:   map[string]string{"alice": "member"},
		RoleRelations: map[string]acl.RoleRelationDef{"owned-by": {Confers: "owner"}},
	}, app.store)
	setTestACL(t, app, d)
	alice := principalCtx("alice")

	resp, rec := listEntitiesAs(alice, t, app, d, "ticket", "tickets", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	require.Equal(t, []string{"TKT-001"}, ids)

	hidden := getEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-002", "")
	missing := getEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-404", "")
	require.Equal(t, http.StatusNotFound, hidden.Code, hidden.Body.String())
	require.Equal(t, missing.Code, hidden.Code)
	require.JSONEq(t, strings.ReplaceAll(missing.Body.String(), "TKT-404", "TKT-002"), hidden.Body.String())
}
