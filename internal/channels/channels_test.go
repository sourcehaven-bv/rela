package channels

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

var (
	alice = Author{User: "alice"}
	t0    = time.Date(2026, 10, 12, 9, 0, 0, 999, time.FixedZone("X", 3600))
	// longFace is a valid face one byte over the cap.
	longFace = entity.Face("f" + strings.Repeat("a", MaxFaceBytes))
)

func TestKeyValidateAndString(t *testing.T) {
	cases := []struct {
		name string
		key  Key
		ok   bool
		str  string
	}{
		{"family", Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}, true, "TKT-1/room"},
		{"face", Key{Anchor: Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}, true, "TKT-1@draft/room"},
		{"named", Key{Anchor: Anchor{ID: "TKT-1"}, Type: "topic", Name: "abc123"}, true, "TKT-1/topic/abc123"},
		{"bad id", Key{Anchor: Anchor{ID: "a/b"}, Type: "room"}, false, ""},
		{"bad face", Key{Anchor: Anchor{ID: "TKT-1", Face: "Bad Face"}, Type: "room"}, false, ""},
		{"reserved type", Key{Anchor: Anchor{ID: "TKT-1"}, Type: "lpt1"}, false, ""},
		{"long type", Key{Anchor: Anchor{ID: "TKT-1"}, Type: "a" + strings.Repeat("b", 63)}, false, ""},
		{"bad name", Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room", Name: "a-b"}, false, ""},
		{"longest id", Key{Anchor: Anchor{ID: strings.Repeat("A", MaxAnchorIDBytes)}, Type: "room"}, true,
			strings.Repeat("A", MaxAnchorIDBytes) + "/room"},
		{"long id", Key{Anchor: Anchor{ID: strings.Repeat("A", MaxAnchorIDBytes+1)}, Type: "room"}, false, ""},
		{"long face", Key{Anchor: Anchor{ID: "TKT-1", Face: longFace}, Type: "room"}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.key.Validate()
			if !tc.ok {
				require.ErrorIs(t, err, ErrInvalidKey)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.str, tc.key.String())
		})
	}
}

func TestPageNormalize(t *testing.T) {
	p, err := Page{}.Normalize()
	require.NoError(t, err)
	require.Equal(t, DefaultPageLimit, p.Limit)
	p, err = Page{Limit: 5000}.Normalize()
	require.NoError(t, err)
	require.Equal(t, MaxPageLimit, p.Limit)
	p, err = Page{AfterRev: 3, Limit: 7}.Normalize()
	require.NoError(t, err)
	require.Equal(t, 7, p.Limit)
	_, err = Page{AfterRev: 1, BeforeSeq: 1}.Normalize()
	require.ErrorIs(t, err, ErrInvalidPage)
	_, err = Page{BeforeSeq: -1}.Normalize()
	require.ErrorIs(t, err, ErrInvalidPage)
}

func TestPrepareMessage(t *testing.T) {
	m, raw, err := PrepareMessage(NewMessage{Type: "text", Author: alice, At: t0, Props: map[string]any{"n": 1}})
	require.NoError(t, err)
	require.JSONEq(t, `{"n":1}`, string(raw))
	require.Equal(t, map[string]any{"n": float64(1)}, m.Props)
	require.Equal(t, time.UTC, m.At.Location())
	require.Zero(t, m.At.Nanosecond()%1000)

	bad := []struct {
		name string
		msg  NewMessage
		want error
	}{
		{"type", NewMessage{Type: "X", Author: alice, At: t0}, ErrInvalidMessage},
		{"author", NewMessage{Type: "text", Author: Author{User: " "}, At: t0}, ErrInvalidMessage},
		{"long author", NewMessage{Type: "text", Author: Author{User: strings.Repeat("a", 201)}, At: t0}, ErrTooLarge},
		{"time", NewMessage{Type: "text", Author: alice}, ErrInvalidMessage},
		{"reply", NewMessage{Type: "text", Author: alice, At: t0, ReplyTo: -1}, ErrInvalidReply},
		{"hash", NewMessage{Type: "text", Author: alice, At: t0, SchemaHash: strings.Repeat("h", 201)}, ErrTooLarge},
		{"incarnation", NewMessage{Type: "text", Author: alice, At: t0, Incarnation: strings.Repeat("i", 201)}, ErrTooLarge},
		{"encode", NewMessage{Type: "text", Author: alice, At: t0, Props: map[string]any{"f": func() {}}}, ErrInvalidMessage},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, _, prepErr := PrepareMessage(tc.msg)
			require.ErrorIs(t, prepErr, tc.want)
		})
	}
	props, raw, err := PrepareProps(nil)
	require.NoError(t, err)
	require.Nil(t, props)
	require.Nil(t, raw)
}

