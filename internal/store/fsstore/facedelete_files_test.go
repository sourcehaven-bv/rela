package fsstore_test

import (
	"context"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// TestFsFaceDeleteTouchesOnlyItsOwnFiles is the FILESYSTEM-level check that
// the conformance suite cannot make: fsstore is the backend where a face's
// identity is part of a FILENAME rather than a column, so "delete one face"
// is a question about which files disappear.
//
// Two hazards it pins, both the fs analog of the tail-dropping bug:
//
//   - Deleting PAGE-1@draft must not disturb PAGE-1.md. The names share a
//     prefix, so any prefix-matching removal would take both.
//   - Relation files carry the tail in the FROM slot
//     (PAGE-1@draft--references--SPEC-1.md), so they must be matched by TAIL,
//     not by prefix — otherwise deleting a face takes the default face's edge
//     file with it.
//
// Asserting the exact SET of removed files, rather than just that the face is
// gone, is what makes this capable of failing: an over-broad removal still
// leaves the queried face absent and would pass aexistence-only check.
func TestFsFaceDeleteTouchesOnlyItsOwnFiles(t *testing.T) {
	memfs := storage.NewMemFS()
	s, err := fsstore.New(newConfig(memfs))
	require.NoError(t, err)
	ctx := context.Background()

	mk := func(id string, p entity.Face) {
		require.NoError(t, s.CreateEntity(ctx,
			&entity.Entity{ID: id, Type: "page", Face: p}))
	}
	mk("PAGE-1", "")
	mk("PAGE-1", "draft")
	mk("SPEC-1", "")

	draft := entity.Face("draft")
	_, err = s.CreateRelation(ctx, "PAGE-1", "references", "SPEC-1", nil)
	require.NoError(t, err)
	_, err = s.CreateRelation(ctx, "PAGE-1", "references", "SPEC-1",
		&store.RelationData{FromFace: draft})
	require.NoError(t, err)

	files := func() []string {
		var out []string
		require.NoError(t, memfs.Walk("/", func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() || !strings.HasSuffix(p, ".md") {
				return nil
			}
			out = append(out, p)
			return nil
		}))
		sort.Strings(out)
		return out
	}
	before := files()
	t.Logf("BEFORE: %v", before)

	_, err = s.DeleteEntityState(ctx, "PAGE-1", draft)
	require.NoError(t, err)
	after := files()
	t.Logf("AFTER:  %v", after)

	gone := map[string]bool{}
	for _, f := range before {
		gone[f] = true
	}
	for _, f := range after {
		delete(gone, f)
	}
	var removed []string
	for f := range gone {
		removed = append(removed, f)
	}
	sort.Strings(removed)
	t.Logf("REMOVED: %v", removed)

	var keptDefault bool
	for _, f := range after {
		if strings.HasSuffix(f, "/PAGE-1.md") {
			keptDefault = true
		}
	}
	if !keptDefault {
		t.Error("PAGE-1.md was removed by a delete of PAGE-1@draft")
	}
	if len(removed) != 2 {
		t.Errorf("want exactly 2 files removed (the face + its own tail edge), got %v", removed)
	}
}
