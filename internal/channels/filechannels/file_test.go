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
	"gopkg.in/yaml.v3"

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

// seedLegacyType adds typ to the anchor-modes file older releases wrote.
func seedLegacyType(s *Store) func(t *testing.T, typ string, anchor channels.AnchorMode) {
	return func(t *testing.T, typ string, anchor channels.AnchorMode) {
		t.Helper()
		modes := map[string]channels.AnchorMode{}
		data, err := s.readFile(legacyTypesFile)
		require.NoError(t, err)
		require.NoError(t, yaml.Unmarshal(data, &modes))
		modes[typ] = anchor
		data, err = yaml.Marshal(modes)
		require.NoError(t, err)
		require.NoError(t, s.rfs.WriteFile(legacyTypesFile, data, filePerm))
	}
}

func TestConformance_Disk(t *testing.T) {
	t.Parallel()
	channelstest.RunAll(t, func(t *testing.T) channelstest.Harness {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "channels")
		s := newDiskStore(t, dir)
		return channelstest.Harness{
			Store:          s,
			Raw:            func(t *testing.T) []byte { t.Helper(); return rawDisk(t, dir) },
			Second:         func(t *testing.T) channels.Store { t.Helper(); return newDiskStore(t, dir) },
			SeedLegacyType: seedLegacyType(s),
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
			Store:          s,
			SeedLegacyType: seedLegacyType(s),
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

// A damaged type record is an error, never an empty record: an empty one
// would let a changed anchor or cardinality through.
func TestDamagedTypeRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	rec := channels.TypeRecord{Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne}
	require.NoError(t, s.RecordTypes(ctx, map[string]channels.TypeRecord{"room": rec}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, typesFile), []byte("room: [unclosed"), 0o600))
	_, err := s.TypeRecords(ctx)
	require.Error(t, err)
	rec.Anchor = channels.AnchorFamily
	err = s.RecordTypes(ctx, map[string]channels.TypeRecord{"room": rec})
	require.Error(t, err)
}

// An unchanged record writes nothing, so recording on every first write
// costs no write.
func TestRecordTypesSkipsUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "channels")
	s := newDiskStore(t, dir)
	rec := map[string]channels.TypeRecord{
		"room": {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne},
	}
	require.NoError(t, s.RecordTypes(ctx, rec))
	path := filepath.Join(dir, typesFile)
	require.NoError(t, os.WriteFile(path, []byte("room: {anchor: face, cardinality: one}\n# kept\n"), 0o600))
	require.NoError(t, s.RecordTypes(ctx, rec))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "# kept", "an unchanged record is not rewritten")
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
	m, err := s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, key, channels.Deletion{
		Expect: channels.StateOpen, Rev: m.Rev, By: channels.Author{User: "alice"}, At: time.Now(),
	}))
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
	_, err = s.Tombstone(ctx, "TKT-1", "draft", time.Now(), nil)
	require.NoError(t, err)
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

// TestMetaWithoutSessionReadsUnset pins that a meta.yaml written before
// sessions, with no session keys, reads as an unset session.
func TestMetaWithoutSessionReadsUnset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, SegmentSize)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	path := "/channels/" + enc("TKT-1") + "/" + none + "/room/" + none + "/" + metaFile
	data, err := mfs.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), "session", "an append without a step writes no session keys")
	require.NoError(t, mfs.WriteFile(path, []byte("state: open\ncreated_by: {user: alice}\n"+
		"created_at: 2026-10-12T09:30:00Z\nlast_seq: 1\nrev: 1\n"), 0o600))
	head, err := s.Head(ctx, key)
	require.NoError(t, err)
	require.True(t, head.Session.IsZero(), "session = %+v", head.Session)
}

