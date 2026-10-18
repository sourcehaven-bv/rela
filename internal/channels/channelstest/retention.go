package channelstest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The conformance tests of the retention methods: Tombstones,
// DueTombstones, ConfirmTombstone(s), Purge and DropTypeRecord.

// fileBatch tombstones id@face with filing f and returns the handle.
func fileBatch(
	t *testing.T, s channels.Store, id string, face entity.Face, at time.Time, f channels.Filing,
) channels.TombstoneHandle {
	t.Helper()
	h, err := s.Tombstone(context.Background(), id, face, at, channels.FileAs(f))
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

// listed returns the batch h from a listing, or nil.
func listed(got []channels.TombstoneBatch, h channels.TombstoneHandle) *channels.TombstoneBatch {
	for i := range got {
		if got[i].Handle == h {
			return &got[i]
		}
	}
	return nil
}

// testFilerSeesMovedChannels pins that the filer runs over the channels the
// batch moves, read inside the Tombstone call: a channel created after a
// caller listed the entity is counted, a face batch sees only its face, and
// nothing moved means the filer does not run.
func testFilerSeesMovedChannels(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	room := key("TKT-1", "room")
	mustAppend(t, h.Store, room, text("one"))
	listedNow, err := h.Store.ListEntity(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, listedNow, 1)

	// Created after the list, before the tombstone.
	late := key("TKT-1", "late")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, late, text("two"))
	mustAppend(t, h.Store, draft, text("three"))

	var faceSeen []channels.Key
	_, err = h.Store.Tombstone(ctx, "TKT-1", "draft", T0, func(moved []channels.Channel) (channels.Filing, error) {
		for _, c := range moved {
			faceSeen = append(faceSeen, c.Key)
		}
		return channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: T0.Add(time.Hour)}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []channels.Key{draft}, faceSeen)

	var seen []channels.Key
	hw, err := h.Store.Tombstone(ctx, "TKT-1", "", T0.Add(time.Second),
		func(moved []channels.Channel) (channels.Filing, error) {
			for _, c := range moved {
				seen = append(seen, c.Key)
			}
			after := T0.Add(time.Duration(len(moved)) * time.Hour)
			return channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: after}, nil
		})
	require.NoError(t, err)
	require.ElementsMatch(t, []channels.Key{room, late}, seen, "the late channel is in the batch the filer saw")

	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	whole := listed(got, hw)
	require.NotNil(t, whole)
	require.Equal(t, channels.NormTime(T0.Add(2*time.Hour)), whole.Filing.PurgeAfter)
	require.ElementsMatch(t, []channels.Key{room, late}, batchKeys(*whole))

	called := false
	empty, err := h.Store.Tombstone(ctx, "TKT-9", "", T0, func([]channels.Channel) (channels.Filing, error) {
		called = true
		return channels.Filing{}, nil
	})
	require.NoError(t, err)
	require.True(t, empty.IsZero())
	require.False(t, called, "nothing moved: the filer does not run")
}

// testFilerRefuses pins that an invalid filing or a filer error moves
// nothing, and that the error comes back as the filer returned it.
func testFilerRefuses(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	mustAppend(t, h.Store, key("TKT-2", "room"), text("four"))
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-2", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, draft, text("five"))

	_, err := h.Store.Tombstone(ctx, "TKT-2", "", T0, channels.FileAs(channels.Filing{Policy: "bogus"}))
	require.ErrorIs(t, err, channels.ErrInvalidMessage)

	refusal := errors.New("a block type is in the batch")
	refuse := func([]channels.Channel) (channels.Filing, error) { return channels.Filing{}, refusal }
	_, err = h.Store.Tombstone(ctx, "TKT-2", "", T0, refuse)
	require.Equal(t, refusal, err, "the filer's error is returned as it is")
	_, err = h.Store.Tombstone(ctx, "TKT-2", "draft", T0, refuse)
	require.Equal(t, refusal, err)

	left, err := h.Store.ListEntity(ctx, "TKT-2")
	require.NoError(t, err)
	require.Len(t, left, 2, "a refused tombstone moves nothing")
	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, got)
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
	early := T0.Add(time.Hour)
	whole := fileBatch(t, h.Store, "TKT-1", "", T0.Add(time.Minute), channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: purgeAfter, Pending: true,
		TypeAfter: map[string]time.Time{"room": early, "late": purgeAfter.Add(time.Hour)},
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
		TypeAfter: map[string]time.Time{"room": channels.NormTime(early)},
	}, got[0].Filing, "a type time at or after the batch's says nothing and is dropped")
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

	mustAppend(t, h.Store, key("TKT-4", "room"), text("four"))
	_, err = h.Store.Tombstone(ctx, "TKT-4", "", T0, channels.FileAs(channels.Filing{
		Policy: channels.PolicyDetach, TypeAfter: map[string]time.Time{"room": T0},
	}))
	require.Error(t, err, "type times need a batch time")
}

