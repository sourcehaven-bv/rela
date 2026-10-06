package bleveindex_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func newTestIndex(t *testing.T) *bleveindex.Index {
	t.Helper()
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestIndex_BasicSearch(t *testing.T) {
	idx := newTestIndex(t)

	e1 := entity.New("REQ-1", "requirement")
	e1.SetString("title", "User authentication")
	e1.Content = "Users must be able to log in with email and password."
	require.NoError(t, idx.EntityPut(e1))

	e2 := entity.New("REQ-2", "requirement")
	e2.SetString("title", "Data export")
	e2.Content = "Users can export their data as CSV."
	require.NoError(t, idx.EntityPut(e2))

	// Search for "authentication" — should find REQ-1.
	ids := searchIDs(t, idx, "authentication", 10)
	assert.Contains(t, ids, "REQ-1")
	assert.NotContains(t, ids, "REQ-2")

	// Search for "export" — should find REQ-2.
	ids = searchIDs(t, idx, "export", 10)
	assert.Contains(t, ids, "REQ-2")
}

func TestIndex_FuzzyMatch(t *testing.T) {
	idx := newTestIndex(t)

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Authentication")
	require.NoError(t, idx.EntityPut(e))

	// Typo: "authentcation" should still match via fuzziness.
	ids := searchIDs(t, idx, "authentcation", 10)
	assert.Contains(t, ids, "REQ-1")
}

func TestIndex_SearchByID(t *testing.T) {
	idx := newTestIndex(t)

	e := entity.New("FEAT-42", "feature")
	e.SetString("title", "Something")
	require.NoError(t, idx.EntityPut(e))

	ids := searchIDs(t, idx, "FEAT-42", 10)
	assert.Contains(t, ids, "FEAT-42")
}

func TestIndex_SearchByIDPrefix(t *testing.T) {
	// A partial ID like `VAD-ACT-` must match every `VAD-ACT-*` entity and
	// rank them above entities that only match incidentally via their
	// title. Before the id-prefix query was added, the keyword `id` field
	// only answered exact full-ID term queries, so this query returned
	// title-scored noise (or nothing) — the rel-picker relevance report.
	idx := newTestIndex(t)

	seed := []struct{ id, title string }{
		{"PRS-ACT-BPHC", "Handelende organisatie"},
		{"PRS-ACT-DWPM", "Afnemende organisatie"},
		{"VAD-ACT-6P4X", "Gegevensuitwisselingspartners"},
		{"VAD-ACT-CV83", "VAD-realisator-burger-authenticatie"},
		{"VAD-ACT-F7A4", "VAD-realisator-beveiligd"},
	}
	for _, s := range seed {
		e := entity.New(s.id, "actor")
		e.SetString("title", s.title)
		require.NoError(t, idx.EntityPut(e))
	}

	ids := searchIDs(t, idx, "VAD-ACT-", 20)

	// All three VAD-ACT-* entities are found...
	assert.Subset(t, ids, []string{"VAD-ACT-6P4X", "VAD-ACT-CV83", "VAD-ACT-F7A4"})
	// ...and rank ahead of any PRS-ACT-* that only matched on title tokens.
	// The first three hits are exactly the id-prefix matches, in some order.
	require.GreaterOrEqual(t, len(ids), 3)
	assert.ElementsMatch(t,
		[]string{"VAD-ACT-6P4X", "VAD-ACT-CV83", "VAD-ACT-F7A4"},
		ids[:3],
		"id-prefix matches must occupy the top ranks, got %v", ids)
}

func TestIndex_SearchByIDPrefix_CaseSensitiveToken(t *testing.T) {
	// The prefix query runs against the unanalyzed (case-sensitive)
	// keyword token, but real callers may type lower-case. Guard that the
	// common upper-case ID prefix path works; lower-case partial-ID search
	// is a known limitation handled by the caller, not asserted here.
	idx := newTestIndex(t)
	e := entity.New("TKT-ABCD", "ticket")
	e.SetString("title", "Example")
	require.NoError(t, idx.EntityPut(e))

	ids := searchIDs(t, idx, "TKT-", 10)
	assert.Contains(t, ids, "TKT-ABCD")
}

func TestIndex_SearchByProperty(t *testing.T) {
	idx := newTestIndex(t)

	e := entity.New("T-1", "ticket")
	e.SetString("status", "critical")
	require.NoError(t, idx.EntityPut(e))

	ids := searchIDs(t, idx, "critical", 10)
	assert.Contains(t, ids, "T-1")
}

func TestIndex_Remove(t *testing.T) {
	idx := newTestIndex(t)

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Removable")
	require.NoError(t, idx.EntityPut(e))

	require.NoError(t, idx.EntityDelete("REQ-1"))

	ids := searchIDs(t, idx, "Removable", 10)
	assert.Empty(t, ids)
}

func TestIndex_EntityRenamed(t *testing.T) {
	// EntityRenamed must atomically drop the old key and index the
	// new content. After the rename: a search for the title still
	// finds the entity, but only under newID — the oldID must be
	// gone from the index.
	idx := newTestIndex(t)

	old := entity.New("REQ-1", "requirement")
	old.SetString("title", "Atomic rename")
	require.NoError(t, idx.EntityPut(old))

	renamed := entity.New("REQ-99", "requirement")
	renamed.SetString("title", "Atomic rename")
	require.NoError(t, idx.EntityRenamed("REQ-1", renamed))

	ids := searchIDs(t, idx, "Atomic rename", 10)
	assert.Contains(t, ids, "REQ-99",
		"renamed entity should be findable under the new ID")
	assert.NotContains(t, ids, "REQ-1",
		"old ID must be gone from the index after rename")
}

func TestIndex_UpdateOverwrites(t *testing.T) {
	idx := newTestIndex(t)

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Old title")
	require.NoError(t, idx.EntityPut(e))

	e.SetString("title", "New title")
	require.NoError(t, idx.EntityPut(e))

	// Old content should not match.
	ids := searchIDs(t, idx, "Old", 10)
	assert.Empty(t, ids)

	// New content should match.
	ids = searchIDs(t, idx, "New", 10)
	assert.Contains(t, ids, "REQ-1")
}

func TestIndex_IndexBatch(t *testing.T) {
	idx := newTestIndex(t)

	entities := []*entity.Entity{
		entity.New("REQ-1", "requirement"),
		entity.New("REQ-2", "requirement"),
		entity.New("REQ-3", "requirement"),
	}
	entities[0].SetString("title", "Alpha")
	entities[1].SetString("title", "Beta")
	entities[2].SetString("title", "Gamma")

	count, err := idx.IndexBatch(entities)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	for _, e := range entities {
		ids := searchIDs(t, idx, e.Properties["title"].(string), 10)
		assert.Contains(t, ids, e.ID)
	}
}

func TestIndex_IndexBatch_Empty(t *testing.T) {
	idx := newTestIndex(t)

	count, err := idx.IndexBatch(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	count, err = idx.IndexBatch([]*entity.Entity{})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestIndex_Limit(t *testing.T) {
	idx := newTestIndex(t)

	for i := range 10 {
		e := entity.New("T-"+string(rune('A'+i)), "ticket")
		e.SetString("title", "Common keyword")
		require.NoError(t, idx.EntityPut(e))
	}

	ids := searchIDs(t, idx, "common", 3)
	assert.Len(t, ids, 3)
}

func TestIndex_EmptySearch(t *testing.T) {
	idx := newTestIndex(t)

	ids := searchIDs(t, idx, "", 10)
	assert.Nil(t, ids)

	ids = searchIDs(t, idx, "   ", 10)
	assert.Nil(t, ids)
}

func TestIndex_WildcardSearch(t *testing.T) {
	idx := newTestIndex(t)

	e1 := entity.New("REQ-1", "requirement")
	e1.SetString("title", "Authentication flow")
	require.NoError(t, idx.EntityPut(e1))

	e2 := entity.New("REQ-2", "requirement")
	e2.SetString("title", "Authorization rules")
	require.NoError(t, idx.EntityPut(e2))

	ids := searchIDs(t, idx, "auth*", 10)
	assert.Contains(t, ids, "REQ-1")
	assert.Contains(t, ids, "REQ-2")
}

func TestNew_PersistentIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "search.bleve")

	// Create and populate.
	idx, err := bleveindex.New(path)
	require.NoError(t, err)

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Persistent search")
	require.NoError(t, idx.EntityPut(e))
	require.NoError(t, idx.Close())

	// Reopen — data should survive.
	idx2, err := bleveindex.New(path)
	require.NoError(t, err)
	defer idx2.Close()

	ids := searchIDs(t, idx2, "persistent", 10)
	assert.Contains(t, ids, "REQ-1")
}

func TestNew_CorruptedIndexRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "search.bleve")

	// Create a valid index, populate it, and close.
	idx, err := bleveindex.New(path)
	require.NoError(t, err)

	e := entity.New("REQ-1", "requirement")
	e.SetString("title", "Will be lost")
	require.NoError(t, idx.EntityPut(e))
	require.NoError(t, idx.Close())

	// Corrupt the index by overwriting a key file with garbage.
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, entry := range entries {
		if !entry.IsDir() {
			require.NoError(t, os.WriteFile(filepath.Join(path, entry.Name()), []byte("corrupted"), 0644))
		}
	}

	// Reopen — should recover by recreating a fresh index.
	idx2, err := bleveindex.New(path)
	require.NoError(t, err, "should recover from corrupted index")
	defer idx2.Close()

	// Old data is gone (index was recreated), but the index works.
	ids := searchIDs(t, idx2, "lost", 10)
	assert.Empty(t, ids, "old data should not survive corruption recovery")

	// Can index new data.
	e2 := entity.New("REQ-2", "requirement")
	e2.SetString("title", "Fresh start")
	require.NoError(t, idx2.EntityPut(e2))

	ids = searchIDs(t, idx2, "fresh", 10)
	assert.Contains(t, ids, "REQ-2")
}

