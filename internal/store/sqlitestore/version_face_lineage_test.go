package sqlitestore_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestConversionFaceLineage(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	draft := entity.Face("draft")
	approved := entity.Face("approved")

	before := store.VersionInput{
		EntityID: "NOTE-1", Face: draft, Op: store.VersionOpCreate, Type: "note",
		Content: "draft content", Properties: map[string]any{"title": "note"},
		SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
	}
	require.NoError(t, s.VersionStore().WriteVersion(ctx, before))

	converted := before
	converted.EntityID = "TASK-001"
	converted.Face = approved
	converted.Op = store.VersionOpRename
	converted.PrevID = "NOTE-1"
	converted.PrevFace = draft
	converted.HasPrevFace = true
	converted.Type = "task"
	require.NoError(t, s.VersionStore().WriteVersion(ctx, converted))

	after := converted
	after.Op = store.VersionOpUpdate
	after.PrevID = ""
	after.PrevFace = ""
	after.Content = "approved content"
	require.NoError(t, s.VersionStore().WriteVersion(ctx, after))

	ref := entity.Ref{ID: "TASK-001", Face: approved}
	metas, err := s.VersionStore().ListVersions(ctx, ref)
	require.NoError(t, err)
	require.Len(t, metas, 3)
	require.Equal(t, draft, metas[0].Face)
	require.Equal(t, "NOTE-1", metas[1].PrevID)
	require.Equal(t, draft, metas[1].PrevFace)

	snapshot, err := s.VersionStore().GetVersion(ctx, ref, 1)
	require.NoError(t, err)
	require.Equal(t, draft, snapshot.Face)
	require.Equal(t, "draft content", snapshot.Content)

	old, err := s.VersionStore().ListVersions(ctx, entity.Ref{ID: "NOTE-1", Face: draft})
	require.NoError(t, err)
	require.Empty(t, old)
}
