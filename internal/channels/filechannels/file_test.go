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
// process would. It also gets its own semaphore: two stores in one process
// over one directory normally share one, which would hide a broken file
// lock from the two-instance tests. With separate semaphores only the file
// lock keeps them apart, as it must keep two processes apart.
func newDiskStore(t *testing.T, dir string) *Store {
	t.Helper()
	locker, err := filelock.New(filepath.Join(filepath.Dir(dir), filepath.Base(dir)+".lock"))
	require.NoError(t, err)
	s, err := newStore(storage.NewOsFS(), dir, locker, SegmentSize)
	require.NoError(t, err)
	s.sem = make(chan struct{}, 1)
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

// Two stores over one directory in one process share a semaphore, so a
// process-private locker still excludes them from each other.
func TestStoresShareSemaphore(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "channels")
	a, err := New(storage.NewOsFS(), dir, filelock.ProcessPrivate{})
	require.NoError(t, err)
	b, err := New(storage.NewOsFS(), dir+"/../channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	require.Equal(t, a.sem, b.sem)
}

// Patches to one segment are folded into a rewrite once they pass the
// threshold, so replies to an old message do not grow its segment forever.
func TestPatchCompaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, 4)
	require.NoError(t, err)
	parent, err := s.Append(ctx, key, msg("parent"))
	require.NoError(t, err)
	maxPatches := 0
	for range 40 {
		reply := msg("reply")
		reply.ReplyTo = parent.Seq
		_, err = s.Append(ctx, key, reply)
		require.NoError(t, err)
		seg, segErr := s.segment(key, 0)
		require.NoError(t, segErr)
		maxPatches = max(maxPatches, patchCount(seg))
	}
	require.LessOrEqual(t, maxPatches, s.compactAfter())
	got, err := s.Get(ctx, key, parent.Seq)
	require.NoError(t, err)
	require.Equal(t, 40, got.ReplyCount)
}

// lastMessage finds the last message record behind patch records, past a
// torn line and beyond the first chunk it reads, on a filesystem that reads
// at an offset and on one that does not.
func TestLastMessageReadsFromTheEnd(t *testing.T) {
	t.Parallel()
	var body strings.Builder
	body.WriteString(`{"k":"m","seq":1,"rev":1}` + "\n")
	body.WriteString(`{"k":"m","seq":2,"rev":2}` + "\n")
	for body.Len() < 3*tailChunk {
		body.WriteString(`{"k":"p","seq":1,"rev":3,"p_reply_count":1}` + "\n")
	}
	body.WriteString("not json\n")
	body.WriteString(`{"k":"m","seq":3,"rev":4,"props":{"body":"torn`)

	stores := map[string]*Store{}
	disk := filepath.Join(t.TempDir(), "channels")
	stores["disk"] = newDiskStore(t, disk)
	mem, err := newStore(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{}, SegmentSize)
	require.NoError(t, err)
	stores["memory"] = mem
	for name, s := range stores {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, s.rfs.MkdirAll(chanDir(key), 0o700))
			require.NoError(t, s.rfs.WriteFile(segKey(key, 0), []byte(body.String()), filePerm))
			seq, ok, err := s.lastMessage(segKey(key, 0))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, int64(2), seq)

			require.NoError(t, s.rfs.WriteFile(segKey(key, 1), []byte(`{"k":"p","seq":1,"rev":5}`+"\n"), filePerm))
			last, err := s.lastSeq(key)
			require.NoError(t, err)
			require.Equal(t, int64(2), last, "a segment with no message record is walked past")

			_, ok, err = s.lastMessage("missing.jsonl")
			require.NoError(t, err)
			require.False(t, ok)
		})
	}
}

// An audit trail with a torn final line is repaired by the next record,
// which reads only the last byte when the trail is intact.
func TestTornAuditIsRepaired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	_, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	audit := filepath.Join(dir, auditKey("TKT-1"))
	f, err := os.OpenFile(audit, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"at":"2026-10-12T09:00:00Z","op":"rem`)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = s.CreateMany(ctx, key.Anchor, channels.NewChannel{
		Type: "topic", By: channels.Author{User: "a"}, At: time.Now(),
	})
	require.NoError(t, err)
	recs, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, recs, 2)
	data, err := os.ReadFile(audit)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"op":"rem`)
}