// searchIDs runs a DEFAULT-WORLD search and returns just the ids, which is
// what the pre-worlds assertions in this file are written against. World
// behavior has its own tests; these keep asserting the properties that hold
// regardless of worlds (ranking, limits, rename/delete eviction).
func searchIDs(t *testing.T, idx *bleveindex.Index, text string, limit int) []string {
	t.Helper()
	faces, err := idx.Search(text, limit, store.TrivialScope())
	if err != nil {
		t.Fatalf("Search(%q, %d): %v", text, limit, err)
	}
	if faces == nil {
		// Preserve the nil-vs-empty distinction the callers assert on: an
		// empty query returns NO result set, which is not the same shape as
		// a query that ran and matched nothing.
		return nil
	}
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	return ids
}

// faceOf builds one face of a procedure, as a store hands it to the index.
func faceOf(id string, face entity.Face, title string) *entity.Entity {
	e := entity.New(id, "procedure")
	e.Face = face
	e.SetString("title", title)
	return e
}

// currentWorld serves the adopted face of a procedure, else its concept.
var currentWorld = store.NewWorldScope(map[string]store.TypeResolution{
	"procedure": {Chain: []entity.Face{"adopted", "concept"}, Fallback: store.FallbackDefaultState},
})

// A batch is what the startup backfill writes. It must key each face as
// EntityPut does, or a world search drops every faced hit until the entity is
// written again (BUG-VYPK9N).
func TestIndex_IndexBatch_KeysEachFace(t *testing.T) {
	batch := []*entity.Entity{
		faceOf("PROC-1", "concept", "Offboarding revised"),
		faceOf("PROC-1", "adopted", "Offboarding"),
		faceOf("PROC-2", "concept", "Onboarding"),
	}
	type hit struct {
		id   string
		face entity.Face
	}
	search := func(t *testing.T, idx *bleveindex.Index, text string) []hit {
		t.Helper()
		faces, err := idx.Search(text, 10, currentWorld)
		require.NoError(t, err)
		out := make([]hit, 0, len(faces))
		for _, f := range faces {
			out = append(out, hit{f.ID, f.Face})
		}
		return out
	}

	batched := newTestIndex(t)
	n, err := batched.IndexBatch(batch)
	require.NoError(t, err)
	require.Equal(t, 3, n)

	put := newTestIndex(t)
	for _, e := range batch {
		require.NoError(t, put.EntityPut(e))
	}

	for _, tc := range []struct {
		text string
		want []hit
	}{
		{"Offboarding", []hit{{"PROC-1", "adopted"}}},
		{"revised", nil},
		{"Onboarding", []hit{{"PROC-2", "concept"}}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			want := tc.want
			if want == nil {
				want = []hit{}
			}
			assert.Equal(t, want, search(t, batched, tc.text), "batch-indexed")
			assert.Equal(t, want, search(t, put, tc.text), "put-indexed")
		})
	}
}

