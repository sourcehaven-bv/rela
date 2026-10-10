// Package pilestest is the conformance harness every [piles.Store]
// implementation must pass.
//
// It exists for the same reason internal/comments/commentstest does: two
// backends without a shared contract test are two subtly different behaviors.
// The rules most likely to drift are the stack order, the duplicate and
// eviction rules, owner isolation and the rename collapse, because a Go slice
// and a SQL table make different things easy.
//
// Every assertion is deterministic. Times are fixed instants passed in by the
// suite, and the limits are passed as small numbers so eviction and the pile
// cap are reached in a handful of rows.
package pilestest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
)

// Factory returns a fresh, empty store for one subtest. Cleanup is the
// factory's responsibility, normally via t.Cleanup.
type Factory func(t *testing.T) piles.Store

// base is an arbitrary fixed instant, whole seconds so a database backend's
// microsecond precision cannot make a round-tripped time differ.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

const (
	// bigCap is a limit the test does not mean to reach.
	bigCap = 1000
	alice  = "alice"
	bob    = "bob"
	draft  = entity.Face("draft")
)

// idSeq mints distinct pile ids across the whole run, so no two subtests can
// collide even when a backend shares storage between them.
var idSeq atomic.Int64

func newID() string { return fmt.Sprintf("PIL-T%07d", idSeq.Add(1)) }

// ref builds an item reference. The face is a parameter, never a literal, so
// a bare ref here is a deliberate implicit-face item.
func ref(id string, face entity.Face) entity.Ref { return entity.Ref{ID: id, Face: face} }

// bare is the implicit-face ref of id.
func bare(id string) entity.Ref { return ref(id, entity.ImplicitFace) }

func refs(ids ...string) []entity.Ref {
	out := make([]entity.Ref, len(ids))
	for i, id := range ids {
		out[i] = bare(id)
	}
	return out
}

// at returns base plus n minutes, so a sequence of operations has distinct,
// ordered times.
func at(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

// create stores a new pile for owner and returns it as read back.
func create(
	ctx context.Context, t *testing.T, s piles.Store, owner, name string,
	created time.Time, items []entity.Ref, maxItems int,
) piles.Pile {
	t.Helper()
	p := piles.Pile{ID: newID(), Owner: owner, Name: name, Icon: piles.DefaultIcon, Created: created, Updated: created}
	require.NoError(t, s.CreatePile(ctx, p, items, bigCap, maxItems))
	got, err := s.GetPile(ctx, owner, p.ID)
	require.NoError(t, err)
	return got
}

// itemRefs returns a pile's refs in stored order.
func itemRefs(p piles.Pile) []entity.Ref {
	out := make([]entity.Ref, len(p.Items))
	for i, it := range p.Items {
		out[i] = it.Ref
	}
	return out
}

// mustGet reads a pile back and returns its refs.
func mustGet(ctx context.Context, t *testing.T, s piles.Store, owner, id string) []entity.Ref {
	t.Helper()
	p, err := s.GetPile(ctx, owner, id)
	require.NoError(t, err)
	return itemRefs(p)
}

// RunAll runs every conformance suite against f.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Piles", func(t *testing.T) { RunPileTests(t, f) })
	t.Run("Isolation", func(t *testing.T) { RunIsolationTests(t, f) })
	t.Run("Items", func(t *testing.T) { RunItemTests(t, f) })
	t.Run("Faces", func(t *testing.T) { RunFaceTests(t, f) })
	t.Run("Lifecycle", func(t *testing.T) { RunLifecycleTests(t, f) })
	t.Run("Owners", func(t *testing.T) { RunOwnerTests(t, f) })
	t.Run("Concurrency", func(t *testing.T) { RunConcurrencyTests(t, f) })
	t.Run("IDs", func(t *testing.T) { RunIDTests(t, f) })
	t.Run("KeepExisting", func(t *testing.T) { RunKeepExistingTests(t, f) })
}