func TestPrepareRemovalAndTransition(t *testing.T) {
	r, err := PrepareRemoval(Removal{By: alice, At: t0, Reason: "spam"})
	require.NoError(t, err)
	require.Equal(t, NormTime(t0), r.At)
	_, err = PrepareRemoval(Removal{By: alice})
	require.ErrorIs(t, err, ErrInvalidMessage)

	tr, err := PrepareTransition(StateOpen, Transition{To: StateClosed, By: alice, At: t0})
	require.NoError(t, err)
	require.Equal(t, NormTime(t0), tr.At)
	_, err = PrepareTransition("bogus", Transition{To: StateClosed, By: alice, At: t0})
	require.ErrorIs(t, err, ErrInvalidMessage)
	_, err = PrepareTransition(StateOpen, Transition{To: StateClosed, At: t0})
	require.ErrorIs(t, err, ErrInvalidMessage)
	_, err = PrepareTransition(StateOpen, Transition{To: StateClosed, By: alice})
	require.ErrorIs(t, err, ErrInvalidMessage)
}

func TestPrepareAction(t *testing.T) {
	a, err := PrepareAction("accept", Action{Status: ActionPending, By: alice, At: t0})
	require.NoError(t, err)
	require.Equal(t, "accept", a.Name)
	require.Equal(t, a.At, a.PendingAt, "a pending action without a pending time is pending since now")

	a, err = PrepareAction("accept", Action{By: alice, At: t0})
	require.NoError(t, err)
	require.Equal(t, Action{}, a)

	for _, next := range []Action{
		{Status: "weird", By: alice, At: t0},
		{Status: ActionDone, At: t0},
		{Status: ActionDone, By: alice},
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "a b", Type: "task"}},
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "T-1"}},
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "T-1", Type: strings.Repeat("t", 65)}},
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "T-1", Type: "ta\x00sk"}},
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "T-1", Type: "\xff"}},
	} {
		_, err = PrepareAction("accept", next)
		require.ErrorIs(t, err, ErrInvalidMessage)
	}
	_, err = PrepareAction("accept", Action{
		Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "T-1", Type: "mt_overleg"},
	})
	require.NoError(t, err, "an entity type may use characters a channel name may not")
	_, err = PrepareAction("Accept", Action{})
	require.ErrorIs(t, err, ErrInvalidMessage)

	require.NoError(t, CheckActionCAS(nil, "accept", ActionNone))
	require.ErrorIs(t, CheckActionCAS(nil, "accept", ActionPending), ErrConflict)
	require.ErrorIs(t, CheckActionCAS(&Action{Name: "reject", Status: ActionPending}, "accept", ActionPending), ErrConflict)
	require.Equal(t, ActionNone, Message{}.ActionStatus())
	require.Equal(t, ActionDone, Message{Action: &Action{Status: ActionDone}}.ActionStatus())
}

