package sqlitecomments_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/commentstest"
	"github.com/Sourcehaven-BV/rela/internal/comments/sqlitecomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestConformance runs the shared backend contract against SQLite.
func TestConformance(t *testing.T) {
	commentstest.RunAll(t, func(t *testing.T) comments.Store {
		t.Helper()
		return newStore(t, filepath.Join(t.TempDir(), "rela.db"))
	})
}

// TestKeyFidelity holds this backend to byte-exact target keys, which RunAll
// deliberately leaves out: filecomments cannot promise case-sensitivity on a
// case-insensitive filesystem and refuses non-ASCII ids outright, both
// reasonably. A database backend has neither excuse.
func TestKeyFidelity(t *testing.T) {
	commentstest.RunKeyFidelityTests(t, func(t *testing.T) comments.Store {
		t.Helper()
		return newStore(t, filepath.Join(t.TempDir(), "rela.db"))
	})
}

// TestNewRejectsNilHandle pins the constructor's nil contract: a wiring mistake
// must fail at construction, not at the first comment someone posts.
func TestNewRejectsNilHandle(t *testing.T) {
	_, err := sqlitecomments.New(nil)
	require.Error(t, err)
}

// TestOrderingWithSubSecondTimestamps is the regression test for the timestamp
// format.
//
// List orders in SQL as a STRING compare. sqlitestore's RFC3339Nano gets this
// wrong: Go omits the fractional part entirely at a whole second and strips
// trailing zeros otherwise, so "12:00:00Z" sorts AFTER "12:00:00.000000005Z"
// ('Z' is 0x5A, '.' is 0x2E) and a thread silently reorders between reads. The
// fixed-width format this package uses is what makes lexical and chronological
// order coincide.
func TestOrderingWithSubSecondTimestamps(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, filepath.Join(t.TempDir(), "rela.db"))

	target := comments.Target{Type: "ticket", ID: "TKT-1"}
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	// Every adjacent pair here is mis-sorted by RFC3339Nano's variable width.
	at := []time.Time{
		base,                             // renders with no fractional part
		base.Add(5 * time.Nanosecond),    // .000000005
		base.Add(100 * time.Millisecond), // ".1" under RFC3339Nano
		base.Add(120 * time.Millisecond), // ".12" under RFC3339Nano
		base.Add(time.Second),            // whole second again
		base.Add(1500 * time.Millisecond),
	}
	want := []string{"c0", "c1", "c2", "c3", "c4", "c5"}

	// Inserted newest-first, so storage order and contract order differ.
	for i := len(at) - 1; i >= 0; i-- {
		require.NoError(t, store.Add(ctx, target, comments.Comment{
			ID:        want[i],
			Author:    "alice",
			CreatedAt: at[i],
			Anchor:    comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
			Body:      "body",
		}))
	}

	got, err := store.List(ctx, target)
	require.NoError(t, err)
	require.Equal(t, want, idsOf(got), "sub-second timestamps must sort chronologically")
}

// TestCommentsSurviveReopen proves commentary travels with the database file,
// which is why this backend exists rather than leaving sqlite on filecomments:
// an operator copying or shipping rela.db gets the remarks along with the rows.
func TestCommentsSurviveReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rela.db")
	target := comments.Target{Type: "ticket", ID: "TKT-1"}

	first, firstDB := openStore(t, path)
	require.NoError(t, first.Add(ctx, target, comments.Comment{
		ID:        "c1",
		Author:    "alice",
		CreatedAt: time.Now().UTC(),
		Anchor:    comments.Anchor{Kind: comments.AnchorSection, Ref: "acceptance-criteria"},
		Body:      "ships with the file",
	}))
	// Close before reopening: the single-writer lock refuses a second opener.
	require.NoError(t, firstDB.Close())

	second, _ := openStore(t, path)
	got, err := second.List(ctx, target)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ships with the file", got[0].Body)
}

// TestRenameDoesNotMatchLikeWildcards pins the escaping in facePrefixPattern.
//
// An entity id may contain "_" (entity.ValidateID admits [A-Za-z0-9_-]), which
// is LIKE's single-character wildcard. Unescaped, renaming "TKT_1" would also
// re-key "TKT-1" and silently merge two unrelated threads.
func TestRenameDoesNotMatchLikeWildcards(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, filepath.Join(t.TempDir(), "rela.db"))

	add := func(id string, face entity.Face, commentID string) {
		require.NoError(t, store.Add(ctx,
			comments.Target{Type: "ticket", ID: id, Face: face},
			comments.Comment{
				ID:        commentID,
				Author:    "alice",
				CreatedAt: time.Now().UTC(),
				Anchor:    comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
				Body:      "body",
			}))
	}
	// Faced, so both threads are reachable only through the prefix pattern.
	add("TKT_1", "draft", "under")
	add("TKT-1", "draft", "hyphen")

	require.NoError(t, store.Rename(ctx, "TKT_1", "TKT-9"))

	untouched, err := store.List(ctx, comments.Target{Type: "ticket", ID: "TKT-1", Face: "draft"})
	require.NoError(t, err)
	require.Equal(t, []string{"hyphen"}, idsOf(untouched),
		"renaming TKT_1 must not re-key TKT-1: _ is a LIKE wildcard")

	moved, err := store.List(ctx, comments.Target{Type: "ticket", ID: "TKT-9", Face: "draft"})
	require.NoError(t, err)
	require.Equal(t, []string{"under"}, idsOf(moved))
}

// newStore opens a database at path and returns a comment store over it.
func newStore(t *testing.T, path string) comments.Store {
	t.Helper()
	store, _ := openStore(t, path)
	return store
}

// openStore also returns the database handle, for tests that need to close it.
func openStore(t *testing.T, path string) (comments.Store, *sqlitedb.DB) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	// Closed via the handle when a test needs an explicit reopen; registered
	// here too so a failing test still releases the single-writer lock. Close
	// is idempotent enough for that (the second call reports an already-closed
	// pool, which nothing here asserts on).
	t.Cleanup(func() { _ = db.Close() })

	store, err := sqlitecomments.New(db.DB())
	require.NoError(t, err)
	return store, db
}

func idsOf(list []comments.Comment) []string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.ID
	}
	return out
}
