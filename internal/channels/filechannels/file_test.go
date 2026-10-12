package filechannels

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// newDiskStore opens a Store over dir with its own file lock, as one
// process would.
func newDiskStore(t *testing.T, dir string) *Store {
	t.Helper()
	locker, err := filelock.New(filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".lock"))
	require.NoError(t, err)
	s, err := newStore(storage.NewSafeFS(storage.NewOsFS()), dir, locker, SegmentSize)
	require.NoError(t, err)
	return s
}

// rawDisk concatenates every file under dir.
func rawDisk(t *testing.T, dir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		buf.WriteString(p + "\n")
		buf.Write(b)
		return nil
	})
	require.NoError(t, err)
	return buf.Bytes()
}

func TestConformance_Disk(t *testing.T) {
	t.Parallel()
	channelstest.RunAll(t, func(t *testing.T) channelstest.Harness {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "channels")
		return channelstest.Harness{
			Store:  newDiskStore(t, dir),
			Raw:    func(t *testing.T) []byte { t.Helper(); return rawDisk(t, dir) },
			Second: func(t *testing.T) channels.Store { t.Helper(); return newDiskStore(t, dir) },
		}
	})
}

// The in-memory run uses four-message segments, so every multi-message
// test crosses segment boundaries.
func TestConformance_MemFS(t *testing.T) {
	t.Parallel()
	channelstest.RunAll(t, func(t *testing.T) channelstest.Harness {
		t.Helper()
		mfs := storage.NewMemFS()
		dir := "/project/.rela/channels-" + strings.ReplaceAll(t.Name(), "/", "-")
		s, err := newStore(mfs, dir, filelock.ProcessPrivate{}, 4)
		require.NoError(t, err)
		rfs, err := storage.NewRootedFS(mfs, dir)
		require.NoError(t, err)
		return channelstest.Harness{
			Store: s,
			Raw: func(t *testing.T) []byte {
				t.Helper()
				var buf bytes.Buffer
				err := rfs.WalkAll(func(p string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return err
					}
					b, err := rfs.ReadFile(p)
					buf.Write(b)
					return err
				})
				if !os.IsNotExist(err) {
					require.NoError(t, err)
				}
				return buf.Bytes()
			},
		}
	})
}

func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := New(nil, t.TempDir(), filelock.ProcessPrivate{})
	require.Error(t, err)
	_, err = New(storage.NewMemFS(), "/x", nil)
	require.Error(t, err)
	_, err = New(storage.NewMemFS(), "", filelock.ProcessPrivate{})
	require.Error(t, err)
}

var key = channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}

func msg(body string) channels.NewMessage {
	return channels.NewMessage{
		Type: "text", Author: channelstest.Alice, At: channelstest.T0, Props: map[string]any{"body": body},
	}
}

// A crash mid-append leaves a torn final line. Readers ignore it and the
// next append cuts it off, so the seq stays gapless.
func TestTornTail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	for _, b := range []string{"one", "two"} {
		_, err := s.Append(ctx, key, msg(b))
		require.NoError(t, err)
	}
	seg := filepath.Join(dir, enc("TKT-1"), none, "room", none, "log-0000.jsonl")
	f, err := os.OpenFile(seg, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"k":"m","seq":3,"rev":3,"type":"text","props":{"body":"tor`)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	reopened := newDiskStore(t, dir)
	got, err := reopened.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	head, err := reopened.Head(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), head.LastSeq)

	m, err := reopened.Append(ctx, key, msg("three"))
	require.NoError(t, err)
	require.Equal(t, int64(3), m.Seq)
	data, err := os.ReadFile(seg)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"tor`)
	require.Equal(t, 3, bytes.Count(data, []byte("\n")))
	got, err = reopened.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, "three", got[2].Props["body"])
}