func TestPrepareChannelAndRekey(t *testing.T) {
	nc, err := PrepareChannel(Anchor{ID: "TKT-1"}, NewChannel{Type: "topic", Title: "t", By: alice, At: t0})
	require.NoError(t, err)
	require.Equal(t, NormTime(t0), nc.At)
	_, err = PrepareChannel(Anchor{ID: "TKT-1"}, NewChannel{Type: "topic", Title: "t", By: alice})
	require.ErrorIs(t, err, ErrInvalidMessage)
	_, err = PrepareChannel(Anchor{ID: "TKT-1"}, NewChannel{
		Type: "topic", Incarnation: strings.Repeat("i", MaxTextBytes+1), By: alice, At: t0,
	})
	require.ErrorIs(t, err, ErrTooLarge)

	require.NoError(t, CheckRekey("A-1", "B-1"))
	require.ErrorIs(t, CheckRekey("A-1", "a b"), ErrInvalidKey)
	require.ErrorIs(t, CheckRekey("a b", "A-1"), ErrInvalidKey)
}

func TestIncarnation(t *testing.T) {
	c := Channel{Key: Key{Anchor: Anchor{ID: "T-1"}, Type: "room"}, Incarnation: "life-1"}
	require.NoError(t, CheckIncarnation(c, "life-1"))
	require.ErrorIs(t, CheckIncarnation(c, "life-2"), ErrNotFound)
	require.ErrorIs(t, CheckIncarnation(c, ""), ErrNotFound)
	require.NoError(t, CheckAppendIncarnation(c.Key, "life-1", "life-1"))
	require.ErrorIs(t, CheckAppendIncarnation(c.Key, "life-1", "life-2"), ErrStaleAnchor)
}

func TestLifecycleTimes(t *testing.T) {
	c := Channel{State: StateOpen, CreatedAt: t0}
	require.Equal(t, t0, c.StateSince())
	ApplyTransition(&c, Transition{To: StateClosed, At: t0.Add(time.Hour)})
	require.Equal(t, t0.Add(time.Hour), c.StateSince())
	ApplyTransition(&c, Transition{To: StateArchived, At: t0.Add(2 * time.Hour)})
	require.Equal(t, t0.Add(2*time.Hour), c.StateSince())
	require.Equal(t, t0.Add(time.Hour), c.ClosedAt)
	ApplyTransition(&c, Transition{To: StateClosed, At: t0.Add(3 * time.Hour)})
	require.True(t, c.ArchivedAt.IsZero())
	ApplyTransition(&c, Transition{To: StateOpen, At: t0.Add(4 * time.Hour)})
	require.True(t, c.ClosedAt.IsZero())
	ApplyTransition(&c, Transition{To: StateArchived, At: t0.Add(5 * time.Hour)})
	require.Equal(t, t0.Add(5*time.Hour), c.ClosedAt, "archiving an open channel closes it too")
	require.Equal(t, t0.Add(5*time.Hour), c.ArchivedAt)
	require.True(t, NormTime(time.Time{}).IsZero())
}

func TestNamesAndOrder(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		n := NewName()
		require.Len(t, n, 13)
		require.NoError(t, Key{Anchor: Anchor{ID: "A"}, Type: "t", Name: n}.Validate())
		require.False(t, seen[n])
		seen[n] = true
	}
	first, second := TombstoneStamp(t0), TombstoneStamp(t0)
	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(TombstoneStamp(t0), "20261012T080000.000000Z-"))

	ks := []Key{
		{Anchor: Anchor{ID: "B"}, Type: "a"},
		{Anchor: Anchor{ID: "A", Face: "x"}, Type: "a"},
		{Anchor: Anchor{ID: "A"}, Type: "b"},
		{Anchor: Anchor{ID: "A"}, Type: "a", Name: "z"},
		{Anchor: Anchor{ID: "A"}, Type: "a"},
	}
	SortKeys(ks)
	var got []string
	for _, k := range ks {
		got = append(got, k.String())
	}
	require.Equal(t, []string{"A/a", "A/a/z", "A/b", "A@x/a", "B/a"}, got)
	cs := []Channel{{Key: ks[4]}, {Key: ks[0]}}
	SortChannels(cs)
	require.Equal(t, ks[0], cs[0].Key)
	require.Equal(t, AuditRecord{At: t0, Op: AuditCreate, Face: "x", Type: "a"}, KeyAudit(ks[3], AuditCreate, t0))
	require.True(t, Author{}.IsZero())
}