// RunPileTests pins create, read, update and delete of piles themselves.
func RunPileTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("create, list, get and find by name", func(t *testing.T) {
		s := f(t)
		first := create(ctx, t, s, alice, "Inbox", at(1), nil, bigCap)
		second := create(ctx, t, s, alice, "Review", at(2), refs("A"), bigCap)

		require.Equal(t, alice, first.Owner)
		require.Equal(t, "Inbox", first.Name)
		require.Equal(t, piles.DefaultIcon, first.Icon)
		require.True(t, first.Created.Equal(at(1)), "created round-trips")
		require.True(t, first.Updated.Equal(at(1)), "updated round-trips")
		require.Empty(t, first.Items)

		list, err := s.ListPiles(ctx, alice)
		require.NoError(t, err)
		require.Len(t, list, 2)
		require.Equal(t, first.ID, list[0].ID, "oldest pile first")
		require.Equal(t, second.ID, list[1].ID)
		require.Equal(t, refs("A"), itemRefs(list[1]), "list carries the items")

		byName, err := s.PileByName(ctx, alice, "rEvIeW")
		require.NoError(t, err, "names match case-insensitively")
		require.Equal(t, second.ID, byName.ID)
		require.Equal(t, refs("A"), itemRefs(byName))
	})

	t.Run("missing piles are not found", func(t *testing.T) {
		s := f(t)
		_, err := s.GetPile(ctx, alice, "PIL-MISSING1")
		require.ErrorIs(t, err, piles.ErrNotFound)
		_, err = s.PileByName(ctx, alice, "nothing")
		require.ErrorIs(t, err, piles.ErrNotFound)
		list, err := s.ListPiles(ctx, alice)
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("names are unique per owner, case-insensitively", func(t *testing.T) {
		s := f(t)
		create(ctx, t, s, alice, "Inbox", at(1), nil, bigCap)
		err := s.CreatePile(ctx, piles.Pile{
			ID: newID(), Owner: alice, Name: "INBOX", Icon: piles.DefaultIcon, Created: at(2), Updated: at(2),
		}, nil, bigCap, bigCap)
		require.ErrorIs(t, err, piles.ErrNameTaken)

		// Another owner may use the same name.
		create(ctx, t, s, bob, "Inbox", at(3), nil, bigCap)
	})

	t.Run("the pile cap holds per owner", func(t *testing.T) {
		s := f(t)
		for i := range 3 {
			require.NoError(t, s.CreatePile(ctx, piles.Pile{
				ID: newID(), Owner: alice, Name: fmt.Sprintf("p%d", i), Icon: piles.DefaultIcon,
				Created: at(i), Updated: at(i),
			}, nil, 3, bigCap))
		}
		err := s.CreatePile(ctx, piles.Pile{
			ID: newID(), Owner: alice, Name: "one too many", Icon: piles.DefaultIcon, Created: at(3), Updated: at(3),
		}, nil, 3, bigCap)
		require.ErrorIs(t, err, piles.ErrLimit)

		require.NoError(t, s.CreatePile(ctx, piles.Pile{
			ID: newID(), Owner: bob, Name: "p0", Icon: piles.DefaultIcon, Created: at(3), Updated: at(3),
		}, nil, 3, bigCap), "another owner's piles do not count against alice's cap")
	})

	t.Run("update renames and re-icons", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Inbox", at(1), refs("A"), bigCap)
		create(ctx, t, s, alice, "Other", at(2), nil, bigCap)

		require.NoError(t, s.UpdatePile(ctx, alice, p.ID, "Today", "star", at(3)))
		got, err := s.GetPile(ctx, alice, p.ID)
		require.NoError(t, err)
		require.Equal(t, "Today", got.Name)
		require.Equal(t, "star", got.Icon)
		require.True(t, got.Updated.Equal(at(3)))
		require.True(t, got.Created.Equal(at(1)), "update leaves created alone")
		require.Equal(t, refs("A"), itemRefs(got), "update leaves items alone")

		require.NoError(t, s.UpdatePile(ctx, alice, p.ID, "TODAY", "star", at(3)),
			"a pile may change the case of its own name")
		require.ErrorIs(t, s.UpdatePile(ctx, alice, p.ID, "other", "star", at(3)), piles.ErrNameTaken)
		require.ErrorIs(t, s.UpdatePile(ctx, alice, "PIL-MISSING1", "x", "star", at(3)), piles.ErrNotFound)
	})

	t.Run("delete removes the pile and its items", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Inbox", at(1), refs("A", "B"), bigCap)
		require.NoError(t, s.DeletePile(ctx, alice, p.ID))
		_, err := s.GetPile(ctx, alice, p.ID)
		require.ErrorIs(t, err, piles.ErrNotFound)
		require.ErrorIs(t, s.DeletePile(ctx, alice, p.ID), piles.ErrNotFound)

		// The name is free again and the new pile starts empty.
		again := create(ctx, t, s, alice, "Inbox", at(2), nil, bigCap)
		require.Empty(t, again.Items)
	})
}