// A crash after meta.yaml but before the record leaves meta ahead of the
// log. The log is the truth, so the seq is not skipped.
func TestMetaAheadOfLog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	_, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	m, err := s.readMeta(key)
	require.NoError(t, err)
	m.LastSeq, m.Rev = 5, 9
	require.NoError(t, s.writeMeta(key, m))

	head, err := s.Head(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.LastSeq)
	got, err := s.Append(ctx, key, msg("two"))
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Seq)
	require.Equal(t, int64(10), got.Rev, "a reserved rev is never reused")
}

// A damaged anchor-mode record is an error, never an empty record: an empty
// one would let a changed anchor through.
func TestDamagedAnchorModes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	require.NoError(t, s.RecordAnchorModes(ctx, map[string]channels.AnchorMode{"room": channels.AnchorFace}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, modesFile), []byte("room: [unclosed"), 0o600))
	_, err := s.AnchorModes(ctx)
	require.Error(t, err)
	err = s.RecordAnchorModes(ctx, map[string]channels.AnchorMode{"room": channels.AnchorFamily})
	require.Error(t, err)
}

func TestDamagedRecordsAreSkipped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	_, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	seg := filepath.Join(dir, enc("TKT-1"), none, "room", none, "log-0000.jsonl")
	f, err := os.OpenFile(seg, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString("not json\n{\"k\":\"p\",\"seq\":77,\"rev\":4}\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	audit := filepath.Join(dir, auditDir, enc("TKT-1")+".jsonl")
	require.NoError(t, os.WriteFile(audit, append(must(os.ReadFile(audit)), []byte("garbage\n")...), 0o600))

	got, err := s.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	recs, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, recs, 1)

	require.NoError(t, os.WriteFile(filepath.Join(dir, enc("TKT-1"), none, "room", none, metaFile), []byte(":\t:"), 0o600))
	_, err = s.Head(ctx, key)
	require.ErrorContains(t, err, "damaged")
}

// A delete interrupted after moving a channel to the trash leaves content
// there; the next delete removes it.
func TestDeleteEmptiesTrash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	left := filepath.Join(dir, trashDir, "interrupted")
	require.NoError(t, os.MkdirAll(left, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(left, "log-0000.jsonl"), []byte("secret\n"), 0o600))
	_, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, key, channels.Author{User: "alice"}, time.Now()))
	require.NoDirExists(t, filepath.Join(dir, trashDir))
}

// A face tombstone leaves no empty entity directory behind.
func TestFaceTombstonePrunes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	faced := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}
	_, err := s.Append(ctx, faced, msg("one"))
	require.NoError(t, err)
	require.NoError(t, s.Tombstone(ctx, "TKT-1", "draft", time.Now()))
	require.NoDirExists(t, filepath.Join(dir, enc("TKT-1")))
}

// Directories that do not decode to a valid key are not channels.
func TestStrayDirectoriesAreIgnored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	_, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	for _, p := range []string{
		"zz9",
		enc("TKT-1") + "/zz9",
		enc("TKT-1") + "/" + none + "/Bad",
		enc("TKT-1") + "/" + none + "/room/BAD",
		enc("TKT-1") + "/" + none + "/room/abc",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, p), 0o755))
	}
	keys, err := s.Keys(ctx)
	require.NoError(t, err)
	require.Equal(t, []channels.Key{key}, keys)
}

func TestLockHonoursContext(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	unlock, err := s.lock(context.Background())
	require.NoError(t, err)
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.Append(ctx, key, msg("x"))
	require.ErrorIs(t, err, context.Canceled)
}

func TestEncodingRoundTrip(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"TKT-1", "tkt-1", "A", "x_y.z"} {
		got, ok := dec(enc(id))
		require.True(t, ok)
		require.Equal(t, id, got)
		require.Equal(t, strings.ToLower(enc(id)), enc(id))
	}
	require.NotEqual(t, enc("TKT-1"), enc("tkt-1"), "ids that differ in case get different directories")
	_, ok := dec("!!")
	require.False(t, ok)
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}
