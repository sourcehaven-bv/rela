package memstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// recordingObserver captures the bare ids an observer is told to drop.
type recordingObserver struct{ deleted []string }

func (o *recordingObserver) EntityPut(*entity.Entity) error { return nil }
func (o *recordingObserver) EntityDelete(id string) error {
	o.deleted = append(o.deleted, id)
	return nil
}
func (o *recordingObserver) EntityRenamed(string, *entity.Entity) error { return nil }

// TestFaceDeleteDoesNotDeIndexALiveEntity pins a property that is otherwise
// convention-only, and whose violation would be invisible for a long time.
//
// Observers (the search indexers) are keyed by BARE ID — pgstore's own
// comment says "observers are bare-id keyed, so a single delete covers every
// face". So notifying them on a PER-FACE delete tells the index the whole
// entity is gone while other faces still exist: the entity silently vanishes
// from search while `GetEntity` still returns it. Nothing surfaces that until
// somebody searches for content that is demonstrably still there.
//
// The notify is therefore correct only when the LAST face goes, and both
// halves need asserting — that it stays silent with siblings standing, and
// that it does still fire when nothing is left.
func TestFaceDeleteDoesNotDeIndexALiveEntity(t *testing.T) {
	obs := &recordingObserver{}
	s := memstore.New(memstore.WithObserver(obs))
	ctx := context.Background()

	mk := func(p entity.Face) {
		require.NoError(t, s.CreateEntity(ctx,
			&entity.Entity{ID: "PAGE-1", Type: "page", Face: p}))
	}
	mk("")
	mk("draft")

	_, err := s.DeleteEntityState(ctx, "PAGE-1", entity.Face("draft"))
	require.NoError(t, err)
	require.Empty(t, obs.deleted,
		"deleting one face must NOT tell bare-id-keyed observers the entity is "+
			"gone — the default face still exists, so this would de-index a live entity")

	// The last face going IS an entity deletion, and must notify.
	_, err = s.DeleteEntityState(ctx, "PAGE-1", "")
	require.NoError(t, err)
	require.Equal(t, []string{"PAGE-1"}, obs.deleted,
		"removing the last face must notify observers, or the index keeps a ghost")
}