// RunIsolationTests pins that every per-pile method treats another owner's
// pile exactly like a missing one, and changes nothing.
func RunIsolationTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()
	s := f(t)
	p := create(ctx, t, s, alice, "Private", at(1), refs("A"), bigCap)

	_, err := s.GetPile(ctx, bob, p.ID)
	require.ErrorIs(t, err, piles.ErrNotFound, "GetPile")
	_, err = s.PileByName(ctx, bob, "Private")
	require.ErrorIs(t, err, piles.ErrNotFound, "PileByName")
	list, err := s.ListPiles(ctx, bob)
	require.NoError(t, err)
	require.Empty(t, list, "ListPiles")
	require.ErrorIs(t, s.UpdatePile(ctx, bob, p.ID, "Mine", "star", at(2)), piles.ErrNotFound, "UpdatePile")
	_, err = s.AddItems(ctx, bob, p.ID, refs("B"), at(2), bigCap, piles.EvictOldest)
	require.ErrorIs(t, err, piles.ErrNotFound, "AddItems")
	require.ErrorIs(t, s.RemoveItems(ctx, bob, p.ID, refs("A")), piles.ErrNotFound, "RemoveItems")
	require.ErrorIs(t, s.DeletePile(ctx, bob, p.ID), piles.ErrNotFound, "DeletePile")

	got, err := s.GetPile(ctx, alice, p.ID)
	require.NoError(t, err, "the owner's pile survives every foreign call")
	require.Equal(t, "Private", got.Name)
	require.Equal(t, refs("A"), itemRefs(got))

	// A foreign pile of the same name must not block bob's own.
	create(ctx, t, s, bob, "Private", at(3), nil, bigCap)
}

// RunItemTests pins the stack: order, duplicates, eviction and removal.
func RunItemTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("items come back newest first", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Stack", at(1), refs("A", "B"), bigCap)
		require.Equal(t, refs("A", "B"), itemRefs(p), "the first created ref is newest")
		for _, it := range p.Items {
			require.True(t, it.Added.Equal(at(1)), "created items carry the pile's creation time")
		}

		added, err := s.AddItems(ctx, alice, p.ID, refs("C", "D"), at(2), bigCap, piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 2, added)
		got, err := s.GetPile(ctx, alice, p.ID)
		require.NoError(t, err)
		require.Equal(t, refs("C", "D", "A", "B"), itemRefs(got))
		require.True(t, got.Items[0].Added.Equal(at(2)), "added items carry the add time")
	})

	t.Run("a duplicate keeps its place and does not count", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Stack", at(1), refs("A", "B", "C"), bigCap)

		added, err := s.AddItems(ctx, alice, p.ID, refs("C"), at(2), bigCap, piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 0, added)
		require.Equal(t, refs("A", "B", "C"), mustGet(ctx, t, s, alice, p.ID))

		added, err = s.AddItems(ctx, alice, p.ID, refs("D", "B", "D"), at(3), bigCap, piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 1, added, "only D is new, and once")
		require.Equal(t, refs("D", "A", "B", "C"), mustGet(ctx, t, s, alice, p.ID))
	})

	t.Run("adding past the cap evicts the oldest", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Stack", at(1), refs("A", "B", "C"), 3)

		added, err := s.AddItems(ctx, alice, p.ID, refs("D"), at(2), 3, piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 1, added)
		require.Equal(t, refs("D", "A", "B"), mustGet(ctx, t, s, alice, p.ID))

		added, err = s.AddItems(ctx, alice, p.ID, refs("E", "F", "G", "H"), at(3), 3, piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 3, added, "refs past the cap in one call are dropped, not added and evicted")
		require.Equal(t, refs("E", "F", "G"), mustGet(ctx, t, s, alice, p.ID))
	})

	t.Run("create collapses duplicates and caps the refs", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Stack", at(1), refs("A", "B", "A", "C", "D"), 3)
		require.Equal(t, refs("A", "B", "C"), itemRefs(p))
	})

	t.Run("remove ignores refs that are not on the pile", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Stack", at(1), refs("A", "B", "C"), bigCap)
		other := create(ctx, t, s, alice, "Other", at(2), refs("B"), bigCap)

		require.NoError(t, s.RemoveItems(ctx, alice, p.ID, refs("B", "X")))
		require.Equal(t, refs("A", "C"), mustGet(ctx, t, s, alice, p.ID))
		require.Equal(t, refs("B"), mustGet(ctx, t, s, alice, other.ID), "remove touches only its pile")

		require.NoError(t, s.RemoveItems(ctx, alice, p.ID, nil), "an empty remove is a no-op")
		require.ErrorIs(t, s.RemoveItems(ctx, alice, "PIL-MISSING1", refs("A")), piles.ErrNotFound)
	})

	t.Run("adding to a missing pile is not found", func(t *testing.T) {
		s := f(t)
		_, err := s.AddItems(ctx, alice, "PIL-MISSING1", refs("A"), at(1), bigCap, piles.EvictOldest)
		require.ErrorIs(t, err, piles.ErrNotFound)
	})
}

