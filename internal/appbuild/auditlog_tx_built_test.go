//go:build postgres || sqlite

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestAuditFailureRollsTheWriteBack pins DEC-U78WD7Y: on sqlite and postgres the
// record of an entity write is appended in the write's transaction, so an
// append that fails takes the write with it.
func TestAuditFailureRollsTheWriteBack(t *testing.T) {
	root := writeMinimalProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"),
		[]byte(metamodelYAML+"relations:\n  refs:\n    from: [doc]\n    to: [doc]\n"), 0o644))
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	log := svc.AuditLog()
	require.NotNil(t, log)

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	kept := entity.New("", "doc")
	kept.SetString("title", "before")
	created, err := svc.EntityManager().CreateEntity(ctx, kept, entity.CreateOptions{})
	require.NoError(t, err)
	v, err := log.Verify(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.Rows, "the create's record committed with it, not later from a queue")

	refuseAuditAppends(t, svc)

	refused := entity.New("", "doc")
	refused.SetString("title", "refused")
	_, err = svc.EntityManager().CreateEntity(ctx, refused, entity.CreateOptions{})
	require.ErrorContains(t, err, "audit refused")
	var docs int
	for _, lErr := range svc.Store().ListEntities(ctx, store.EntityQuery{Type: "doc", Faces: store.AllFaces()}) {
		require.NoError(t, lErr)
		docs++
	}
	require.Equal(t, 1, docs, "the create whose record failed was rolled back")

	_, err = svc.EntityManager().PatchEntity(ctx, created.Entity.ID, entity.Patch{
		Properties: map[string]any{"title": "after"},
	})
	require.ErrorContains(t, err, "audit refused")
	got, err := svc.Store().GetEntity(ctx, created.Entity.Ref())
	require.NoError(t, err)
	require.Equal(t, "before", got.GetString("title"), "the update whose record failed was rolled back")

	_, err = svc.EntityManager().CreateRelation(ctx, entity.RelationKey{
		From: created.Entity.ID, Type: "refs", To: created.Entity.ID,
	}, entity.RelationOptions{})
	require.ErrorContains(t, err, "audit refused")
	var rels int
	for _, lErr := range svc.Store().ListRelations(ctx, store.RelationQuery{Type: "refs"}) {
		require.NoError(t, lErr)
		rels++
	}
	require.Zero(t, rels, "the relation create whose record failed was rolled back")

	_, err = svc.EntityManager().DeleteEntity(ctx, created.Entity.ID, false)
	require.ErrorContains(t, err, "audit refused")
	_, err = svc.Store().GetEntity(ctx, created.Entity.Ref())
	require.NoError(t, err, "the delete whose record failed was rolled back")
}
