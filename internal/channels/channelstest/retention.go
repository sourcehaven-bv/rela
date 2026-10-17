package channelstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The conformance tests of the retention methods: Tombstones,
// ConfirmTombstone, Purge and DropTypeRecord.

// fileBatch tombstones id@face with filing f and returns the handle.
func fileBatch(
	t *testing.T, s channels.Store, id string, face entity.Face, at time.Time, f channels.Filing,
) channels.TombstoneHandle {
	t.Helper()
	h, err := s.Tombstone(context.Background(), id, face, at, f)
	require.NoError(t, err)
	require.False(t, h.IsZero())
	return h
}

// batchKeys returns the keys of a listed batch's channels.
func batchKeys(b channels.TombstoneBatch) []channels.Key {
	out := make([]channels.Key, len(b.Channels))
	for i, c := range b.Channels {
		out[i] = c.Key
	}
	return out
}

func testTombstonesList(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	room := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-2", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, room, text("one"))
	mustAppend(t, h.Store, draft, text("two"))
	_, err := h.Store.SetLifecycle(ctx, room, channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: Bob, At: T0.Add(time.Second)})
	require.NoError(t, err)

	purgeAfter := T0.Add(30 * 24 * time.Hour)
	whole := fileBatch(t, h.Store, "TKT-1", "", T0.Add(time.Minute), channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: purgeAfter, Pending: true,
	})
	face := fileBatch(t, h.Store, "TKT-2", "draft", T0.Add(2*time.Minute), channels.Filing{})
	mustAppend(t, h.Store, key("TKT-3", "room"), text("three"))
	later := fileBatch(t, h.Store, "TKT-3", "", T0.Add(time.Hour), channels.Filing{Policy: channels.PolicyOrphan})

	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 2, "a batch filed at the bound is left out")
	require.Equal(t, whole, got[0].Handle)
	require.Equal(t, channels.NormTime(T0.Add(time.Minute)), got[0].At)
	require.Equal(t, channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: channels.NormTime(purgeAfter), Pending: true,
	}, got[0].Filing)
	require.Equal(t, []channels.Key{room}, batchKeys(got[0]))
	require.Equal(t, channels.StateClosed, got[0].Channels[0].State)
	require.Equal(t, channels.NormTime(T0.Add(time.Second)), got[0].Channels[0].ClosedAt)
	require.Equal(t, face, got[1].Handle)
	require.Equal(t, channels.Filing{}, got[1].Filing)
	require.Equal(t, []channels.Key{draft}, batchKeys(got[1]))

	got, err = h.Store.Tombstones(ctx, T0.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, later, got[2].Handle)
	require.Equal(t, channels.PolicyOrphan, got[2].Filing.Policy)

	require.NoError(t, h.Store.Untombstone(ctx, face, T0.Add(3*time.Hour)))
	got, err = h.Store.Tombstones(ctx, T0.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 2, "a restored batch is not listed")

	_, err = h.Store.Tombstone(ctx, "TKT-1", "", T0, channels.Filing{Policy: "bogus"})
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
}

func testConfirmTombstone(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	mustAppend(t, h.Store, key("TKT-1", "room"), text("x"))
	b := fileBatch(t, h.Store, "TKT-1", "", T0, channels.Filing{Policy: channels.PolicyDelete, Pending: true})

	require.NoError(t, h.Store.ConfirmTombstone(ctx, b))
	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.False(t, got[0].Filing.Pending)
	require.Equal(t, channels.PolicyDelete, got[0].Filing.Policy, "confirming keeps the rest of the filing")
	require.NoError(t, h.Store.ConfirmTombstone(ctx, b), "a confirmed batch stays confirmed")

	require.NoError(t, h.Store.Untombstone(ctx, b, T0.Add(time.Minute)))
	require.ErrorIs(t, h.Store.ConfirmTombstone(ctx, b), channels.ErrNotFound)
	require.ErrorIs(t, h.Store.ConfirmTombstone(ctx, channels.TombstoneHandle{}), channels.ErrInvalidKey)
}

