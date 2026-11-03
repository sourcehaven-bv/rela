//go:build !postgres && !memorybackend

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// schemaWithRiskRegister adds a type whose directory is not "<type>s", so a
// write that guessed the directory would land where the next open does not
// look.
const schemaWithRiskRegister = `
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title:
        type: string
        required: true
  risk:
    label: Risk
    plural: riskregister
    id_type: sequential
    id_prefix: RSK-
    properties:
      title:
        type: string
        required: true
relations: {}
`

// TestMCPServices_ReloadedTypeIsStoredInItsDirectory pins that a schema
// reload reaches the markdown store (TKT-C2YVX1): a type the reload adds is
// written to its declared directory and survives a reopen, and files already
// in that directory join the index.
func TestMCPServices_ReloadedTypeIsStoredInItsDirectory(t *testing.T) {
	root := seedProject(t)
	// RSK-9 also keeps the project on markdown under the sqlite build,
	// which opens a database for a project without entity files.
	dir := filepath.Join(root, "entities", "riskregister")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "RSK-9.md"),
		[]byte("---\nid: RSK-9\ntype: risk\ntitle: Already on disk\n---\n"), 0o644))

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	ctx := context.Background()

	writeSchema(t, root, schemaWithRiskRegister)
	deps, err := reloadDeps(svc)
	require.NoError(t, err)

	_, err = svc.current().Store().GetEntity(ctx, entity.Ref{ID: "RSK-9"})
	require.NoError(t, err, "a file in the new type's directory must join the index on reload")

	res, err := deps.EntityManager.CreateEntity(ctx,
		&entity.Entity{Type: "risk", Properties: map[string]any{"title": "Supplier outage"}},
		entity.CreateOptions{})
	require.NoError(t, err)
	id := res.Entity.ID
	assert.FileExists(t, filepath.Join(dir, id+".md"), "the new type must be written to its declared directory")
	require.NoError(t, svc.Close())

	reopened, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := reopened.current().Store().GetEntity(ctx, entity.Ref{ID: id})
	require.NoError(t, err, "an entity of the reloaded type must survive a reopen")
	assert.Equal(t, "risk", got.Type)
}

// TestMCPServices_RejectedReloadLeavesStoreSchema pins that a reload the
// server rejects does not touch the markdown store: the store is refreshed
// only after publish succeeds, so it keeps refusing a type the services in
// force do not declare, and the new type's files stay out of the index.
func TestMCPServices_RejectedReloadLeavesStoreSchema(t *testing.T) {
	root := seedProject(t)
	dir := filepath.Join(root, "entities", "riskregister")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "RSK-9.md"),
		[]byte("---\nid: RSK-9\ntype: risk\ntitle: Already on disk\n---\n"), 0o644))
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	writeSchema(t, root, schemaWithRiskRegister)
	err = svc.reload(func(relamcp.Deps) error { return errors.New("rejected") })
	require.ErrorContains(t, err, "rejected")

	st := svc.current().Store()
	ctx := context.Background()
	_, err = st.GetEntity(ctx, entity.Ref{ID: "RSK-9"})
	require.ErrorIs(t, err, store.ErrNotFound, "the rejected type's files must not join the index")
	err = st.CreateEntity(ctx, &entity.Entity{ID: "RSK-1", Type: "risk", Properties: map[string]any{"title": "x"}})
	require.ErrorContains(t, err, "unknown entity type")
	assert.NoFileExists(t, filepath.Join(dir, "RSK-1.md"))
}

// TestMCPServices_ReloadWithMovedPluralKeepsServing pins that a reload that
// moves a type with entities to another directory still applies the rest of
// the schema, in the services and in the store. The store keeps that type
// in its old directory and logs which directory to rename, so the type's
// entities stay readable.
func TestMCPServices_ReloadWithMovedPluralKeepsServing(t *testing.T) {
	root := seedProject(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "entities", "items"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "entities", "items", "ITEM-1.md"),
		[]byte("---\nid: ITEM-1\ntype: item\ntitle: Seed\n---\n"), 0o644))
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	writeSchema(t, root, strings.Replace(schemaWithRiskRegister,
		"    label: Item\n", "    label: Item\n    plural: things\n", 1))
	require.NoError(t, svc.reload(nil))

	_, ok := svc.current().Meta().Entities["risk"]
	assert.True(t, ok, "the rest of the schema applies")
	st := svc.current().Store()
	ctx := context.Background()
	_, err = st.GetEntity(ctx, entity.Ref{ID: "ITEM-1"})
	require.NoError(t, err, "the moved type's entities stay readable from the old directory")
	require.NoError(t, st.CreateEntity(ctx,
		&entity.Entity{ID: "RSK-1", Type: "risk", Properties: map[string]any{"title": "x"}}),
		"the store learns the added type despite the refused plural")
	assert.FileExists(t, filepath.Join(root, "entities", "riskregister", "RSK-1.md"))
}