// RunFaceTests pins that an item is a (id, face) pair, not an id.
func RunFaceTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("each face is its own item", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Faces", at(1), nil, bigCap)
		added, err := s.AddItems(ctx, alice, p.ID, []entity.Ref{bare("X"), ref("X", draft)}, at(2), bigCap,
			piles.EvictOldest)
		require.NoError(t, err)
		require.Equal(t, 2, added)
		require.Equal(t, []entity.Ref{bare("X"), ref("X", draft)}, mustGet(ctx, t, s, alice, p.ID))

		require.NoError(t, s.RemoveItems(ctx, alice, p.ID, []entity.Ref{ref("X", draft)}))
		require.Equal(t, []entity.Ref{bare("X")}, mustGet(ctx, t, s, alice, p.ID),
			"removing one face leaves the other")
	})

	t.Run("deleting a face drops only that face, everywhere", func(t *testing.T) {
		s := f(t)
		mine := create(ctx, t, s, alice, "Faces", at(1),
			[]entity.Ref{ref("X", draft), bare("X"), ref("X", "published")}, bigCap)
		theirs := create(ctx, t, s, bob, "Faces", at(2), []entity.Ref{ref("X", draft), bare("Y")}, bigCap)

		require.NoError(t, s.DeleteFace(ctx, "X", draft))
		require.Equal(t, []entity.Ref{bare("X"), ref("X", "published")}, mustGet(ctx, t, s, alice, mine.ID))
		require.Equal(t, []entity.Ref{bare("Y")}, mustGet(ctx, t, s, bob, theirs.ID))
	})
}

// RunLifecycleTests pins the item hooks: rename and delete of an entity on a
// pile. They never touch owners; see [RunOwnerTests].
func RunLifecycleTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("rename rewrites every face on every pile, in place", func(t *testing.T) {
		s := f(t)
		mine := create(ctx, t, s, alice, "One", at(1), []entity.Ref{bare("A"), ref("OLD", draft), bare("OLD")}, bigCap)
		theirs := create(ctx, t, s, bob, "Two", at(2), []entity.Ref{bare("OLD"), bare("B")}, bigCap)

		require.NoError(t, s.RenameEntity(ctx, "OLD", "NEW"))
		require.Equal(t, []entity.Ref{bare("A"), ref("NEW", draft), bare("NEW")}, mustGet(ctx, t, s, alice, mine.ID))
		require.Equal(t, []entity.Ref{bare("NEW"), bare("B")}, mustGet(ctx, t, s, bob, theirs.ID))
	})

	t.Run("rename collapses onto a ref already on the pile", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "One", at(1),
			[]entity.Ref{bare("NEW"), bare("A"), bare("OLD"), ref("OLD", draft)}, bigCap)

		require.NoError(t, s.RenameEntity(ctx, "OLD", "NEW"))
		require.Equal(t, []entity.Ref{bare("NEW"), bare("A"), ref("NEW", draft)}, mustGet(ctx, t, s, alice, p.ID),
			"the existing NEW keeps its place; OLD@draft has no twin and moves")
	})

	t.Run("rename matches the id exactly", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "One", at(1), refs("T-1", "T-10", "T_1"), bigCap)
		require.NoError(t, s.RenameEntity(ctx, "T-1", "T-9"))
		require.Equal(t, refs("T-9", "T-10", "T_1"), mustGet(ctx, t, s, alice, p.ID))
	})

	t.Run("rename leaves owners alone", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, "P-1", "Mine", at(1), refs("P-1"), bigCap)
		require.NoError(t, s.RenameEntity(ctx, "P-1", "P-2"))

		got, err := s.GetPile(ctx, "P-1", p.ID)
		require.NoError(t, err, "only RenameOwner moves piles")
		require.Equal(t, refs("P-2"), itemRefs(got), "the item is still rewritten")
		_, err = s.GetPile(ctx, "P-2", p.ID)
		require.ErrorIs(t, err, piles.ErrNotFound)
	})

	t.Run("rename to the same id changes nothing", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "One", at(1), refs("A"), bigCap)
		require.NoError(t, s.RenameEntity(ctx, "A", "A"))
		require.Equal(t, refs("A"), mustGet(ctx, t, s, alice, p.ID))
	})

	t.Run("delete drops the items on every face and leaves owners alone", func(t *testing.T) {
		s := f(t)
		mine := create(ctx, t, s, alice, "One", at(1), []entity.Ref{bare("A"), bare("GONE"), ref("GONE", draft)}, bigCap)
		owned := create(ctx, t, s, "GONE", "Theirs", at(2), refs("A"), bigCap)

		require.NoError(t, s.DeleteEntity(ctx, "GONE"))
		require.Equal(t, refs("A"), mustGet(ctx, t, s, alice, mine.ID))
		require.Equal(t, refs("A"), mustGet(ctx, t, s, "GONE", owned.ID), "only DeleteOwner drops piles")
	})
}