func TestTypeRecords(t *testing.T) {
	require.True(t, AnchorFamily.Valid())
	require.True(t, AnchorFace.Valid())
	require.False(t, AnchorMode("").Valid())
	require.True(t, CardinalityOne.Valid())
	require.True(t, CardinalityMany.Valid())
	require.False(t, Cardinality("").Valid())

	famOne := TypeRecord{Anchor: AnchorFamily, Cardinality: CardinalityOne}
	faceOne := TypeRecord{Anchor: AnchorFace, Cardinality: CardinalityOne}
	famMany := TypeRecord{Anchor: AnchorFamily, Cardinality: CardinalityMany}
	require.NoError(t, PrepareTypeRecords(nil))
	require.ErrorIs(t, PrepareTypeRecords(map[string]TypeRecord{"CON": famOne}), ErrInvalidKey)
	require.ErrorIs(t, PrepareTypeRecords(map[string]TypeRecord{"room": {Anchor: "x", Cardinality: CardinalityOne}}),
		ErrInvalidMessage)
	require.ErrorIs(t, PrepareTypeRecords(map[string]TypeRecord{"room": {Anchor: AnchorFace}}), ErrInvalidMessage)

	recorded := map[string]TypeRecord{"room": famOne, "review": faceOne, "old": faceOne, "a": famOne, "b": famOne}
	configured := map[string]TypeRecord{"room": faceOne, "review": faceOne, "new": famOne, "a": famMany, "b": famOne}
	require.Equal(t, []TypeRecordChange{
		{Type: "a", Recorded: famOne, Configured: famMany},
		{Type: "room", Recorded: famOne, Configured: faceOne},
	}, TypeRecordChanges(recorded, configured))
	require.Empty(t, TypeRecordChanges(nil, configured), "a store with no record has no changes")

	require.Equal(t, map[string]TypeRecord{"room": faceOne, "new": famOne, "a": famMany},
		ChangedTypeRecords(recorded, configured))
	require.Empty(t, ChangedTypeRecords(configured, configured))
}

func TestTombstoneHandle(t *testing.T) {
	stamp := TombstoneStamp(time.Date(2026, 1, 2, 3, 4, 5, 6000, time.UTC))
	h := NewTombstoneHandle(Anchor{ID: "TKT-1", Face: "draft"}, stamp)
	require.False(t, h.IsZero())
	require.NoError(t, h.Validate())
	back, err := ParseTombstoneHandle(h.String())
	require.NoError(t, err)
	require.Equal(t, h, back)

	fam := NewTombstoneHandle(Anchor{ID: "TKT_2-x"}, stamp)
	back, err = ParseTombstoneHandle(fam.String())
	require.NoError(t, err)
	require.Equal(t, fam, back, "a whole-entity handle round-trips")

	require.Empty(t, TombstoneHandle{}.String())
	require.ErrorIs(t, TombstoneHandle{}.Validate(), ErrInvalidKey)
	require.ErrorIs(t, NewTombstoneHandle(Anchor{ID: "TKT-1"}, "../x").Validate(), ErrInvalidKey)
	require.ErrorIs(t, NewTombstoneHandle(Anchor{ID: "../x"}, stamp).Validate(), ErrInvalidKey)
	for _, bad := range []string{"t1:!:x:" + stamp, "t1:QQ:!:" + stamp} {
		_, err = ParseTombstoneHandle(bad)
		require.ErrorIs(t, err, ErrInvalidKey, bad)
	}
	err = &OccupiedError{Anchor: Anchor{ID: "TKT-1", Face: "draft"}}
	require.Contains(t, err.Error(), "TKT-1@draft")
}
