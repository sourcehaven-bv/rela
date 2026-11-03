package channels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckWaitingQuery pins the argument checks of ListWaiting: each
// refusal is ErrInvalidKey.
func TestCheckWaitingQuery(t *testing.T) {
	ok := map[string][]string{"room": {"asked", ""}}
	anchors := []Anchor{{ID: "T-1"}}
	tests := []struct {
		name  string
		q     WaitingQuery
		valid bool
	}{
		{"anchors", WaitingQuery{States: ok, Anchors: anchors}, true},
		{"creator", WaitingQuery{States: ok, CreatorTypes: []string{"room"}, CreatedBy: "alice"}, true},
		{"no states", WaitingQuery{Anchors: anchors}, false},
		{"bad type", WaitingQuery{States: map[string][]string{"bad type!": {"asked"}}, Anchors: anchors}, false},
		{"type without states", WaitingQuery{States: map[string][]string{"room": {}}, Anchors: anchors}, false},
		{"bad state", WaitingQuery{States: map[string][]string{"room": {"bad state!"}}, Anchors: anchors}, false},
		{"nothing to match", WaitingQuery{States: ok, CreatorTypes: []string{"room"}}, false},
		{"bad anchor", WaitingQuery{States: ok, Anchors: []Anchor{{ID: "bad id!"}}}, false},
		{"bad creator type", WaitingQuery{
			States: ok, Anchors: anchors, CreatorTypes: []string{"bad type!"}, CreatedBy: "alice",
		}, false},
		{"too many anchors", WaitingQuery{States: ok, Anchors: make([]Anchor, MaxWaitingAnchors+1)}, false},
		{"row ceiling", WaitingQuery{States: ok, Anchors: anchors, MaxRows: 5}, true},
		{"negative row ceiling", WaitingQuery{States: ok, Anchors: anchors, MaxRows: -1}, false},
		{"row ceiling too high", WaitingQuery{States: ok, Anchors: anchors, MaxRows: MaxWaitingRows + 1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckWaitingQuery(tc.q)
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}

// TestWaitingQuery_Matches pins the row test the naive backends share.
func TestWaitingQuery_Matches(t *testing.T) {
	q := WaitingQuery{
		States:       map[string][]string{"room": {"asked"}, "thread": {""}},
		CreatorTypes: []string{"thread"}, CreatedBy: "alice",
	}
	held := map[Anchor]bool{{ID: "T-1"}: true}
	row := func(id, typ, state, by string, st State) Channel {
		return Channel{
			Key: Key{Anchor: Anchor{ID: id}, Type: typ}, State: st,
			Session: Session{State: state}, CreatedBy: Author{User: by},
		}
	}
	tests := []struct {
		name string
		c    Channel
		want bool
	}{
		{"held anchor", row("T-1", "room", "asked", "bob", StateOpen), true},
		{"other anchor", row("T-2", "room", "asked", "bob", StateOpen), false},
		{"closed", row("T-1", "room", "asked", "bob", StateClosed), false},
		{"unlisted state", row("T-1", "room", "done", "bob", StateOpen), false},
		{"unlisted type", row("T-1", "chat", "asked", "bob", StateOpen), false},
		{"creator", row("T-2", "thread", "", "alice", StateOpen), true},
		{"other creator", row("T-2", "thread", "", "bob", StateOpen), false},
	}
	for _, tc := range tests {
		require.Equal(t, tc.want, q.Matches(tc.c, held), tc.name)
	}
	require.False(t, WaitingQuery{CreatorTypes: []string{"thread"}}.CreatorBranch())
	require.ErrorIs(t, TooManyWaiting(WaitingQuery{MaxRows: 3}), ErrTooManyRows)
	require.Equal(t, MaxWaitingRows, WaitingQuery{}.RowCeiling())
	require.Equal(t, 3, WaitingQuery{MaxRows: 3}.RowCeiling())
}
