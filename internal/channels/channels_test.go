package channels

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	alice = Author{User: "alice"}
	t0    = time.Date(2026, 10, 12, 9, 0, 0, 999, time.FixedZone("X", 3600))
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
		{Status: ActionDone, By: alice, At: t0, Promoted: &Promoted{ID: "a b"}},
	} {
		_, err = PrepareAction("accept", next)
		require.ErrorIs(t, err, ErrInvalidMessage)
	}
	_, err = PrepareAction("Accept", Action{})
	require.ErrorIs(t, err, ErrInvalidMessage)

	require.NoError(t, CheckActionCAS(nil, "accept", ActionNone))
	require.ErrorIs(t, CheckActionCAS(nil, "accept", ActionPending), ErrConflict)
	require.ErrorIs(t, CheckActionCAS(&Action{Name: "reject", Status: ActionPending}, "accept", ActionPending), ErrConflict)
	require.Equal(t, ActionNone, Message{}.ActionStatus())
	require.Equal(t, ActionDone, Message{Action: &Action{Status: ActionDone}}.ActionStatus())
}

func TestPrepareChannelAndRekey(t *testing.T) {
	at, err := PrepareChannel(Anchor{ID: "TKT-1"}, "topic", "t", alice, t0)
	require.NoError(t, err)
	require.Equal(t, NormTime(t0), at)
	_, err = PrepareChannel(Anchor{ID: "TKT-1"}, "topic", "t", alice, time.Time{})
	require.ErrorIs(t, err, ErrInvalidMessage)

	require.NoError(t, CheckRekey("A-1", "B-1"))
	require.ErrorIs(t, CheckRekey("A-1", "a b"), ErrInvalidKey)
	require.ErrorIs(t, CheckRekey("a b", "A-1"), ErrInvalidKey)
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

func TestAnchorModes(t *testing.T) {
	require.True(t, AnchorFamily.Valid())
	require.True(t, AnchorFace.Valid())
	require.False(t, AnchorMode("").Valid())

	require.NoError(t, PrepareAnchorModes(nil))
	require.ErrorIs(t, PrepareAnchorModes(map[string]AnchorMode{"CON": AnchorFace}), ErrInvalidKey)
	require.ErrorIs(t, PrepareAnchorModes(map[string]AnchorMode{"room": "x"}), ErrInvalidMessage)

	recorded := map[string]AnchorMode{"room": AnchorFamily, "review": AnchorFace, "old": AnchorFace}
	configured := map[string]AnchorMode{"room": AnchorFace, "review": AnchorFace, "new": AnchorFamily, "a": AnchorFace}
	recorded["a"] = AnchorFamily
	require.Equal(t, []AnchorModeChange{
		{Type: "a", Recorded: AnchorFamily, Configured: AnchorFace},
		{Type: "room", Recorded: AnchorFamily, Configured: AnchorFace},
	}, AnchorModeChanges(recorded, configured))
	require.Empty(t, AnchorModeChanges(nil, configured), "a store with no record has no changes")
}
