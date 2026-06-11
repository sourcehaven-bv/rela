package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestSearchAfterMixedCaseRename guards a bug the shared conformance suite can't
// catch: it uses same-case IDs, so it never exercises search_text after renaming
// to a mixed-case ID. The store maintains search_text as all-lowercase; a rename
// must keep it lowercase or the renamed entity becomes unfindable by its new ID
// (regression: RR-YXFYK / go-architect C1).
func TestSearchAfterMixedCaseRename(t *testing.T) {
	pool := newScopedPool(t)
	backend := pgstore.NewSearchBackend(pool)
	st, err := pgstore.New(pool, pgstore.WithObserver(backend))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("Old-ID", "ticket")))

	_, err = st.RenameEntity(ctx, "Old-ID", "New-MixedCase")
	require.NoError(t, err)

	// The backend matches case-insensitively, so a lowercased query for the new
	// ID must find the renamed entity.
	ids, err := backend.Search("new-mixedcase", 0)
	require.NoError(t, err)
	require.Contains(t, ids, "New-MixedCase",
		"renamed entity must be findable by its new ID (search_text stayed lowercase)")

	// The old ID must no longer match.
	old, err := backend.Search("old-id", 0)
	require.NoError(t, err)
	require.NotContains(t, old, "New-MixedCase")
	require.NotContains(t, old, "Old-ID")
}
