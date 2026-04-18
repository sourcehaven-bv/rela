package storeutil_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func TestValidateID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr string
	}{
		{"empty", "", "empty ID"},
		{"consecutive dashes", "FOO--BAR", "consecutive dashes"},
		{"forward slash", "FOO/BAR", "path separator"},
		{"backslash", "FOO\\BAR", "path separator"},
		{"NUL byte", "FOO\x00BAR", "control character"},
		{"tab", "FOO\tBAR", "control character"},
		{"newline", "FOO\nBAR", "control character"},
		{"DEL", "FOO\x7fBAR", "control character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := storeutil.ValidateID(tc.id)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}

	t.Run("accepts plain ASCII", func(t *testing.T) {
		assert.NoError(t, storeutil.ValidateID("FOO-BAR"))
		assert.NoError(t, storeutil.ValidateID("TKT-001"))
		assert.NoError(t, storeutil.ValidateID("a"))
	})

	t.Run("accepts multi-byte UTF-8", func(t *testing.T) {
		// Continuation bytes are 0x80+ so they never register as control chars.
		assert.NoError(t, storeutil.ValidateID("café"))
		assert.NoError(t, storeutil.ValidateID("日本語"))
	})
}

func TestValidateProperty(t *testing.T) {
	t.Run("rejects empty", func(t *testing.T) {
		err := storeutil.ValidateProperty("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty property name")
	})

	t.Run("rejects slash", func(t *testing.T) {
		err := storeutil.ValidateProperty("foo/bar")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "slash")
	})

	t.Run("accepts normal names", func(t *testing.T) {
		assert.NoError(t, storeutil.ValidateProperty("screenshot"))
		assert.NoError(t, storeutil.ValidateProperty("spec-sheet"))
	})
}

func TestSortedInsert(t *testing.T) {
	t.Run("into empty", func(t *testing.T) {
		got := storeutil.SortedInsert(nil, "b")
		assert.Equal(t, []string{"b"}, got)
	})

	t.Run("at start", func(t *testing.T) {
		got := storeutil.SortedInsert([]string{"b", "c"}, "a")
		assert.Equal(t, []string{"a", "b", "c"}, got)
	})

	t.Run("in middle", func(t *testing.T) {
		got := storeutil.SortedInsert([]string{"a", "c"}, "b")
		assert.Equal(t, []string{"a", "b", "c"}, got)
	})

	t.Run("at end", func(t *testing.T) {
		got := storeutil.SortedInsert([]string{"a", "b"}, "c")
		assert.Equal(t, []string{"a", "b", "c"}, got)
	})
}

func TestSortedRemove(t *testing.T) {
	t.Run("from middle", func(t *testing.T) {
		got := storeutil.SortedRemove([]string{"a", "b", "c"}, "b")
		assert.Equal(t, []string{"a", "c"}, got)
	})

	t.Run("only element", func(t *testing.T) {
		got := storeutil.SortedRemove([]string{"x"}, "x")
		assert.Empty(t, got)
	})

	t.Run("panics on missing key", func(t *testing.T) {
		assert.PanicsWithValue(t,
			"storeutil: SortedRemove called with missing key: z",
			func() { storeutil.SortedRemove([]string{"a", "b"}, "z") })
	})
}

