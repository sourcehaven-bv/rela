package filelock

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew_RejectsEmptyPath(t *testing.T) {
	t.Parallel()
	_, err := New("")
	require.Error(t, err)
}

func TestLock_WaitEndsWithContext(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x.lock")
	holder, err := New(path)
	require.NoError(t, err)
	unlock, err := holder.Lock(context.Background())
	require.NoError(t, err)
	defer unlock()

	waiter, err := New(path)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = waiter.Lock(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded, "a held lock must not be taken twice")
}

func TestLock_ReleaseLetsTheNextHolderIn(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "x.lock")
	a, err := New(path)
	require.NoError(t, err)
	b, err := New(path)
	require.NoError(t, err)

	unlock, err := a.Lock(context.Background())
	require.NoError(t, err)
	unlock()
	unlock, err = b.Lock(context.Background())
	require.NoError(t, err, "a released lock must be free for another locker")
	unlock()
}

// TryLock fails at once while another locker holds the lock, and takes it
// once that locker lets go.
func TestTryLock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "x.lock")
	a, err := New(path)
	require.NoError(t, err)
	b, err := New(path)
	require.NoError(t, err)

	release, ok, err := a.TryLock()
	require.NoError(t, err)
	require.True(t, ok)
	_, ok, err = b.TryLock()
	require.NoError(t, err)
	require.False(t, ok, "a held lock must not be taken twice")
	release()
	release, ok, err = b.TryLock()
	require.NoError(t, err)
	require.True(t, ok)
	release()

	blocked := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	c, err := New(filepath.Join(blocked, "x.lock"))
	require.NoError(t, err)
	_, _, err = c.TryLock()
	require.Error(t, err)
}

// The lock file is created private to the user and is never truncated, so a
// file that happens to have content keeps it.
func TestLock_FileIsPrivateAndNeverTruncated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x.lock")
	l, err := New(path)
	require.NoError(t, err)
	unlock, err := l.Lock(context.Background())
	require.NoError(t, err)
	unlock()
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	require.NoError(t, os.WriteFile(path, []byte("keep"), 0o600))
	unlock, err = l.Lock(context.Background())
	require.NoError(t, err)
	unlock()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "keep", string(data))
}

func TestLock_RefusesSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("O_NOFOLLOW is unix-only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(dir, "x.lock")
	require.NoError(t, os.Symlink(target, link))

	l, err := New(link)
	require.NoError(t, err)
	_, err = l.Lock(context.Background())
	require.Error(t, err, "the lock file must not be opened through a symlink")
}

func TestLock_DirectoryCreationFails(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	l, err := New(filepath.Join(blocker, "x.lock"))
	require.NoError(t, err)
	_, err = l.Lock(context.Background())
	require.Error(t, err)
}