// RunOwnerTests pins RenameOwner and DeleteOwner, which the service calls for
// a renamed or deleted person.
func RunOwnerTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("rename owner moves every pile with its items", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, "P-1", "Mine", at(1), refs("A"), bigCap)
		q := create(ctx, t, s, "P-1", "Also", at(2), nil, bigCap)
		other := create(ctx, t, s, bob, "Mine", at(3), refs("B"), bigCap)
		require.NoError(t, s.RenameOwner(ctx, "P-1", "P-2"))

		_, err := s.GetPile(ctx, "P-1", p.ID)
		require.ErrorIs(t, err, piles.ErrNotFound, "the old owner no longer holds the pile")
		got, err := s.GetPile(ctx, "P-2", p.ID)
		require.NoError(t, err)
		require.Equal(t, "P-2", got.Owner)
		require.Equal(t, refs("A"), itemRefs(got))
		list, err := s.ListPiles(ctx, "P-2")
		require.NoError(t, err)
		require.Len(t, list, 2)
		require.Equal(t, q.ID, list[1].ID)
		require.Equal(t, refs("B"), mustGet(ctx, t, s, bob, other.ID), "other owners are untouched")
	})

	t.Run("a moved pile wins a name clash", func(t *testing.T) {
		s := f(t)
		moved := create(ctx, t, s, "P-1", "Inbox", at(1), refs("A"), bigCap)
		create(ctx, t, s, "P-2", "inbox", at(2), refs("STALE"), bigCap)
		require.NoError(t, s.RenameOwner(ctx, "P-1", "P-2"))

		list, err := s.ListPiles(ctx, "P-2")
		require.NoError(t, err)
		require.Len(t, list, 1, "two piles of one name must not coexist")
		require.Equal(t, moved.ID, list[0].ID)
		require.Equal(t, refs("A"), itemRefs(list[0]))
	})

	t.Run("rename owner to the same owner changes nothing", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, "P-1", "Mine", at(1), refs("A"), bigCap)
		require.NoError(t, s.RenameOwner(ctx, "P-1", "P-1"))
		require.Equal(t, refs("A"), mustGet(ctx, t, s, "P-1", p.ID))
	})

	t.Run("delete owner drops every pile and nothing else", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, "GONE", "Theirs", at(1), refs("A"), bigCap)
		kept := create(ctx, t, s, alice, "Mine", at(2), refs("GONE"), bigCap)
		require.NoError(t, s.DeleteOwner(ctx, "GONE"))

		_, err := s.GetPile(ctx, "GONE", p.ID)
		require.ErrorIs(t, err, piles.ErrNotFound, "a deleted person's piles must not pass to a reused id")
		list, err := s.ListPiles(ctx, "GONE")
		require.NoError(t, err)
		require.Empty(t, list)
		require.Equal(t, refs("GONE"), mustGet(ctx, t, s, alice, kept.ID), "items naming the owner stay")
	})
}

