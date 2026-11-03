package channels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckGetMessages(t *testing.T) {
	t.Parallel()
	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	tooMany := make([]MessageRef, MaxGetMessages+1)
	for i := range tooMany {
		tooMany[i] = MessageRef{Key: k, Seq: int64(i + 1)}
	}
	tests := []struct {
		name string
		refs []MessageRef
		ok   bool
	}{
		{name: "none", ok: true},
		{name: "valid", refs: []MessageRef{{Key: k, Seq: 1}}, ok: true},
		{name: "at the cap", refs: tooMany[:MaxGetMessages], ok: true},
		{name: "over the cap", refs: tooMany},
		{name: "zero seq", refs: []MessageRef{{Key: k}}},
		{name: "negative seq", refs: []MessageRef{{Key: k, Seq: -1}}},
		{name: "invalid key", refs: []MessageRef{{Key: Key{Type: "room"}, Seq: 1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckGetMessages(tc.refs)
			if tc.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}

func TestGroupRefs(t *testing.T) {
	t.Parallel()
	a := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	b := Key{Anchor: Anchor{ID: "TKT-2"}, Type: "room"}
	keys, seqs := GroupRefs([]MessageRef{{Key: b, Seq: 3}, {Key: a, Seq: 2}, {Key: b, Seq: 1}, {Key: b, Seq: 3}})
	require.Equal(t, []Key{b, a}, keys)
	require.Equal(t, map[Key][]int64{b: {3, 1}, a: {2}}, seqs)

	keys, seqs = GroupRefs(nil)
	require.Empty(t, keys)
	require.Empty(t, seqs)
}
