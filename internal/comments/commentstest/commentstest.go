// Package commentstest is the conformance suite every [comments.Store]
// implementation must pass.
//
// It exists because the parts of the contract most likely to diverge between
// backends are the ones nobody notices: List's ordering, whether a returned
// slice aliases stored state, and whether two concurrent Adds both survive. A
// backend that gets those subtly wrong still passes a hand-written smoke test
// and then behaves differently in production from the one the tests ran
// against.
package commentstest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
)

// Factory builds a fresh, empty store for one subtest.
type Factory func(t *testing.T) comments.Store

var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func target(id string) comments.Target {
	return comments.Target{Type: "ticket", ID: id}
}

// comment builds a comment with a fixed anchor, offsetting CreatedAt so
// ordering assertions are unambiguous.
func comment(id string, offset time.Duration, author, body string) comments.Comment {
	return comments.Comment{
		ID:        id,
		Author:    author,
		CreatedAt: base.Add(offset),
		Anchor:    comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
		Body:      body,
	}
}

// RunAll runs every conformance suite against f.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Empty", func(t *testing.T) { RunEmptyTests(t, f) })
	t.Run("Ordering", func(t *testing.T) { RunOrderingTests(t, f) })
	t.Run("Update", func(t *testing.T) { RunUpdateTests(t, f) })
	t.Run("Delete", func(t *testing.T) { RunDeleteTests(t, f) })
	t.Run("Isolation", func(t *testing.T) { RunIsolationTests(t, f) })
	t.Run("Rename", func(t *testing.T) { RunRenameTests(t, f) })
	t.Run("Concurrency", func(t *testing.T) { RunConcurrencyTests(t, f) })
	t.Run("RoundTrip", func(t *testing.T) { RunRoundTripTests(t, f) })
}

// RunEmptyTests pins the empty-target contract.
func RunEmptyTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("unknown target lists empty, not error", func(t *testing.T) {
		s := f(t)
		got, err := s.List(ctx, target("TKT-none"))
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("empty slice not nil", func(t *testing.T) {
		// Callers marshal this straight to JSON, where nil becomes `null` and
		// an empty slice becomes `[]`. A client distinguishing the two would
		// see different shapes from different backends.
		s := f(t)
		got, err := s.List(ctx, target("TKT-none"))
		require.NoError(t, err)
		require.NotNil(t, got)
	})

	t.Run("DeleteTarget on empty target is not an error", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.DeleteTarget(ctx, target("TKT-none")))
	})
}

// RunOrderingTests pins the ordering contract: oldest first, ID breaking ties.
func RunOrderingTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("oldest first regardless of insertion order", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		// Inserted newest-first so storage order and contract order differ; a
		// backend returning insertion order fails here.
		require.NoError(t, s.Add(ctx, tgt, comment("c3", 2*time.Hour, "alice", "third")))
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "first")))
		require.NoError(t, s.Add(ctx, tgt, comment("c2", time.Hour, "bob", "second")))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Len(t, got, 3)
		require.Equal(t, []string{"c1", "c2", "c3"}, ids(got))
	})

	t.Run("identical timestamps break ties by ID", func(t *testing.T) {
		// A coarse clock can stamp two comments in one tick; without the
		// tie-break the thread would reorder between reads.
		s := f(t)
		tgt := target("TKT-1")
		require.NoError(t, s.Add(ctx, tgt, comment("zzz", 0, "alice", "z")))
		require.NoError(t, s.Add(ctx, tgt, comment("aaa", 0, "alice", "a")))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Equal(t, []string{"aaa", "zzz"}, ids(got))
	})
}

// RunUpdateTests pins which fields an update may change.
func RunUpdateTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("updates body and resolved", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "before")))

		require.NoError(t, s.Update(ctx, tgt, "c1", "after", true))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "after", got[0].Body)
		require.True(t, got[0].Resolved)
	})

	t.Run("does not change author, created_at or anchor", func(t *testing.T) {
		// An edit must not be able to rewrite who said something or what it
		// was about — otherwise "edit your own comment" becomes a way to
		// reattribute someone else's.
		s := f(t)
		tgt := target("TKT-1")
		orig := comment("c1", 0, "alice", "before")
		require.NoError(t, s.Add(ctx, tgt, orig))

		require.NoError(t, s.Update(ctx, tgt, "c1", "after", false))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Equal(t, orig.Author, got[0].Author)
		require.True(t, orig.CreatedAt.Equal(got[0].CreatedAt))
		require.Equal(t, orig.Anchor, got[0].Anchor)
	})

	t.Run("missing comment reports ErrNotFound", func(t *testing.T) {
		s := f(t)
		err := s.Update(ctx, target("TKT-1"), "nope", "x", false)
		require.ErrorIs(t, err, comments.ErrNotFound)
	})
}

// RunDeleteTests pins deletion.
func RunDeleteTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("removes only the named comment", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, tgt, comment("c2", time.Hour, "bob", "two")))

		require.NoError(t, s.Delete(ctx, tgt, "c1"))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Equal(t, []string{"c2"}, ids(got))
	})

	t.Run("missing comment reports ErrNotFound", func(t *testing.T) {
		s := f(t)
		err := s.Delete(ctx, target("TKT-1"), "nope")
		require.ErrorIs(t, err, comments.ErrNotFound)
	})

	t.Run("DeleteTarget removes the whole thread", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, tgt, comment("c2", time.Hour, "bob", "two")))

		require.NoError(t, s.DeleteTarget(ctx, tgt))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// RunIsolationTests pins that targets do not bleed into each other.
func RunIsolationTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("targets are independent", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Add(ctx, target("TKT-1"), comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, target("TKT-2"), comment("c2", 0, "bob", "two")))

		got1, err := s.List(ctx, target("TKT-1"))
		require.NoError(t, err)
		require.Equal(t, []string{"c1"}, ids(got1))

		got2, err := s.List(ctx, target("TKT-2"))
		require.NoError(t, err)
		require.Equal(t, []string{"c2"}, ids(got2))
	})

	t.Run("DeleteTarget leaves other targets alone", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Add(ctx, target("TKT-1"), comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, target("TKT-2"), comment("c2", 0, "bob", "two")))

		require.NoError(t, s.DeleteTarget(ctx, target("TKT-1")))

		got, err := s.List(ctx, target("TKT-2"))
		require.NoError(t, err)
		require.Equal(t, []string{"c2"}, ids(got))
	})

	t.Run("returned slice does not alias stored state", func(t *testing.T) {
		// A backend handing out its backing array lets a caller mutate stored
		// comments in place — impossible for a persistent backend, so tests
		// written against such a store would not transfer.
		s := f(t)
		tgt := target("TKT-1")
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "original")))

		first, err := s.List(ctx, tgt)
		require.NoError(t, err)
		first[0].Body = "mutated"

		second, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Equal(t, "original", second[0].Body)
	})
}

// RunRenameTests pins the re-key path.
//
// This is the behavior that keeps comments reachable across an entity rename:
// the store emits exactly one rename callback, so if this is wrong the comments
// remain on disk under an ID nothing resolves to.
func RunRenameTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("moves comments to the new id", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Add(ctx, target("TKT-old"), comment("c1", 0, "alice", "one")))

		require.NoError(t, s.Rename(ctx, "TKT-old", "TKT-new"))

		moved, err := s.List(ctx, target("TKT-new"))
		require.NoError(t, err)
		require.Equal(t, []string{"c1"}, ids(moved))

		old, err := s.List(ctx, target("TKT-old"))
		require.NoError(t, err)
		require.Empty(t, old)
	})

	t.Run("preserves order across the rename", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-old")
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, tgt, comment("c2", time.Hour, "bob", "two")))

		require.NoError(t, s.Rename(ctx, "TKT-old", "TKT-new"))

		got, err := s.List(ctx, target("TKT-new"))
		require.NoError(t, err)
		require.Equal(t, []string{"c1", "c2"}, ids(got))
	})

	t.Run("renaming a target with no comments is not an error", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Rename(ctx, "TKT-none", "TKT-new"))
	})

	t.Run("merges into an occupied destination", func(t *testing.T) {
		// rela permits ID reuse, so the destination is not guaranteed empty.
		// Merging is the conservative choice: discarding the occupant's
		// comments would destroy data the operator never asked to remove.
		s := f(t)
		require.NoError(t, s.Add(ctx, target("TKT-old"), comment("c1", 0, "alice", "one")))
		require.NoError(t, s.Add(ctx, target("TKT-new"), comment("c2", time.Hour, "bob", "two")))

		require.NoError(t, s.Rename(ctx, "TKT-old", "TKT-new"))

		got, err := s.List(ctx, target("TKT-new"))
		require.NoError(t, err)
		require.Equal(t, []string{"c1", "c2"}, ids(got))
	})
}

// RunConcurrencyTests pins that concurrent writes to one target do not lose
// updates.
//
// This is the read-modify-write hazard: a backend that reads a thread, appends,
// and writes it back without serializing will drop comments under load — and
// will pass every sequential test.
func RunConcurrencyTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("concurrent adds to one target all survive", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")

		const n = 16
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() {
				c := comment(fmt.Sprintf("c%02d", i), time.Duration(i)*time.Minute, "alice", "body")
				errs[i] = s.Add(ctx, tgt, c)
			})
		}
		wg.Wait()
		for _, err := range errs {
			require.NoError(t, err)
		}

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Len(t, got, n, "every concurrent add must survive")
	})

	t.Run("concurrent adds across targets all survive", func(t *testing.T) {
		s := f(t)

		const n = 16
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				tgt := target(fmt.Sprintf("TKT-%02d", i))
				_ = s.Add(ctx, tgt, comment("c1", 0, "alice", "body"))
			})
		}
		wg.Wait()

		for i := range n {
			got, err := s.List(ctx, target(fmt.Sprintf("TKT-%02d", i)))
			require.NoError(t, err)
			require.Len(t, got, 1)
		}
	})
}

// RunRoundTripTests pins that every field survives storage unchanged.
func RunRoundTripTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("all fields round-trip", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		want := comments.Comment{
			ID:        "c1",
			Author:    "alice@example.com",
			CreatedAt: base,
			Anchor:    comments.Anchor{Kind: comments.AnchorSection, Ref: "acceptance-criteria"},
			Body:      "A body with **markdown**, a newline\nand a tab\there.",
			Resolved:  true,
		}
		require.NoError(t, s.Add(ctx, tgt, want))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, want.ID, got[0].ID)
		require.Equal(t, want.Author, got[0].Author)
		require.True(t, want.CreatedAt.Equal(got[0].CreatedAt), "CreatedAt must round-trip")
		require.Equal(t, want.Anchor, got[0].Anchor)
		require.Equal(t, want.Body, got[0].Body)
		require.Equal(t, want.Resolved, got[0].Resolved)
	})

	t.Run("unicode body round-trips", func(t *testing.T) {
		s := f(t)
		tgt := target("TKT-1")
		body := "emoji 🎉, RTL \u202bمرحبا\u202c, CJK 日本語, quote \"x\" and 'y'"
		require.NoError(t, s.Add(ctx, tgt, comment("c1", 0, "alice", body)))

		got, err := s.List(ctx, tgt)
		require.NoError(t, err)
		require.Equal(t, body, got[0].Body)
	})
}

func ids(list []comments.Comment) []string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.ID
	}
	return out
}