func TestCursorRoundTrip(t *testing.T) {
	cases := []string{"", "key", "TKT-001", "with spaces", "日本語", "a/b\\c"}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			cursor := storeutil.EncodeCursor(key)
			got, err := storeutil.DecodeCursor(cursor)
			require.NoError(t, err)
			assert.Equal(t, key, got)
		})
	}

	t.Run("empty cursor decodes to empty", func(t *testing.T) {
		got, err := storeutil.DecodeCursor("")
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("invalid cursor errors", func(t *testing.T) {
		_, err := storeutil.DecodeCursor("!!not-base64!!")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cursor")
	})
}

func TestPaginateSortedKeys(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "e"}
	matchAll := func(string) bool { return true }

	t.Run("no limit returns all", func(t *testing.T) {
		page := storeutil.PaginateSortedKeys(keys, "", 0, matchAll)
		assert.Equal(t, keys, page.Keys)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("limit with more results sets cursor", func(t *testing.T) {
		page := storeutil.PaginateSortedKeys(keys, "", 2, matchAll)
		assert.Equal(t, []string{"a", "b"}, page.Keys)
		got, err := storeutil.DecodeCursor(page.NextCursor)
		require.NoError(t, err)
		assert.Equal(t, "b", got)
	})

	t.Run("limit on exact last page leaves cursor empty", func(t *testing.T) {
		page := storeutil.PaginateSortedKeys(keys, "", 5, matchAll)
		assert.Equal(t, keys, page.Keys)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("cursor skips emitted key", func(t *testing.T) {
		page := storeutil.PaginateSortedKeys(keys, "b", 2, matchAll)
		assert.Equal(t, []string{"c", "d"}, page.Keys)
	})

	t.Run("cursor past end yields empty page", func(t *testing.T) {
		page := storeutil.PaginateSortedKeys(keys, "z", 10, matchAll)
		assert.Empty(t, page.Keys)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("filters via match", func(t *testing.T) {
		vowels := func(k string) bool { return strings.Contains("aeiou", k) }
		page := storeutil.PaginateSortedKeys(keys, "", 0, vowels)
		assert.Equal(t, []string{"a", "e"}, page.Keys)
	})

	t.Run("non-present cursor resumes at next key", func(t *testing.T) {
		// "bb" isn't in keys but sorts between "b" and "c"; resume from "c".
		page := storeutil.PaginateSortedKeys(keys, "bb", 2, matchAll)
		assert.Equal(t, []string{"c", "d"}, page.Keys)
	})
}

func TestMatchRelation(t *testing.T) {
	rel := &entity.Relation{From: "A", Type: "links", To: "B"}

	cases := []struct {
		name  string
		query store.RelationQuery
		want  bool
	}{
		{"empty query matches", store.RelationQuery{}, true},
		{"type match", store.RelationQuery{Type: "links"}, true},
		{"type mismatch", store.RelationQuery{Type: "other"}, false},
		{"from match", store.RelationQuery{From: "A"}, true},
		{"from mismatch", store.RelationQuery{From: "X"}, false},
		{"to match", store.RelationQuery{To: "B"}, true},
		{"to mismatch", store.RelationQuery{To: "X"}, false},
		{"entity outgoing match", store.RelationQuery{EntityID: "A", Direction: store.DirectionOutgoing}, true},
		{"entity outgoing mismatch", store.RelationQuery{EntityID: "B", Direction: store.DirectionOutgoing}, false},
		{"entity incoming match", store.RelationQuery{EntityID: "B", Direction: store.DirectionIncoming}, true},
		{"entity incoming mismatch", store.RelationQuery{EntityID: "A", Direction: store.DirectionIncoming}, false},
		{"entity both (from)", store.RelationQuery{EntityID: "A", Direction: store.DirectionBoth}, true},
		{"entity both (to)", store.RelationQuery{EntityID: "B", Direction: store.DirectionBoth}, true},
		{"entity both mismatch", store.RelationQuery{EntityID: "Z", Direction: store.DirectionBoth}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, storeutil.MatchRelation(rel, tc.query))
		})
	}
}

// Sanity check that ValidateID errors compose with errors.Is-style wrapping
// if a caller ever needs it — the function returns plain fmt.Errorf errors,
// so equality by message is the current contract.
func TestValidateIDErrorShape(t *testing.T) {
	err := storeutil.ValidateID("")
	require.Error(t, err)
	assert.False(t, errors.Is(err, store.ErrNotFound),
		"ValidateID errors must not collide with store sentinels")
}