// An index written in another document format is emptied on open, so the
// caller's backfill rewrites it rather than trusting stale keys.
func TestNew_EmptiesAnIndexOfAnotherFormat(t *testing.T) {
	for _, tc := range []struct{ name, format string }{
		{"unversioned", ""},
		{"older version", "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "search.bleve")

			idx, err := bleveindex.New(path)
			require.NoError(t, err)
			stale := make([]*entity.Entity, 0, clearPageOverflow)
			for i := range clearPageOverflow {
				e := entity.New(fmt.Sprintf("REQ-%d", i), "requirement")
				e.SetString("title", "Stale document")
				stale = append(stale, e)
			}
			_, err = idx.IndexBatch(stale)
			require.NoError(t, err)
			require.NoError(t, bleveindex.SetFormat(idx, tc.format))
			require.NoError(t, idx.Close())

			idx2, err := bleveindex.New(path)
			require.NoError(t, err)
			count, err := idx2.DocCount()
			require.NoError(t, err)
			assert.Zero(t, count, "an index of another format is emptied")
			assert.True(t, idx2.LastModified().IsZero(), "its LastModified is cleared")

			// The emptied index carries the current format, so its new
			// documents survive a reopen.
			e := entity.New("REQ-NEW", "requirement")
			e.SetString("title", "Fresh document")
			require.NoError(t, idx2.EntityPut(e))
			require.NoError(t, idx2.Close())

			idx3, err := bleveindex.New(path)
			require.NoError(t, err)
			t.Cleanup(func() { idx3.Close() })
			assert.Equal(t, []string{"REQ-NEW"}, searchIDs(t, idx3, "fresh", 10))
		})
	}
}

// clearPageOverflow is more documents than one clearing pass deletes, so the
// test covers the loop.
const clearPageOverflow = 1001