// TestSessionStepAcrossSegments pins that a notice whose message is the
// last of a segment goes first in the next one, and that both read back,
// also through a rev cursor.
func TestSessionStepAcrossSegments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	m := msg("two")
	m.Session = &channels.SessionStep{
		Decide: func(channels.Channel) (channels.SessionMove, bool) {
			return channels.SessionMove{From: "detected", To: "contained"}, true
		},
		NoticeType: "session", NoticeBy: channels.Author{User: "system:realtime"}, TriggeredBy: "session:room",
	}
	cause, err := s.Append(ctx, key, m)
	require.NoError(t, err)
	require.Equal(t, int64(2), cause.Seq)

	dir := "/channels/" + enc("TKT-1") + "/" + none + "/room/" + none + "/"
	first, err := mfs.ReadFile(dir + "log-0000.jsonl")
	require.NoError(t, err)
	require.NotContains(t, string(first), "\"type\":\"session\"")
	second, err := mfs.ReadFile(dir + "log-0001.jsonl")
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(second, []byte("{\"k\":\"m\",\"seq\":3,")), "%s", second)

	got, err := s.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "session", got[2].Type)
	byRev, err := s.Messages(ctx, key, channels.Page{AfterRev: 1})
	require.NoError(t, err)
	require.Len(t, byRev, 2)
	require.Equal(t, []int64{2, 3}, []int64{byRev[0].Seq, byRev[1].Seq})
}

// moveStep is a session step that moves detected to contained.
func moveStep() *channels.SessionStep {
	return &channels.SessionStep{
		Decide: func(channels.Channel) (channels.SessionMove, bool) {
			return channels.SessionMove{From: "detected", To: "contained"}, true
		},
		NoticeType: "session", NoticeBy: channels.Author{User: "system:realtime"}, TriggeredBy: "session:room",
	}
}

// TestSessionReplyAcrossSegments pins a reply that moves the session in
// segment 1 to a parent in segment 0: the message, the parent's patch and
// the notice take three revs in that order, and an AfterRev page returns
// all three.
func TestSessionReplyAcrossSegments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := newStore(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	parent, err := s.Append(ctx, key, msg("parent"))
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("other"))
	require.NoError(t, err)
	m := msg("reply")
	m.ReplyTo, m.Session = parent.Seq, moveStep()
	reply, err := s.Append(ctx, key, m)
	require.NoError(t, err)
	require.Equal(t, int64(3), reply.Seq)

	page, err := s.Messages(ctx, key, channels.Page{AfterRev: reply.Rev - 1})
	require.NoError(t, err)
	var seqs, revs []int64
	for _, got := range page {
		seqs, revs = append(seqs, got.Seq), append(revs, got.Rev)
	}
	require.Equal(t, []int64{3, 1, 4}, seqs, "message, then parent, then notice")
	require.Equal(t, []int64{reply.Rev, reply.Rev + 1, reply.Rev + 2}, revs)
	require.Equal(t, 1, page[1].ReplyCount)
}

// TestSessionCrashBetweenSegments pins the crash window the package doc
// names for a move whose notice opens a new segment: meta.yaml and the
// message are written, the notice and the audit record are not. The test
// builds that state by removing what the crash would not have written,
// because the store has no hook to stop between its writes. The session
// stays moved, the message keeps its mark, the next append takes the lost
// seq, and no notice names a wrong cause.
func TestSessionCrashBetweenSegments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	auditPath := "/channels/" + auditKey("TKT-1")
	before, err := mfs.ReadFile(auditPath)
	require.NoError(t, err)
	m := msg("two")
	m.Session = moveStep()
	_, err = s.Append(ctx, key, m)
	require.NoError(t, err)

	dir := "/channels/" + enc("TKT-1") + "/" + none + "/room/" + none + "/"
	require.NoError(t, mfs.Remove(dir+"log-0001.jsonl"))
	require.NoError(t, mfs.WriteFile(auditPath, before, 0o600))

	head, err := s.Head(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "contained", head.Session.State, "meta.yaml was written first")
	next, err := s.Append(ctx, key, msg("three"))
	require.NoError(t, err)
	require.Equal(t, int64(3), next.Seq, "the lost notice's seq is taken again")
	all, err := s.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.True(t, all[1].MovedSession)
	for _, got := range all {
		require.NotEqual(t, "session", got.Type, "no notice survives to name a cause")
	}
	recs, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	for _, r := range recs {
		require.NotEqual(t, channels.AuditSession, r.Op)
	}
}
