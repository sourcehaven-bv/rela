package filechannels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var alice = channels.ReaderOf(channelstest.Alice)

// TestSetRead_NoOpTakesNoLock pins that a SetRead whose seq is not above
// the marker returns without the lock, and that a moving one takes it
// once (TKT-2B2AIA AC 1).
func TestSetRead_NoOpTakesNoLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	locker := &countingLocker{}
	s, err := newStore(storage.NewMemFS(), "/project/.rela/channels", locker, SegmentSize)
	require.NoError(t, err)
	for i := range 3 {
		_, err = s.Append(ctx, key, msg(fmt.Sprint("m", i)))
		require.NoError(t, err)
	}
	_, moved, err := s.SetRead(ctx, key, alice, 2, channelstest.T0)
	require.NoError(t, err)
	require.True(t, moved)
	locks := locker.locks

	for _, seq := range []int64{0, 1, 2} {
		st, again, setErr := s.SetRead(ctx, key, alice, seq, channelstest.T0)
		require.NoError(t, setErr)
		require.False(t, again)
		require.Equal(t, channels.ReadState{Seq: 2, At: channels.NormTime(channelstest.T0), LastSeq: 3, Unread: 1}, st)
	}
	require.Equal(t, locks, locker.locks, "a mark that does not move takes no lock")

	_, moved, err = s.SetRead(ctx, key, alice, 3, channelstest.T0)
	require.NoError(t, err)
	require.True(t, moved)
	require.Equal(t, locks+1, locker.locks)
}

// readCountFS counts the bytes read through files it opens, by path.
type readCountFS struct {
	storage.FS
	mu    sync.Mutex
	bytes map[string]int
}

func (c *readCountFS) Open(p string) (io.ReadCloser, error) {
	f, err := c.FS.Open(p)
	if err != nil {
		return nil, err
	}
	ra, ok := f.(interface {
		io.ReadCloser
		io.ReaderAt
	})
	if !ok {
		return f, nil
	}
	return &countedFile{ReadCloser: ra, ra: ra, add: func(n int) {
		c.mu.Lock()
		c.bytes[p] += n
		c.mu.Unlock()
	}}, nil
}

func (c *readCountFS) read(suffix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for p, b := range c.bytes {
		if strings.HasSuffix(p, suffix) {
			n += b
		}
	}
	return n
}

func (c *readCountFS) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bytes = map[string]int{}
}

type countedFile struct {
	io.ReadCloser
	ra  io.ReaderAt
	add func(int)
}

func (f *countedFile) Read(p []byte) (int, error) {
	n, err := f.ReadCloser.Read(p)
	f.add(n)
	return n, err
}

func (f *countedFile) ReadAt(p []byte, off int64) (int, error) {
	n, err := f.ra.ReadAt(p, off)
	f.add(n)
	return n, err
}

// TestReads_TailCost pins that the file backend's Reads cost follows the
// unread messages, not the segment (TKT-2B2AIA AC 2): a caught-up key
// reads one tail chunk of its log, and so does a 2 000-message channel
// with one unread message.
func TestReads_TailCost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfs := &readCountFS{FS: storage.NewOsFS(), bytes: map[string]int{}}
	s, err := newStore(cfs, filepath.Join(t.TempDir(), "channels"), &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	const n = 2000
	for i := range n {
		_, err = s.Append(ctx, key, msg(fmt.Sprint("message ", i, " ", strings.Repeat("x", 600))))
		require.NoError(t, err)
	}
	size, err := s.rfs.Stat(segKey(key, s.idx(n)))
	require.NoError(t, err)
	require.Greater(t, size.Size(), int64(4*tailChunk), "the segment must be much larger than a tail chunk")

	_, _, err = s.SetRead(ctx, key, alice, n, channelstest.T0)
	require.NoError(t, err)
	cfs.reset()
	got, err := s.Reads(ctx, alice, []channels.Key{key})
	require.NoError(t, err)
	require.Zero(t, got[key].Unread)
	require.LessOrEqual(t, cfs.read(".jsonl"), tailChunk, "a caught-up key reads one tail chunk")

	_, err = s.Append(ctx, key, msg("one more"))
	require.NoError(t, err)
	cfs.reset()
	got, err = s.Reads(ctx, alice, []channels.Key{key})
	require.NoError(t, err)
	require.Equal(t, 1, got[key].Unread)
	require.LessOrEqual(t, cfs.read(".jsonl"), 2*tailChunk, "one unread message reads one tail chunk per scan")

	t.Run("non-counting run above the marker", func(t *testing.T) {
		t.Parallel()
		// TKT-O6RC8Z AC 4: 2 000 messages of a type with unread: false sit
		// above the marker in one segment, and the marker is the last seq
		// of the previous segment. The scan reads no byte of the previous
		// segment. The doubling tail read and the head lookup together stay
		// within three times the run's segment.
		const run = 2000
		cfs := &readCountFS{FS: storage.NewOsFS(), bytes: map[string]int{}}
		s, err := newStore(cfs, filepath.Join(t.TempDir(), "channels"), &countingLocker{}, run)
		require.NoError(t, err)
		for i := range run {
			_, err = s.Append(ctx, key, msg(fmt.Sprint("read ", i)))
			require.NoError(t, err)
		}
		_, _, err = s.SetRead(ctx, key, alice, run, channelstest.T0)
		require.NoError(t, err)
		for i := range run {
			m := msg(fmt.Sprint("progress ", i))
			m.NoUnread = true
			_, err = s.Append(ctx, key, m)
			require.NoError(t, err)
		}
		size, err := s.rfs.Stat(segKey(key, 1))
		require.NoError(t, err)
		cfs.reset()
		got, err := s.Reads(ctx, alice, []channels.Key{key})
		require.NoError(t, err)
		require.Zero(t, got[key].Unread, "non-counting messages do not count")
		require.Zero(t, cfs.read(filepath.Base(segKey(key, 0))), "no byte of the marker's segment is read")
		require.LessOrEqual(t, cfs.read(filepath.Base(segKey(key, 1))), int(3*size.Size()),
			"the run's segment is read a bounded number of times")
	})
}

// readsFailFS fails every write of a reads.json.
type readsFailFS struct {
	storage.FS
}

func (f *readsFailFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	if strings.HasSuffix(p, "/"+readsFile) {
		return errors.New("injected reads.json failure")
	}
	return f.FS.WriteFile(p, data, perm)
}

// TestAppend_MarkReadWriteFailure pins that a failed reads.json write
// after the log records does not fail the post (TKT-2B2AIA AC 4): Append
// returns the stored message, and the marker is unchanged.
func TestAppend_MarkReadWriteFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	dir := "/project/.rela/channels"
	ok, err := newStore(mfs, dir, &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	_, err = ok.Append(ctx, key, msg("first"))
	require.NoError(t, err)
	_, _, err = ok.SetRead(ctx, key, alice, 1, channelstest.T0)
	require.NoError(t, err)

	failing, err := newStore(&readsFailFS{FS: mfs}, dir, &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	m := msg("second")
	m.MarkRead = true
	got, err := failing.Append(ctx, key, m)
	require.NoError(t, err, "a stored post is never reported as failed")
	require.Equal(t, int64(2), got.Seq)
	head, err := ok.Head(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), head.LastSeq)
	st, err := ok.Reads(ctx, alice, []channels.Key{key})
	require.NoError(t, err)
	require.Equal(t, int64(1), st[key].Seq, "the marker is unchanged")
}
