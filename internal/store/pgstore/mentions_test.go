package pgstore_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

func mentionIDs(t *testing.T, s *pgstore.Store, target string) []string {
	t.Helper()
	refs, err := s.MentionSources(t.Context(), target, entity.Ref{}, 100)
	require.NoError(t, err)
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.ID)
	}
	return out
}

// TestMentions_OlderWriterHealed covers a body written by a binary that
// predates the mention columns: an INSERT leaves them NULL, and an UPDATE of
// content leaves the hash untouched, which the trigger turns into NULL. Until
// the reindex pass runs the row is not a backlink; after it, it is.
func TestMentions_OlderWriterHealed(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	_, err = pool.Exec(ctx, `INSERT INTO entities (id, face, type, properties, content, search_text)
		VALUES ('FEAT-001', '', 'feature', '{}', 'see `+"`REQ-001`"+`', '')`)
	require.NoError(t, err)
	require.Empty(t, mentionIDs(t, s, "REQ-001"), "an unindexed row is not listed")

	require.NoError(t, pgstore.ReindexMentionsForTest(ctx, s))
	require.Equal(t, []string{"FEAT-001"}, mentionIDs(t, s, "REQ-001"))

	// An older binary edits the body without touching the mention columns.
	_, err = pool.Exec(ctx, `UPDATE entities SET content = 'see `+"`REQ-002`"+`' WHERE id = 'FEAT-001'`)
	require.NoError(t, err)
	require.Empty(t, mentionIDs(t, s, "REQ-001"), "the trigger clears stale targets")

	require.NoError(t, pgstore.ReindexMentionsForTest(ctx, s))
	require.Empty(t, mentionIDs(t, s, "REQ-001"))
	require.Equal(t, []string{"FEAT-001"}, mentionIDs(t, s, "REQ-002"))

	// A current writer's update keeps its own targets: the hash changes with
	// the content, so the trigger leaves the row alone.
	e := entity.New("FEAT-001", "feature")
	e.Content = "`REQ-003`"
	require.NoError(t, s.UpdateEntity(ctx, e))
	require.Equal(t, []string{"FEAT-001"}, mentionIDs(t, s, "REQ-003"))
}

// TestMentions_VisibleAcrossStores pins that the index lives in the database:
// a write through one store is a backlink through another on the same schema,
// as for two rela-server processes.
func TestMentions_VisibleAcrossStores(t *testing.T) {
	pool := newScopedPool(t)
	writer, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	e := entity.New("FEAT-001", "feature")
	e.Content = "`REQ-001`"
	require.NoError(t, writer.CreateEntity(t.Context(), e))
	require.Equal(t, []string{"FEAT-001"}, mentionIDs(t, reader, "REQ-001"))
}