// testDueTombstones pins the purge poll's listing: confirmed batches with
// something due, by the batch time or an earlier type time.
func testDueTombstones(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	for _, id := range []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"} {
		mustAppend(t, h.Store, key(id, "room"), text(id))
	}
	mustAppend(t, h.Store, key("TKT-2", "wipe"), text("wipe"))
	soon := fileBatch(t, h.Store, "TKT-1", "", T0, channels.Filing{
		Policy: channels.PolicyDetach, PurgeAfter: T0.Add(time.Hour),
	})
	early := fileBatch(t, h.Store, "TKT-2", "", T0, channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: T0.Add(30 * 24 * time.Hour),
		TypeAfter: map[string]time.Time{"wipe": T0},
	})
	pending := fileBatch(t, h.Store, "TKT-3", "", T0, channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: T0, Pending: true,
	})
	fileBatch(t, h.Store, "TKT-4", "", T0, channels.Filing{})

	got, err := h.Store.DueTombstones(ctx, T0)
	require.NoError(t, err)
	require.Len(t, got, 1, "only the type due at once; pending and unrecorded batches are left out")
	require.Equal(t, early, got[0].Handle)
	require.Len(t, got[0].Channels, 2)

	got, err = h.Store.DueTombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 2, "the bound is inclusive")
	require.NotNil(t, listed(got, soon))

	require.NoError(t, h.Store.ConfirmTombstone(ctx, pending))
	got, err = h.Store.DueTombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 3)
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
	require.ErrorIs(t, h.Store.Untombstone(ctx, b, T0.Add(time.Minute)), channels.ErrConflict,
		"a confirmed delete batch is final")

	mustAppend(t, h.Store, key("TKT-2", "room"), text("y"))
	o := fileBatch(t, h.Store, "TKT-2", "", T0, channels.Filing{Policy: channels.PolicyOrphan})
	require.NoError(t, h.Store.ConfirmTombstone(ctx, o))
	got, err = h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, channels.PolicyDetach, listed(got, o).Filing.Policy, "a confirmed orphan is a detach")
	require.ErrorIs(t, h.Store.Untombstone(ctx, o, T0.Add(time.Minute)), channels.ErrConflict)

	mustAppend(t, h.Store, key("TKT-3", "room"), text("z"))
	n := fileBatch(t, h.Store, "TKT-3", "", T0, channels.Filing{})
	require.NoError(t, h.Store.Untombstone(ctx, n, T0.Add(time.Minute)), "a batch without a policy may come back")
	require.ErrorIs(t, h.Store.ConfirmTombstone(ctx, n), channels.ErrNotFound)
	require.ErrorIs(t, h.Store.ConfirmTombstone(ctx, channels.TombstoneHandle{}), channels.ErrInvalidKey)
}

// testConfirmTombstones pins the per-anchor confirm a new entity reusing
// an id runs: every batch of the id, or the face's batches only.
func testConfirmTombstones(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	final := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "final"}, Type: "room"}
	mustAppend(t, h.Store, draft, text("d"))
	mustAppend(t, h.Store, final, text("f"))
	mustAppend(t, h.Store, key("TKT-2", "room"), text("other"))
	hd := fileBatch(t, h.Store, "TKT-1", "draft", T0, channels.Filing{
		Policy: channels.PolicyDelete, Pending: true,
	})
	hf := fileBatch(t, h.Store, "TKT-1", "final", T0.Add(time.Second), channels.Filing{
		Policy: channels.PolicyOrphan,
	})
	other := fileBatch(t, h.Store, "TKT-2", "", T0, channels.Filing{Policy: channels.PolicyOrphan})

	require.NoError(t, h.Store.ConfirmTombstones(ctx, channels.Anchor{ID: "TKT-1", Face: "draft"}))
	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, listed(got, hd).Filing.Pending)
	require.Equal(t, channels.PolicyOrphan, listed(got, hf).Filing.Policy, "another face is not touched")

	require.NoError(t, h.Store.ConfirmTombstones(ctx, channels.Anchor{ID: "TKT-1"}))
	got, err = h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, channels.PolicyDetach, listed(got, hf).Filing.Policy, "the whole id covers every face")
	require.Equal(t, channels.PolicyOrphan, listed(got, other).Filing.Policy, "another id is not touched")
	require.ErrorIs(t, h.Store.Untombstone(ctx, hf, T0.Add(time.Minute)), channels.ErrConflict)

	require.NoError(t, h.Store.ConfirmTombstones(ctx, channels.Anchor{ID: "TKT-9"}), "no batches is not an error")
	require.ErrorIs(t, h.Store.ConfirmTombstones(ctx, channels.Anchor{}), channels.ErrInvalidKey)
}

