package pilestest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
)

// inboxKey is the key of the declared pile the read-state tests use.
const inboxKey = "inbox"

// createKeyed stores the declared inbox pile for owner and returns it as read
// back.
func createKeyed(ctx context.Context, t *testing.T, s piles.Store, owner string, items []entity.Ref) piles.Pile {
	t.Helper()
	key := inboxKey
	p := piles.Pile{ID: newID(), Owner: owner, Key: key, Name: "Declared " + key, Icon: piles.DefaultIcon,
		Created: at(0), Updated: at(0)}
	require.NoError(t, s.CreatePile(ctx, p, items, bigCap, bigCap))
	got, err := s.PileByKey(ctx, owner, key)
	require.NoError(t, err)
	return got
}

func add(ctx context.Context, t *testing.T, s piles.Store, p piles.Pile, ids []string, opts piles.AddOptions) int {
	t.Helper()
	n, err := s.AddItems(ctx, p.Owner, p.ID, refs(ids...), opts)
	require.NoError(t, err)
	return n
}

func mustPile(ctx context.Context, t *testing.T, s piles.Store, p piles.Pile) piles.Pile {
	t.Helper()
	got, err := s.GetPile(ctx, p.Owner, p.ID)
	require.NoError(t, err)
	return got
}

// RunDeclaredTests pins keyed piles: a key is unique per owner, a keyed pile
// is outside the name space and the pile limit, and it moves with its owner.
func RunDeclaredTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("a key is found by key and not by name", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		require.Equal(t, "inbox", p.Key)
		_, err := s.PileByName(ctx, alice, p.Name)
		require.ErrorIs(t, err, piles.ErrNotFound, "a keyed pile is not in the name space")
		_, err = s.PileByKey(ctx, bob, "inbox")
		require.ErrorIs(t, err, piles.ErrNotFound, "keys are per owner")
		_, err = s.PileByKey(ctx, alice, "")
		require.ErrorIs(t, err, piles.ErrNotFound, "the empty key names no pile")
	})

	t.Run("a second pile with the key is refused", func(t *testing.T) {
		s := f(t)
		createKeyed(ctx, t, s, alice, nil)
		dup := piles.Pile{ID: newID(), Owner: alice, Key: inboxKey, Name: "Other", Icon: piles.DefaultIcon,
			Created: at(1), Updated: at(1)}
		require.ErrorIs(t, s.CreatePile(ctx, dup, nil, bigCap, bigCap), piles.ErrKeyTaken)
		createKeyed(ctx, t, s, bob, nil)
	})

	t.Run("a keyed pile clashes with no name and counts toward no limit", func(t *testing.T) {
		s := f(t)
		user := piles.Pile{ID: newID(), Owner: alice, Name: "Declared inbox", Icon: piles.DefaultIcon,
			Created: at(1), Updated: at(1)}
		require.NoError(t, s.CreatePile(ctx, user, nil, 1, bigCap))
		keyed := piles.Pile{ID: newID(), Owner: alice, Key: inboxKey, Name: "Declared inbox",
			Icon: piles.DefaultIcon, Created: at(2), Updated: at(2)}
		require.NoError(t, s.CreatePile(ctx, keyed, nil, 1, bigCap), "a full user quota does not block a keyed pile")
		other := piles.Pile{ID: newID(), Owner: alice, Name: "Second", Icon: piles.DefaultIcon,
			Created: at(3), Updated: at(3)}
		require.ErrorIs(t, s.CreatePile(ctx, other, nil, 1, bigCap), piles.ErrLimit,
			"the keyed pile leaves the quota as it was")
		got, err := s.PileByName(ctx, alice, "declared inbox")
		require.NoError(t, err)
		require.Equal(t, user.ID, got.ID)
	})

	t.Run("rename owner moves a keyed pile and the moved pile wins a key clash", func(t *testing.T) {
		s := f(t)
		moved := createKeyed(ctx, t, s, "P-1", refs("A"))
		createKeyed(ctx, t, s, "P-2", refs("STALE"))
		require.NoError(t, s.RenameOwner(ctx, "P-1", "P-2"))
		got, err := s.PileByKey(ctx, "P-2", "inbox")
		require.NoError(t, err)
		require.Equal(t, moved.ID, got.ID)
		require.Equal(t, refs("A"), itemRefs(got))
		list, err := s.ListPiles(ctx, "P-2")
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("a keyed and an unkeyed pile of one name both survive an owner rename", func(t *testing.T) {
		s := f(t)
		createKeyed(ctx, t, s, "P-1", nil)
		create(ctx, t, s, "P-2", "Declared inbox", at(1), nil, bigCap)
		require.NoError(t, s.RenameOwner(ctx, "P-1", "P-2"))
		list, err := s.ListPiles(ctx, "P-2")
		require.NoError(t, err)
		require.Len(t, list, 2)
	})
}

// RunReadStateTests pins the item attribution, bump, read state, pruning and
// read-first eviction a declared pile relies on.
func RunReadStateTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("an add records actor and reason, unread", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		add(ctx, t, s, p, []string{"A"}, piles.AddOptions{Now: at(1), MaxItems: bigCap, Overflow: piles.EvictReadFirst,
			Actor: bob, Reason: "done"})
		got := mustPile(ctx, t, s, p)
		require.Len(t, got.Items, 1)
		it := got.Items[0]
		require.Equal(t, bob, it.Actor)
		require.Equal(t, "done", it.Reason)
		require.False(t, it.Read())
		require.True(t, it.ReadAt.IsZero())
	})

	t.Run("mark read stamps the named refs once", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, refs("A", "B", "C"))
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("B", "MISSING"), false, at(1)))
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("B"), false, at(2)))
		got := mustPile(ctx, t, s, p)
		require.False(t, got.Items[0].Read())
		require.True(t, got.Items[1].Read())
		require.True(t, got.Items[1].ReadAt.Equal(at(1)), "a second mark keeps the first read time")
		require.False(t, got.Items[2].Read())
		require.Equal(t, refs("A", "B", "C"), itemRefs(got), "marking read does not reorder")

		require.NoError(t, s.MarkRead(ctx, alice, p.ID, nil, true, at(3)))
		got = mustPile(ctx, t, s, p)
		for _, it := range got.Items {
			require.True(t, it.Read())
		}
		require.True(t, got.Items[1].ReadAt.Equal(at(1)))
		require.True(t, got.Items[0].ReadAt.Equal(at(3)))
	})

	t.Run("mark read is owner scoped", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, refs("A"))
		require.ErrorIs(t, s.MarkRead(ctx, bob, p.ID, refs("A"), false, at(1)), piles.ErrNotFound)
		require.ErrorIs(t, s.MarkRead(ctx, alice, "PIL-MISSING1", refs("A"), false, at(1)), piles.ErrNotFound)
		require.False(t, mustPile(ctx, t, s, p).Items[0].Read())
	})

	t.Run("bump moves a present ref to the top, unread, with the new actor", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, refs("A", "B", "C"))
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("C"), false, at(1)))
		n := add(ctx, t, s, p, []string{"D", "C"}, piles.AddOptions{Now: at(2), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst, Bump: true, Actor: bob, Reason: "again"})
		require.Equal(t, 2, n, "D is new and C is bumped")
		got := mustPile(ctx, t, s, p)
		require.Equal(t, refs("D", "C", "A", "B"), itemRefs(got))
		c := got.Items[1]
		require.False(t, c.Read(), "a bumped item is unread again")
		require.Equal(t, bob, c.Actor)
		require.Equal(t, "again", c.Reason)
		require.True(t, c.Added.Equal(at(2)))
	})

	t.Run("without bump a present ref keeps its place and state", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, refs("A", "B"))
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("B"), false, at(1)))
		add(ctx, t, s, p, []string{"B"}, piles.AddOptions{Now: at(2), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst, Actor: bob})
		got := mustPile(ctx, t, s, p)
		require.Equal(t, refs("A", "B"), itemRefs(got))
		require.True(t, got.Items[1].Read())
		require.Empty(t, got.Items[1].Actor)
	})

	runEvictAndPrune(ctx, t, f)
}