func testPurge(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	room := key("TKT-1", "room")
	mustAppend(t, h.Store, room, text("secret of the first life"))
	b := fileBatch(t, h.Store, "TKT-1", "", T0.Add(time.Minute), channels.Filing{
		Policy: channels.PolicyDelete, Pending: true,
	})
	require.ErrorIs(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)), channels.ErrConflict,
		"a batch whose delete has not committed is not purged")
	require.NoError(t, h.Store.ConfirmTombstone(ctx, b))

	// A new entity with the same id has its own channels and trail.
	mustAppend(t, h.Store, room, text("second life"))
	liveBefore, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)

	require.ErrorIs(t, h.Store.Purge(ctx, b, channels.Author{}, T0.Add(time.Hour)), channels.ErrInvalidMessage)
	require.NoError(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)))

	raw := string(h.Raw(t))
	require.NotContains(t, raw, "secret of the first life", "a purge erases the messages")
	require.Contains(t, raw, `"op":"purge"`, "the filed trail keeps a purge record")
	liveAfter, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Equal(t, liveBefore, liveAfter, "the live trail of the id is not touched")
	msgs, err := h.Store.Messages(ctx, room, channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "second life", msgs[0].Props["body"])

	got, err := h.Store.Tombstones(ctx, T0.Add(24*time.Hour))
	require.NoError(t, err)
	require.Empty(t, got)
	require.ErrorIs(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)), channels.ErrNotFound, "a batch is purged once")
	require.ErrorIs(t, h.Store.Untombstone(ctx, b, T0.Add(time.Hour)), channels.ErrNotFound,
		"a purged batch cannot come back")
	require.NoError(t, h.Store.Purge(ctx, channels.TombstoneHandle{}, Bob, T0), "the zero handle is a no-op")
}

func testPurgeFace(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("family"))
	mustAppend(t, h.Store, draft, text("draft only"))
	b := fileBatch(t, h.Store, "TKT-1", "draft", T0, channels.Filing{Policy: channels.PolicyDetach})

	require.NoError(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)))
	require.NotContains(t, string(h.Raw(t)), "draft only")
	_, err := h.Store.Head(ctx, fam)
	require.NoError(t, err, "the family channel never moved")
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	for _, r := range trail {
		require.NotEqual(t, channels.AuditPurge, r.Op, "the purge record is not in the live trail")
	}
	require.ErrorIs(t, h.Store.Untombstone(ctx, b, T0), channels.ErrNotFound)
}

func testPurgeAfterUntombstone(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	room := key("TKT-1", "room")
	mustAppend(t, h.Store, room, text("kept"))
	b := fileBatch(t, h.Store, "TKT-1", "", T0, channels.Filing{})
	require.NoError(t, h.Store.Untombstone(ctx, b, T0.Add(time.Minute)))
	require.ErrorIs(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)), channels.ErrNotFound)
	msgs, err := h.Store.Messages(ctx, room, channels.Page{})
	require.NoError(t, err)
	require.Len(t, msgs, 1, "a refused purge erases nothing")
}

func testDropTypeRecord(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	family := channels.TypeRecord{Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne}
	require.NoError(t, h.Store.RecordTypes(ctx, map[string]channels.TypeRecord{
		"room": family, "gone": family, "detached": family,
	}))
	mustAppend(t, h.Store, key("TKT-1", "room"), text("x"))
	mustAppend(t, h.Store, key("TKT-2", "detached"), text("y"))
	b := fileBatch(t, h.Store, "TKT-2", "", T0, channels.Filing{})

	require.ErrorIs(t, h.Store.DropTypeRecord(ctx, "room"), channels.ErrTypeInUse, "a live channel keeps it")
	require.ErrorIs(t, h.Store.DropTypeRecord(ctx, "detached"), channels.ErrTypeInUse,
		"a tombstoned channel keeps it")
	require.NoError(t, h.Store.DropTypeRecord(ctx, "gone"))
	require.NoError(t, h.Store.DropTypeRecord(ctx, "never-recorded"))
	require.ErrorIs(t, h.Store.DropTypeRecord(ctx, "Bad Type"), channels.ErrInvalidKey)

	got, err := h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Contains(t, got, "room")
	require.Contains(t, got, "detached")
	require.NotContains(t, got, "gone")

	require.NoError(t, h.Store.Purge(ctx, b, Bob, T0.Add(time.Hour)))
	require.NoError(t, h.Store.DropTypeRecord(ctx, "detached"), "a purged batch no longer holds the type")
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]channels.TypeRecord{"room": family}, got)
}
