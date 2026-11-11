package filechannels

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const filesRoot = "/project/.rela/channels"

func withFile(body, name, data string) channels.NewMessage {
	m := msg(body)
	m.Files = []channels.NewFile{{Property: "shot", Name: name, MIME: "text/plain", Data: []byte(data)}}
	return m
}

// heldLocker reports whether its lock is held.
type heldLocker struct {
	mu   sync.Mutex
	held bool
}

func (l *heldLocker) Lock(context.Context) (func(), error) {
	l.mu.Lock()
	l.held = true
	l.mu.Unlock()
	return func() {
		l.mu.Lock()
		l.held = false
		l.mu.Unlock()
	}, nil
}

func (l *heldLocker) isHeld() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held
}

// trashWatchFS records each removal under the trash taken without the
// lock of locker.
type trashWatchFS struct {
	storage.FS
	locker   *heldLocker
	mu       sync.Mutex
	unlocked []string
	removed  int
}

func (f *trashWatchFS) Remove(p string) error {
	if strings.Contains(p, "/"+trashDir) {
		f.mu.Lock()
		f.removed++
		if !f.locker.isHeld() {
			f.unlocked = append(f.unlocked, p)
		}
		f.mu.Unlock()
	}
	return f.FS.Remove(p)
}

// TestRemoveEmptiesTrashUnderLock pins that a removal of a message with
// files empties the trash while it holds the lock. Delete and purge
// create the trash, move a channel into it and empty it under the lock;
// an unlocked walk could remove the trash between their steps.
func TestRemoveEmptiesTrashUnderLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	locker := &heldLocker{}
	wfs := &trashWatchFS{FS: storage.NewMemFS(), locker: locker}
	s, err := newStore(wfs, filesRoot, locker, SegmentSize)
	require.NoError(t, err)

	m, err := s.Append(ctx, key, withFile("one", "a.txt", "secret"))
	require.NoError(t, err)
	_, err = s.Remove(ctx, key, m.Seq, channels.Removal{By: channelstest.Bob, At: channelstest.T0})
	require.NoError(t, err)

	require.NotZero(t, wfs.removed, "the removal emptied the trash")
	require.Empty(t, wfs.unlocked, "the trash was emptied without the lock")
	_, err = wfs.Stat(filesRoot + "/" + trashDir)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// logFailFS fails every write of a log segment while fail is set.
type logFailFS struct {
	storage.FS
	fail bool
}

func (f *logFailFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	if f.fail && strings.HasPrefix(path.Base(p), "log-") {
		return errors.New("injected log failure")
	}
	return f.FS.WriteFile(p, data, perm)
}

// TestAppendFilesWithoutLogRecord pins that no bytes stay under a seq the
// log does not hold: a failed append discards the files it placed, and
// an append without files clears what a crashed append left at its seq.
func TestAppendFilesWithoutLogRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ffs := &logFailFS{FS: storage.NewMemFS()}
	s, err := newStore(ffs, filesRoot, &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	seq2 := filesRoot + "/" + filesKey(key, 2)

	ffs.fail = true
	_, err = s.Append(ctx, key, withFile("two", "a.txt", "secret"))
	require.ErrorContains(t, err, "injected log failure")
	_, err = ffs.Stat(seq2)
	require.ErrorIs(t, err, fs.ErrNotExist, "the failed append discarded its files")
	_, err = ffs.Stat(filesRoot + "/" + trashDir)
	require.ErrorIs(t, err, fs.ErrNotExist, "the discard emptied the trash")

	// A crash between placing the files and the log record.
	ffs.fail = false
	require.NoError(t, ffs.MkdirAll(seq2, 0o755))
	require.NoError(t, ffs.WriteFile(seq2+"/left", []byte("secret"), filePerm))
	got, err := s.Append(ctx, key, msg("two"))
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Seq)
	_, err = ffs.Stat(seq2)
	require.ErrorIs(t, err, fs.ErrNotExist, "the append cleared the files left at its seq")
}
