package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/userstatetest"
)

var userStateBase = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newUserStateStore returns a store over a freshly migrated, schema-isolated
// pool. DB-gated like the rest of this package: without
// RELA_TEST_DATABASE_URL the harness skips (or fails, under
// RELA_TEST_DATABASE_REQUIRED).
func newUserStateStore(t *testing.T) *pgstore.UserStateStore {
	t.Helper()
	pool := newScopedPool(t)
	require.NoError(t, pgstore.Migrate(context.Background(), pool))
	s, err := pgstore.NewUserStateStore(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestUserStateConformance runs the shared contract. Three backends without
// one are three subtly different behaviors — and the SQL implementation is
// where the expiry boundary and replace-don't-stack rules are easiest to get
// plausibly-but-differently wrong.
func TestUserStateConformance(t *testing.T) {
	userstatetest.RunAll(t, func(t *testing.T) userstate.Store {
		t.Helper()
		return newUserStateStore(t)
	})
}

func TestNewUserStateStore_RejectsNilDB(t *testing.T) {
	t.Parallel()
	_, err := pgstore.NewUserStateStore(nil)
	require.Error(t, err)
}

// The reason this backend exists: two processes sharing one database must not
// clobber each other. Two independent handles over the same pool stand in for
// two servers — the filesystem backend fails this by design, because its
// whole-document read-modify-write is last-writer-wins.
func TestUserState_ConcurrentHandlesDoNotClobber(t *testing.T) {
	pool := newScopedPool(t)
	require.NoError(t, pgstore.Migrate(context.Background(), pool))
	ctx := context.Background()

	first, err := pgstore.NewUserStateStore(pool)
	require.NoError(t, err)
	second, err := pgstore.NewUserStateStore(pool)
	require.NoError(t, err)

	aliceKey := userstate.Key{User: "alice", Source: "stale", EntityID: "T-1"}
	bobKey := userstate.Key{User: "bob", Source: "stale", EntityID: "T-2"}

	require.NoError(t, first.SetSnooze(ctx, aliceKey, userStateBase.Add(24*time.Hour)))
	require.NoError(t, second.SetSnooze(ctx, bobKey, userStateBase.Add(24*time.Hour)))

	// Each write must be visible through the OTHER handle, and neither may
	// have dropped the other's row.
	_, ok, err := second.SnoozedUntil(ctx, aliceKey, userStateBase)
	require.NoError(t, err)
	require.True(t, ok, "the second handle must see the first's write")

	_, ok, err = first.SnoozedUntil(ctx, bobKey, userStateBase)
	require.NoError(t, err)
	require.True(t, ok, "the first handle must see the second's write")
}

// Closing this handle must not tear down the shared pool: appbuild owns it and
// hands the same one to the store and the search backend.
func TestUserState_CloseLeavesPoolUsable(t *testing.T) {
	pool := newScopedPool(t)
	require.NoError(t, pgstore.Migrate(context.Background(), pool))
	ctx := context.Background()

	closing, err := pgstore.NewUserStateStore(pool)
	require.NoError(t, err)
	require.NoError(t, closing.Close())

	survivor, err := pgstore.NewUserStateStore(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = survivor.Close() })

	key := userstate.Key{User: "alice", Source: "stale"}
	require.NoError(t, survivor.SetSnooze(ctx, key, userStateBase.Add(time.Hour)),
		"closing one handle must not close the shared pool")
}

// A count-based source has no entity, and a source without key_props has no
// variant. Both land as empty strings rather than NULL so the primary key
// stays comparable — NULLs would let the same logical key insert repeatedly.
func TestUserState_EmptyKeyPartsAreStable(t *testing.T) {
	s := newUserStateStore(t)
	ctx := context.Background()
	key := userstate.Key{User: "alice", Source: "first-run"}

	require.NoError(t, s.SetSnooze(ctx, key, userStateBase.Add(time.Hour)))
	require.NoError(t, s.SetSnooze(ctx, key, userStateBase.Add(48*time.Hour)))

	until, ok, err := s.SnoozedUntil(ctx, key, userStateBase)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, until.Equal(userStateBase.Add(48*time.Hour)),
		"the second write must UPDATE the first, not insert a duplicate row")
}
