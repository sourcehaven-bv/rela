package entitymanager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// The id listing only helps if the real fsstore matches entityIDLister,
// outside and inside a transaction. A renamed or re-signed method would
// otherwise silently bring back a full file scan per create (BUG-TT73D0P)
// while every other test stays green.
var _ entityIDLister = (*fsstore.FSStore)(nil)

func TestFSStoreTxViewIsEntityIDLister(t *testing.T) {
	memfs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(memfs, "/")
	require.NoError(t, err)
	st, err := fsstore.New(fsstore.Config{
		FS: memfs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations",
		AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{"note": {Plural: "notes"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	require.NoError(t, st.Tx(context.Background(), func(tx store.Store) error {
		_, ok := tx.(entityIDLister)
		require.True(t, ok, "fsstore's transaction view must expose EntityIDs")
		return nil
	}))
}
