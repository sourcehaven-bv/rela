package sqlitestore_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestMentions_UnindexedRowIsIndexedOnRead covers a row from before the v17
// rung, whose mention_targets is NULL: the first backlink read indexes it.
func TestMentions_UnindexedRowIsIndexedOnRead(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "m.db"))
	ctx := t.Context()

	e := entity.New("FEAT-001", "feature")
	e.Content = "see `REQ-001`"
	require.NoError(t, s.CreateEntity(ctx, e))
	_, err := db.DB().ExecContext(ctx, `UPDATE entities SET mention_targets = NULL`)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.DB().QueryRowContext(ctx,
		`SELECT count(*) FROM entity_mention_targets`).Scan(&n))
	require.Zero(t, n, "clearing the column clears the fan-out")

	refs, err := s.MentionSources(ctx, "REQ-001", entity.Ref{}, 10)
	require.NoError(t, err)
	require.Equal(t, []entity.Ref{{ID: "FEAT-001"}}, refs)
}

// TestMentions_SurvivesVacuumRenumbering pins that the backlink index names a
// source by (id, face), not by the entities rowid, which VACUUM may renumber
// (RR-44YDTU). The rowids are moved by hand, as in
// TestSearch_SurvivesVacuumRenumbering.
func TestMentions_SurvivesVacuumRenumbering(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "m.db"))
	ctx := t.Context()
	for i := range 6 {
		e := entity.New(fmt.Sprintf("N-%d", i), "note")
		e.Content = "see `REQ-001`"
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	for i := range 3 {
		_, err := s.DeleteFamily(ctx, fmt.Sprintf("N-%d", i), true)
		require.NoError(t, err)
	}
	_, err := db.DB().ExecContext(ctx, `UPDATE entities SET rowid = rowid - 3`)
	require.NoError(t, err)

	sources := func() []entity.Ref {
		t.Helper()
		refs, readErr := s.MentionSources(ctx, "REQ-001", entity.Ref{}, 10)
		require.NoError(t, readErr)
		return refs
	}
	require.Equal(t, []entity.Ref{{ID: "N-3"}, {ID: "N-4"}, {ID: "N-5"}}, sources())

	e, err := s.GetEntity(ctx, entity.Ref{ID: "N-4"})
	require.NoError(t, err)
	e.Content = "no mention"
	require.NoError(t, s.UpdateEntity(ctx, e))
	_, err = s.RenameFamily(ctx, "N-5", "M-5")
	require.NoError(t, err)
	require.Equal(t, []entity.Ref{{ID: "M-5"}, {ID: "N-3"}}, sources())
}