// RunConcurrencyTests pins that each method is atomic: concurrent adds lose
// nothing and the limits hold under contention.
func RunConcurrencyTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()
	const workers = 20

	t.Run("concurrent adds lose nothing", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Busy", at(1), nil, bigCap)
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		for i := range workers {
			wg.Go(func() {
				_, err := s.AddItems(ctx, alice, p.ID, refs(fmt.Sprintf("R-%d", i)), at(2), bigCap,
					piles.EvictOldest)
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Len(t, mustGet(ctx, t, s, alice, p.ID), workers)
	})

	t.Run("concurrent creates never exceed the pile cap", func(t *testing.T) {
		s := f(t)
		const maxPiles = 5
		var wg sync.WaitGroup
		var created, limited atomic.Int64
		errs := make(chan error, workers)
		for i := range workers {
			wg.Go(func() {
				err := s.CreatePile(ctx, piles.Pile{
					ID: newID(), Owner: alice, Name: fmt.Sprintf("p%d", i), Icon: piles.DefaultIcon,
					Created: at(i), Updated: at(i),
				}, nil, maxPiles, bigCap)
				switch {
				case err == nil:
					created.Add(1)
				case errors.Is(err, piles.ErrLimit):
					limited.Add(1)
				default:
					errs <- err
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Equal(t, int64(maxPiles), created.Load())
		require.Equal(t, int64(workers-maxPiles), limited.Load())
		list, err := s.ListPiles(ctx, alice)
		require.NoError(t, err)
		require.Len(t, list, maxPiles)
	})

	t.Run("concurrent adds at the cap leave exactly the cap", func(t *testing.T) {
		s := f(t)
		const maxItems = 5
		p := create(ctx, t, s, alice, "Full", at(1), refs("A", "B", "C", "D", "E"), maxItems)
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		for i := range workers {
			wg.Go(func() {
				_, err := s.AddItems(ctx, alice, p.ID, refs(fmt.Sprintf("R-%d", i)), at(2), maxItems,
					piles.EvictOldest)
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Len(t, mustGet(ctx, t, s, alice, p.ID), maxItems)
	})
}

// RunIDTests pins that a pile id is never reused.
func RunIDTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()
	t.Run("an existing pile id is refused", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Inbox", at(1), refs("A"), bigCap)
		err := s.CreatePile(ctx, piles.Pile{
			ID: p.ID, Owner: bob, Name: "Other", Icon: piles.DefaultIcon, Created: at(2), Updated: at(2),
		}, nil, bigCap, bigCap)
		require.ErrorIs(t, err, piles.ErrIDTaken)
		require.Equal(t, refs("A"), mustGet(ctx, t, s, alice, p.ID), "the existing pile is untouched")
	})
}

// RunKeepExistingTests pins [piles.KeepExisting], the overflow mode of a push
// into another user's pile: it never evicts, and fills only free room.
func RunKeepExistingTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()
	const workers, room = 20, 4

	t.Run("keep-existing on a full pile adds nothing", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Full", at(1), refs("A", "B", "C"), 3)

		added, err := s.AddItems(ctx, alice, p.ID, refs("D", "E"), at(2), 3, piles.KeepExisting)
		require.NoError(t, err)
		require.Zero(t, added)
		require.Equal(t, refs("A", "B", "C"), mustGet(ctx, t, s, alice, p.ID), "no item is evicted")
	})

	t.Run("keep-existing fills only the free room, first refs first", func(t *testing.T) {
		s := f(t)
		p := create(ctx, t, s, alice, "Room", at(1), refs("A", "B"), room)

		added, err := s.AddItems(ctx, alice, p.ID, refs("B", "C", "D", "E"), at(2), room, piles.KeepExisting)
		require.NoError(t, err)
		require.Equal(t, 2, added, "B is already there; C and D fill the two free slots; E is dropped")
		require.Equal(t, refs("C", "D", "A", "B"), mustGet(ctx, t, s, alice, p.ID))
	})

	t.Run("concurrent keep-existing adds fill the room and evict nothing", func(t *testing.T) {
		s := f(t)
		const maxItems = 5
		p := create(ctx, t, s, alice, "Room", at(1), refs("A", "B"), maxItems)
		var wg sync.WaitGroup
		var added atomic.Int64
		errs := make(chan error, workers)
		for i := range workers {
			wg.Go(func() {
				n, err := s.AddItems(ctx, alice, p.ID, refs(fmt.Sprintf("R-%d", i)), at(2), maxItems,
					piles.KeepExisting)
				added.Add(int64(n))
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Equal(t, int64(maxItems-2), added.Load())
		got := mustGet(ctx, t, s, alice, p.ID)
		require.Len(t, got, maxItems)
		require.Equal(t, refs("A", "B"), got[maxItems-2:], "the existing items survive")
	})
}
