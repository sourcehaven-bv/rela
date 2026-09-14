package sqlitestore_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// seedRelationEndpoints creates the two entities a relation needs.
func seedRelationEndpoints(t *testing.T, s store.Store, ids ...string) {
	t.Helper()
	for _, id := range ids {
		e := entity.New(id, "feature")
		e.SetString("title", id)
		require.NoError(t, s.CreateEntity(t.Context(), e))
	}
}

// relSeqNext reads the lineage allocator's counter straight from the file.
func relSeqNext(t *testing.T, path string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var next int64
	require.NoError(t, db.QueryRow(`SELECT next FROM rel_record_seq WHERE id = 1`).Scan(&next))
	return next
}

// TestDuplicateRelationDoesNotBurnALineageID pins the allocation ORDER.
//
// ErrConflict on a duplicate triple is an ordinary outcome — automations retry
// on it — so if allocation ran before the insert, every retry would consume an
// id that no row ever carries. Outside a Tx the counter bump autocommits, so
// the failed insert cannot give it back.
//
// The counter is checked rather than the ids themselves because that is where
// the leak shows: the surviving rows look correct either way.
func TestDuplicateRelationDoesNotBurnALineageID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dup.db")
	s := openAt(t, path)
	seedRelationEndpoints(t, s, "FEAT-1", "FEAT-2")

	_, err := s.CreateRelation(t.Context(), "FEAT-1", "rel", "FEAT-2", &store.RelationData{})
	require.NoError(t, err)
	afterFirst := relSeqNext(t, path)

	for range 3 {
		_, dupErr := s.CreateRelation(t.Context(), "FEAT-1", "rel", "FEAT-2", &store.RelationData{})
		require.ErrorIs(t, dupErr, store.ErrConflict)
	}

	require.Equal(t, afterFirst, relSeqNext(t, path),
		"a rejected duplicate consumed a lineage id; the allocation runs before "+
			"the insert instead of after it")
}

// TestRelationsGetDistinctLineageIDs is the property the counter exists for:
// two relations must never share a lineage, or their histories merge.
func TestRelationsGetDistinctLineageIDs(t *testing.T) {
	s := open(t)
	seedRelationEndpoints(t, s, "FEAT-1", "FEAT-2", "FEAT-3")

	// On the VERSION service, not the store — the same place pgstore puts it,
	// which is what lets storetest find both with one lookup.
	recordIDer, ok := s.VersionStore().(interface {
		RelationRecordID(ctx context.Context, from, relType, to string) (int64, error)
	})
	require.True(t, ok, "sqlitestore's version service must expose RelationRecordID")

	seen := map[int64]bool{}
	for _, to := range []string{"FEAT-2", "FEAT-3"} {
		_, err := s.CreateRelation(t.Context(), "FEAT-1", "rel", to, &store.RelationData{})
		require.NoError(t, err)

		id, err := recordIDer.RelationRecordID(t.Context(), "FEAT-1", "rel", to)
		require.NoError(t, err)
		require.NotZero(t, id, "a relation left at the 0 sentinel shares a lineage with every other")
		require.Falsef(t, seen[id], "lineage id %d assigned twice", id)
		seen[id] = true
	}
}
