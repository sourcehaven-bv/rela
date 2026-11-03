package filechannels

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

var (
	retT0  = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	retBob = channels.Author{User: "bob"}
)

// confirmedBatch tombstones a channel of id@face and files it confirmed and
// due at retT0.
func confirmedBatch(t *testing.T, s *Store, id, face string) channels.TombstoneHandle {
	t.Helper()
	ctx := context.Background()
	k := channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}
	if face != "" {
		k.Face = entity.Face(face)
	}
	_, err := s.Append(ctx, k, msg("secret of "+id))
	require.NoError(t, err)
	h, err := s.Tombstone(ctx, id, k.Face, retT0, channels.FileAs(channels.Filing{
		Policy: channels.PolicyDetach, PurgeAfter: retT0,
	}))
	require.NoError(t, err)
	return h
}

// purgeRecords counts the purge records in the purged trail of id.
func purgeRecords(t *testing.T, dir, id string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, purgedAuditKey(id)))
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return bytes.Count(data, []byte(`"op":"purge"`))
}

// One damaged filing is skipped and logged; every other batch is still
// listed and purged.
func TestDamagedFilingIsSkipped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	bad := confirmedBatch(t, s, "TKT-1", "")
	good := confirmedBatch(t, s, "TKT-2", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, filingKey(bad)), []byte("id: [unclosed"), 0o600))

	got, err := s.Tombstones(ctx, retT0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, good, got[0].Handle)
	due, err := s.DueTombstones(ctx, retT0)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, good, due[0].Handle)
	require.NoError(t, s.Purge(ctx, good, nil, retBob, retT0))
	require.Error(t, s.Purge(ctx, bad, nil, retBob, retT0), "the damaged batch itself fails closed")
}

// A whole purge cut short after its records were written completes on the
// next Purge, without a second copy of the records or the trail.
func TestPurgeRetryWritesRecordsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	h := confirmedBatch(t, s, "TKT-1", "")
	batch, err := batchChannels(&s.disk, h, "")
	require.NoError(t, err)
	filed, err := s.readFile(detachedAuditKey(h))
	require.NoError(t, err)

	// Step 1 ran, then the process died.
	require.NoError(t, s.appendPurgeRecords(h, batch, true, filed, retBob, retT0))
	require.NoError(t, s.Purge(ctx, h, nil, retBob, retT0))
	require.Equal(t, 1, purgeRecords(t, dir, "TKT-1"))
	data, err := os.ReadFile(filepath.Join(dir, purgedAuditKey("TKT-1")))
	require.NoError(t, err)
	require.Equal(t, 1, bytes.Count(data, []byte(`"op":"tombstone"`)), "the filed trail is copied once")
	require.NotContains(t, string(rawDisk(t, dir)), "secret of TKT-1")
}

// A face purge cut short after its batch moved to the trash keeps its
// record, and the next Purge removes the filing it left.
func TestPurgeRetryAfterTrash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	h := confirmedBatch(t, s, "TKT-1", "draft")
	batch, err := batchChannels(&s.disk, h, "")
	require.NoError(t, err)

	// Steps 1 and 2 ran, then the process died.
	require.NoError(t, s.appendPurgeRecords(h, batch, true, nil, retBob, retT0))
	require.NoError(t, s.trashErased(h, batch, true, true))
	require.ErrorIs(t, s.Purge(ctx, h, nil, retBob, retT0), channels.ErrNotFound)
	require.Equal(t, 1, purgeRecords(t, dir, "TKT-1"), "the record survives the crash")
	require.NoFileExists(t, filepath.Join(dir, filingKey(h)), "the filing left behind is removed")
}

// A filing written by a Tombstone that died before moving its batch is
// collected by the purge poll.
func TestLeakedFilingIsCollected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	h := channels.NewTombstoneHandle(channels.Anchor{ID: "TKT-1"}, channels.TombstoneStamp(retT0))
	require.NoError(t, writeFiling(&s.disk, h, channels.Filing{Policy: channels.PolicyDetach, PurgeAfter: retT0}))
	kept := confirmedBatch(t, s, "TKT-2", "")

	due, err := s.DueTombstones(ctx, retT0)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, kept, due[0].Handle)
	require.NoFileExists(t, filepath.Join(dir, filingKey(h)))
	require.FileExists(t, filepath.Join(dir, filingKey(kept)))
}
