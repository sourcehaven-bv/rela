package bleveindex_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestMatchedFields_AgreesWithTheIndex uses the index as the oracle. Each
// case indexes one entity that holds the value in one property only, then
// checks that MatchedFields reports the property exactly when the index
// returns the entity. Reporting a field the index did not match would let a
// field scope return a hit whose real match is a hidden property; missing
// one would drop a hit the reader may see.
func TestMatchedFields_AgreesWithTheIndex(t *testing.T) {
	for _, tc := range []struct{ value, query string }{
		{"foobar", "foo,bar"},      // the raw word is one edit from the token
		{"foo visible", "foo,bar"}, // analyzer tokens of the word would match; the raw word does not
		{"xxfoobar", "foobar"},     // a substring, but two edits away
		{"foobaz", "foobar"},
		{"foo", "fo*"},
		{"foo", "f?o"},
		{"foo", "fo?"},
		{"café", "cafe"}, // one rune edit, two byte edits
		{"naïve", "naive"},
		{"FOO bar", "foo"},
		{"the cat", "the"}, // a stop word is not a token
		{"İstanbul", "istanbul"},
		{"a+b", "a+b"},
		{"x.y", "x.?"},
	} {
		t.Run(tc.value+"/"+tc.query, func(t *testing.T) {
			idx := newTestIndex(t)
			e := entity.New("QQ-7", "thing")
			e.SetString("code", tc.value)
			require.NoError(t, idx.EntityPut(e))

			faces, err := idx.Search(tc.query, 10, store.TrivialScope())
			require.NoError(t, err)
			_, reported := idx.MatchedFields(e, tc.query)[search.PropFieldPrefix+"code"]
			require.Equal(t, len(faces) == 1, reported,
				"index returned %d hits; MatchedFields reported the field: %v", len(faces), reported)
		})
	}
}

// TestMatchedFields_IDTokens: the "all" field indexes the id's analyzed
// tokens, so a word matching one is an id match, not a property match.
func TestMatchedFields_IDTokens(t *testing.T) {
	idx := newTestIndex(t)
	e := entity.New("VAD-ACT-6P4X", "thing")
	e.SetString("code", "secret")
	require.NoError(t, idx.EntityPut(e))
	faces, err := idx.Search("act", 10, store.TrivialScope())
	require.NoError(t, err)
	require.Len(t, faces, 1)
	require.Equal(t, map[string]struct{}{search.FieldID: {}}, idx.MatchedFields(e, "act"))
}