// runEvictAndPrune is the second half of RunReadStateTests: what leaves a
// declared pile, and what a rename keeps.
func runEvictAndPrune(ctx context.Context, t *testing.T, f Factory) {
	t.Helper()
	t.Run("evict read first keeps unread items", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, refs("A", "B", "C"))
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("A"), false, at(1)))
		n := add(ctx, t, s, p, []string{"D"}, piles.AddOptions{Now: at(2), MaxItems: 3, Overflow: piles.EvictReadFirst})
		require.Equal(t, 1, n)
		require.Equal(t, refs("D", "B", "C"), mustGet(ctx, t, s, alice, p.ID), "the read A goes before the older C")

		n = add(ctx, t, s, p, []string{"E"}, piles.AddOptions{Now: at(3), MaxItems: 3, Overflow: piles.EvictReadFirst})
		require.Equal(t, 1, n)
		require.Equal(t, refs("E", "D", "B"), mustGet(ctx, t, s, alice, p.ID), "with nothing read the oldest goes")
	})

	t.Run("prune drops items by read and by added time", func(t *testing.T) {
		const (
			tOld   = 1
			tAdded = 5
			tRead  = 6
			tAfter = 7
			tLate  = 9
		)
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		add(ctx, t, s, p, []string{"OLD"}, piles.AddOptions{Now: at(tOld), MaxItems: bigCap, Overflow: piles.EvictReadFirst})
		add(ctx, t, s, p, []string{"READ", "FRESH"}, piles.AddOptions{Now: at(tAdded), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst})
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("READ"), false, at(tRead)))

		require.NoError(t, s.PruneItems(ctx, alice, p.ID, at(tRead), at(0)))
		require.Equal(t, refs("READ", "FRESH", "OLD"), mustGet(ctx, t, s, alice, p.ID), "cutoffs are exclusive")

		require.NoError(t, s.PruneItems(ctx, alice, p.ID, at(tAfter), time.Time{}))
		require.Equal(t, refs("FRESH", "OLD"), mustGet(ctx, t, s, alice, p.ID))

		require.NoError(t, s.PruneItems(ctx, alice, p.ID, time.Time{}, at(2)))
		require.Equal(t, refs("FRESH"), mustGet(ctx, t, s, alice, p.ID))

		require.ErrorIs(t, s.PruneItems(ctx, bob, p.ID, at(tLate), at(tLate)), piles.ErrNotFound)
	})

	t.Run("rename owner moves the actor on every pile", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		add(ctx, t, s, p, []string{"A"}, piles.AddOptions{Now: at(1), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst, Actor: "P-OLD"})
		require.NoError(t, s.RenameOwner(ctx, "P-OLD", "P-NEW"))
		require.Equal(t, "P-NEW", mustPile(ctx, t, s, p).Items[0].Actor, "the actor owns no pile of their own")
	})

	t.Run("delete owner clears the actor on every pile", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		add(ctx, t, s, p, []string{"A"}, piles.AddOptions{Now: at(1), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst, Actor: "P-GONE"})
		require.NoError(t, s.DeleteOwner(ctx, "P-GONE"))
		got := mustPile(ctx, t, s, p)
		require.Len(t, got.Items, 1, "the item stays; only its actor goes")
		require.Empty(t, got.Items[0].Actor)
	})

	t.Run("rename entity keeps actor, reason and read state", func(t *testing.T) {
		s := f(t)
		p := createKeyed(ctx, t, s, alice, nil)
		add(ctx, t, s, p, []string{"OLD-ID"}, piles.AddOptions{Now: at(1), MaxItems: bigCap,
			Overflow: piles.EvictReadFirst, Actor: bob, Reason: "done"})
		require.NoError(t, s.MarkRead(ctx, alice, p.ID, refs("OLD-ID"), false, at(2)))
		require.NoError(t, s.RenameEntity(ctx, "OLD-ID", "NEW-ID"))
		got := mustPile(ctx, t, s, p)
		require.Equal(t, refs("NEW-ID"), itemRefs(got))
		it := got.Items[0]
		require.Equal(t, bob, it.Actor)
		require.Equal(t, "done", it.Reason)
		require.True(t, it.ReadAt.Equal(at(2)))
	})
}
