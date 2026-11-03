package channelstest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// waitingTests are the inbox listing tests (TKT-ZZ4PNX), registered in
// [RunAll].
var waitingTests = []conformanceTest{
	{"ListWaiting", testListWaiting},
	{"ListWaitingCreator", testListWaitingCreator},
	{"ListWaitingChunks", testListWaitingChunks},
	{"ListWaitingNames", testListWaitingNames},
	{"ListWaitingInvalid", testListWaitingInvalid},
}

// waitStates lists room in contained and unmoved, topic and thread
// unmoved.
func waitStates() map[string][]string {
	return map[string][]string{"room": {"", "contained"}, "topic": {""}, "thread": {""}}
}

// listWaitingKeys runs q and returns the keys, failing on an error.
func listWaitingKeys(t *testing.T, s channels.Store, q channels.WaitingQuery) []channels.Key {
	t.Helper()
	got, err := s.ListWaiting(context.Background(), q)
	require.NoError(t, err)
	keys := make([]channels.Key, len(got))
	for i, c := range got {
		keys[i] = c.Key
		require.Equal(t, channels.StateOpen, c.State)
	}
	return keys
}

// faced is the key of typ on id@face.
func faced(id string, face entity.Face, typ string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id, Face: face}, Type: typ}
}

// setState moves k from open to each of to in turn.
func setState(t *testing.T, s channels.Store, k channels.Key, to ...channels.State) {
	t.Helper()
	from := channels.StateOpen
	for _, st := range to {
		_, err := s.SetLifecycle(context.Background(), k, from, channels.Transition{To: st, By: Bob, At: T0})
		require.NoError(t, err)
		from = st
	}
}

// testListWaiting pins the anchor branch of [channels.Store.ListWaiting]:
// exact anchors, listed types and states, open and live channels only, in
// key order.
func testListWaiting(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	moved := key("W-1", "room")
	contained(t, s, moved, dueAt(time.Time{}))
	unmoved := key("W-2", "room")
	mustAppend(t, s, unmoved, text("never moved"))
	mustAppend(t, s, key("W-3", "topic-x"), text("type not listed"))
	contained(t, s, key("W-4", "topic"), dueAt(time.Time{})) // topic lists only the unmoved state
	shut := key("W-5", "room")
	contained(t, s, shut, dueAt(time.Time{}))
	setState(t, s, shut, channels.StateClosed)
	archived := key("W-6", "room")
	contained(t, s, archived, dueAt(time.Time{}))
	setState(t, s, archived, channels.StateClosed, channels.StateArchived)
	contained(t, s, key("W-7", "room"), dueAt(time.Time{}))
	tombstone(t, s, "W-7", "", T0)
	contained(t, s, faced("W-8", "draft", "room"), dueAt(time.Time{})) // asked as the family only
	onFace := faced("W-9", "draft", "room")
	contained(t, s, onFace, dueAt(time.Time{}))
	contained(t, s, faced("W-9", "published", "room"), dueAt(time.Time{})) // another face

	anchors := make([]channels.Anchor, 0, 10)
	anchors = append(anchors, channels.Anchor{ID: "W-9", Face: "draft"}, channels.Anchor{ID: "W-11"})
	for i := 1; i <= 8; i++ {
		anchors = append(anchors, channels.Anchor{ID: fmt.Sprintf("W-%d", i)})
	}
	got := listWaitingKeys(t, s, channels.WaitingQuery{States: waitStates(), Anchors: anchors})
	require.Equal(t, []channels.Key{moved, unmoved, onFace}, got)

	full, err := s.ListWaiting(context.Background(), channels.WaitingQuery{
		States: waitStates(), Anchors: []channels.Anchor{{ID: "W-1"}},
	})
	require.NoError(t, err)
	require.Len(t, full, 1)
	require.Equal(t, "contained", full[0].Session.State)
	require.Equal(t, Alice, full[0].CreatedBy)
}

// testListWaitingCreator pins the creator branch: created_by_user is
// written by CreateMany and the first append, kept by a rekey and an
// untombstone, and a channel both branches match is listed once.
func testListWaitingCreator(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	s := h.Store
	create := func(id, typ string, by channels.Author) channels.Key {
		t.Helper()
		c, err := s.CreateMany(ctx, channels.Anchor{ID: id}, channels.NewChannel{Type: typ, By: by, At: T0})
		require.NoError(t, err)
		return c.Key
	}
	mine := create("C-1", "topic", Alice)
	create("C-2", "topic", Bob)
	create("C-3", "thread", Alice) // thread is not a creator type
	implicit := key("C-4", "room")
	mustAppend(t, s, implicit, text("first post creates it"))
	old := create("R-OLD", "topic", Alice)
	require.NoError(t, s.Rekey(ctx, "R-OLD", "R-NEW", T0))
	renamed := old
	renamed.ID = "R-NEW"
	gone := create("T-1", "topic", Alice)

	q := channels.WaitingQuery{
		States: waitStates(), CreatorTypes: []string{"room", "topic"}, CreatedBy: Alice.User,
		Anchors: []channels.Anchor{{ID: "C-1"}},
	}
	require.Equal(t, []channels.Key{mine, implicit, renamed, gone}, listWaitingKeys(t, s, q))

	batch := tombstone(t, s, "T-1", "", T0)
	require.Equal(t, []channels.Key{mine, implicit, renamed}, listWaitingKeys(t, s, q))
	require.NoError(t, s.Untombstone(ctx, batch, T0.Add(time.Minute), nil))
	require.Equal(t, []channels.Key{mine, implicit, renamed, gone}, listWaitingKeys(t, s, q))

	q.Anchors = nil
	q.CreatedBy = Bob.User
	require.Len(t, listWaitingKeys(t, s, q), 1)
	q.CreatedBy = "nobody"
	require.Empty(t, listWaitingKeys(t, s, q))
}

