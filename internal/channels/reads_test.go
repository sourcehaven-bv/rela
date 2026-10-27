package channels

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestChannelReadsMigrations_SessionMarker pins that the 'session:'
// literal in the sqlite v28 rung and in pg migration 0035 is
// [SessionMarker], and that the sqlite substr length matches it
// (TKT-2B2AIA AC 6). The migrations are SQL text, so they cannot use the
// constant.
func TestChannelReadsMigrations_SessionMarker(t *testing.T) {
	t.Parallel()
	sqlite, err := os.ReadFile("../sqlitedb/migrate.go")
	require.NoError(t, err)
	require.Contains(t, string(sqlite),
		"substr(json_extract(doc, '$.triggered_by'), 1, "+strconv.Itoa(len(SessionMarker))+") = '"+SessionMarker+"'")
	pg, err := os.ReadFile("../store/pgstore/migrations/0035_channel_reads.sql")
	require.NoError(t, err)
	require.Contains(t, string(pg), `strpos(doc, '"triggered_by":"`+SessionMarker+`') > 0`)
	require.Contains(t, string(pg), "starts_with(COALESCE(d ->> 'triggered_by', ''), '"+SessionMarker+"')")
}

func TestReader_Validate(t *testing.T) {
	t.Parallel()
	require.NoError(t, Reader{User: "alice", OrgID: "org"}.Validate())
	for _, r := range []Reader{
		{}, {User: "  "}, {User: "a\x00b"}, {User: "a", OrgID: "o\x00"}, {User: "\xff"},
		{User: strings.Repeat("a", MaxTextBytes+1)},
	} {
		require.ErrorIs(t, r.Validate(), ErrInvalidMessage, "%q", r.User)
	}
}

func TestReadChecks(t *testing.T) {
	t.Parallel()
	r := Reader{User: "alice"}
	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	require.NoError(t, CheckSetRead(k, r, 0))
	require.ErrorIs(t, CheckSetRead(k, r, -1), ErrInvalidMessage)
	require.ErrorIs(t, CheckSetRead(Key{}, r, 0), ErrInvalidKey)
	require.ErrorIs(t, CheckSetRead(k, Reader{}, 0), ErrInvalidMessage)
	require.NoError(t, CheckReads(r, nil))
	require.ErrorIs(t, CheckReads(r, make([]Key, MaxReadKeys+1)), ErrInvalidKey)
	require.ErrorIs(t, CheckReads(r, []Key{{}}), ErrInvalidKey)
	require.ErrorIs(t, CheckReads(Reader{}, nil), ErrInvalidMessage)
	require.NoError(t, CheckListReads(r, MaxReadWindow))
	require.ErrorIs(t, CheckListReads(r, 0), ErrInvalidMessage)
	require.ErrorIs(t, CheckListReads(r, MaxReadWindow+1), ErrInvalidMessage)
	require.ErrorIs(t, CheckListReads(Reader{}, 1), ErrInvalidMessage)
}

func TestClampRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ old, asked, last, want int64 }{
		{0, 0, 0, 0}, {1, 0, 5, 1}, {1, 3, 5, 3}, {1, 9, 5, 5}, {5, 9, 3, 5},
	} {
		require.Equal(t, c.want, ClampRead(c.old, c.asked, c.last), "%+v", c)
	}
}

func TestCounts(t *testing.T) {
	t.Parallel()
	require.True(t, Counts(Message{}))
	require.True(t, Counts(Message{TriggeredBy: "reminder:room/x"}))
	require.False(t, Counts(Message{Removed: &Removal{}}))
	require.False(t, Counts(Message{TriggeredBy: SessionMarker + "room"}))
}

func TestLifecycleAt(t *testing.T) {
	t.Parallel()
	c := Channel{CreatedAt: t0}
	require.Equal(t, t0, LifecycleAt(c))
	c.ClosedAt, c.ArchivedAt, c.ReopenedAt = t0.Add(3*time.Hour), t0.Add(time.Hour), t0.Add(2*time.Hour)
	require.Equal(t, t0.Add(3*time.Hour), LifecycleAt(c))
}

func TestSortReadWindow(t *testing.T) {
	t.Parallel()
	mk := func(id string, seq, last int64, at time.Time) ReadMarker {
		return ReadMarker{Channel: Channel{Key: Key{Anchor: Anchor{ID: id}, Type: "room"}, LastSeq: last}, Seq: seq,
			LastAt: at}
	}
	ms := []ReadMarker{
		mk("A-1", 1, 1, t0.Add(time.Hour)),
		mk("A-2", 1, 2, t0),
		mk("A-3", 1, 1, t0),
		mk("A-4", 1, 1, t0.Add(time.Hour)),
		mk("A-5", 1, 3, t0.Add(time.Minute)),
	}
	got := SortReadWindow(ms, 4)
	ids := make([]string, len(got))
	for i, m := range got {
		ids[i] = m.Channel.Key.ID
	}
	require.Equal(t, []string{"A-5", "A-2", "A-1", "A-4"}, ids)
}