func testPurge(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	room := key("TKT-1", "room")
	mustAppend(t, h.Store, room, text("secret of the first life"))
	b := fileBatch(t, h.Store, "TKT-1", "", T0.Add(time.Minute), channels.Filing{
		Policy: channels.PolicyDelete, Pending: true,
	})
	require.ErrorIs(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)), channels.ErrConflict,
		"a batch whose delete has not committed is not purged")
	require.NoError(t, h.Store.ConfirmTombstone(ctx, b))

	// A new entity with the same id has its own channels and trail.
	mustAppend(t, h.Store, room, text("second life"))
	liveBefore, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)

	require.ErrorIs(t, h.Store.Purge(ctx, b, nil, channels.Author{}, T0.Add(time.Hour)), channels.ErrInvalidMessage)
	require.ErrorIs(t, h.Store.Purge(ctx, b, []string{"Bad Type"}, Bob, T0.Add(time.Hour)), channels.ErrInvalidKey)
	require.NoError(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))

	raw := string(h.Raw(t))
	require.NotContains(t, raw, "secret of the first life", "a purge erases the messages")
	require.Contains(t, raw, `"op":"purge"`, "the filed trail keeps a purge record")
	require.Contains(t, raw, `"type":"room"`, "the purge record names the erased channel")
	require.Contains(t, raw, `"tomb":"`+b.Stamp()+`"`, "the purge record names the batch")
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
	require.ErrorIs(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)), channels.ErrNotFound,
		"a batch is purged once")
	require.ErrorIs(t, h.Store.Untombstone(ctx, b, T0.Add(time.Hour)), channels.ErrNotFound,
		"a purged batch cannot come back")
	require.NoError(t, h.Store.Purge(ctx, channels.TombstoneHandle{}, nil, Bob, T0), "the zero handle is a no-op")
}

// testPurgeTypes pins the partial purge: the due types go first, the rest
// stays restorable as a batch, and erasing the last channel ends it.
func testPurgeTypes(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	keep := key("TKT-1", "keep")
	wipe := key("TKT-1", "wipe")
	mustAppend(t, h.Store, keep, text("kept a while"))
	mustAppend(t, h.Store, wipe, text("erased at once"))
	b := fileBatch(t, h.Store, "TKT-1", "", T0, channels.Filing{
		Policy: channels.PolicyDelete, PurgeAfter: T0.Add(30 * 24 * time.Hour),
		TypeAfter: map[string]time.Time{"wipe": T0},
	})

	require.NoError(t, h.Store.Purge(ctx, b, []string{"wipe"}, Bob, T0))
	raw := string(h.Raw(t))
	require.NotContains(t, raw, "erased at once")
	require.Contains(t, raw, "kept a while")
	got, err := h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, []channels.Key{keep}, batchKeys(got[0]))
	require.Nil(t, got[0].Filing.TypeAfter, "the erased type leaves the filing")
	due, err := h.Store.DueTombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, due, "nothing else is due before the batch time")
	trail, err := h.Store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	for _, r := range trail {
		require.NotEqual(t, channels.AuditPurge, r.Op, "a purge record is never in the live trail")
	}

	require.NoError(t, h.Store.Purge(ctx, b, []string{"keep"}, Bob, T0.Add(time.Hour)),
		"erasing the last channel is a whole purge")
	require.NotContains(t, string(h.Raw(t)), "kept a while")
	require.ErrorIs(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)), channels.ErrNotFound)
	got, err = h.Store.Tombstones(ctx, T0.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, got)
}

func testPurgeFace(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	fam := key("TKT-1", "room")
	draft := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	mustAppend(t, h.Store, fam, text("family"))
	mustAppend(t, h.Store, draft, text("draft only"))
	b := fileBatch(t, h.Store, "TKT-1", "draft", T0, channels.Filing{Policy: channels.PolicyDetach})

	require.NoError(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))
	raw := string(h.Raw(t))
	require.NotContains(t, raw, "draft only")
	require.Contains(t, raw, `"tomb":"`+b.Stamp()+`"`, "a face batch keeps a purge record too")
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
	require.ErrorIs(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)), channels.ErrNotFound)
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

	require.NoError(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))
	require.NoError(t, h.Store.DropTypeRecord(ctx, "detached"), "a purged batch no longer holds the type")
	got, err = h.Store.TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]channels.TypeRecord{"room": family}, got)
}