// testListWaitingChunks pins a long anchor list, which the SQL backends
// split into statements, and the row ceiling on the merged result.
func testListWaitingChunks(t *testing.T, h Harness) {
	t.Helper()
	s := h.Store
	anchors := make([]channels.Anchor, 1200)
	for i := range anchors {
		anchors[i] = channels.Anchor{ID: fmt.Sprintf("L-%04d", i)}
	}
	want := make([]channels.Key, 0, 4)
	for _, i := range []int{3, 610, 1150} {
		k := key(anchors[i].ID, "room")
		mustAppend(t, s, k, text("waits"))
		want = append(want, k)
	}
	byBob, err := s.CreateMany(context.Background(), channels.Anchor{ID: "L-9999"},
		channels.NewChannel{Type: "topic", By: Bob, At: T0})
	require.NoError(t, err)
	want = append(want, byBob.Key)
	q := channels.WaitingQuery{
		States: waitStates(), Anchors: anchors, CreatorTypes: []string{"topic"}, CreatedBy: Bob.User,
	}
	require.Equal(t, want, listWaitingKeys(t, s, q))

	q.MaxRows = len(want)
	require.Equal(t, want, listWaitingKeys(t, s, q), "the ceiling itself is allowed")
	q.MaxRows = len(want) - 1
	_, err = s.ListWaiting(context.Background(), q)
	require.ErrorIs(t, err, channels.ErrTooManyRows)
	// A channel both branches match counts once toward the ceiling.
	both := q
	both.Anchors = append(slices.Clone(q.Anchors), channels.Anchor{ID: "L-9999"})
	both.MaxRows = len(want)
	require.Equal(t, want, listWaitingKeys(t, s, both))
}

// testListWaitingNames pins [channels.WaitingQuery.Names] on both
// branches: an anchor holds one unnamed and two named topics, and the
// filter decides which are listed and which count toward the ceiling
// (TKT-21G6JS).
func testListWaitingNames(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	s := h.Store
	unnamed := key("N-1", "topic")
	mustAppend(t, s, unnamed, text("unnamed"))
	named := make([]channels.Key, 0, 2)
	for range 2 {
		c, err := s.CreateMany(ctx, channels.Anchor{ID: "N-1"}, channels.NewChannel{Type: "topic", By: Alice, At: T0})
		require.NoError(t, err)
		named = append(named, c.Key)
	}
	slices.SortFunc(named, func(a, b channels.Key) int { return strings.Compare(a.Name, b.Name) })
	for _, branch := range []channels.WaitingQuery{
		{States: waitStates(), Anchors: []channels.Anchor{{ID: "N-1"}}},
		{States: waitStates(), CreatorTypes: []string{"topic"}, CreatedBy: Alice.User},
	} {
		q := branch
		require.Len(t, listWaitingKeys(t, s, q), 3)
		q.Names = channels.UnnamedOnly
		q.MaxRows = 1
		require.Equal(t, []channels.Key{unnamed}, listWaitingKeys(t, s, q), "the named topics do not count")
		q.Names = channels.NamedOnly
		q.MaxRows = 2
		require.Equal(t, named, listWaitingKeys(t, s, q), "the unnamed topic does not count")
	}
}

// testListWaitingInvalid pins the argument checks.
func testListWaitingInvalid(t *testing.T, h Harness) {
	t.Helper()
	ok := []channels.Anchor{{ID: "X-1"}}
	tooMany := make([]channels.Anchor, channels.MaxWaitingAnchors+1)
	for i := range tooMany {
		tooMany[i] = channels.Anchor{ID: fmt.Sprintf("X-%d", i)}
	}
	cases := map[string]channels.WaitingQuery{
		"no states":          {Anchors: ok},
		"invalid type":       {States: map[string][]string{"Bad Type": {""}}, Anchors: ok},
		"type without state": {States: map[string][]string{"room": {}}, Anchors: ok},
		"invalid state":      {States: map[string][]string{"room": {"Bad State"}}, Anchors: ok},
		"nothing to match":   {States: waitStates()},
		"creator without types": {
			States: waitStates(), CreatedBy: Alice.User,
		},
		"too many anchors": {States: waitStates(), Anchors: tooMany},
		"invalid anchor":   {States: waitStates(), Anchors: []channels.Anchor{{ID: "not an id"}}},
		"invalid creator type": {
			States: waitStates(), CreatorTypes: []string{"Bad Type"}, CreatedBy: Alice.User,
		},
		"negative row ceiling": {States: waitStates(), Anchors: ok, MaxRows: -1},
		"row ceiling too high": {States: waitStates(), Anchors: ok, MaxRows: channels.MaxWaitingRows + 1},
		"unknown name filter":  {States: waitStates(), Anchors: ok, Names: channels.NamedOnly + 1},
	}
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h.Store.ListWaiting(context.Background(), q)
			require.ErrorIs(t, err, channels.ErrInvalidKey)
		})
	}
}
