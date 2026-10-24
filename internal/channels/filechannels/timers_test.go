package filechannels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// countingFS counts the reads a store makes: walks, directory reads and
// file reads by path.
type countingFS struct {
	storage.FS
	mu       sync.Mutex
	walks    int
	readDirs int
	files    []string
}

func (c *countingFS) ReadDir(p string) ([]fs.DirEntry, error) {
	c.mu.Lock()
	c.readDirs++
	c.mu.Unlock()
	return c.FS.ReadDir(p)
}

func (c *countingFS) ReadFile(p string) ([]byte, error) {
	c.mu.Lock()
	c.files = append(c.files, p)
	c.mu.Unlock()
	return c.FS.ReadFile(p)
}

func (c *countingFS) Open(p string) (io.ReadCloser, error) {
	c.mu.Lock()
	c.files = append(c.files, p)
	c.mu.Unlock()
	return c.FS.Open(p)
}

func (c *countingFS) Walk(root string, fn fs.WalkDirFunc) error {
	c.mu.Lock()
	c.walks++
	c.mu.Unlock()
	return c.FS.Walk(root, fn)
}

func (c *countingFS) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.walks, c.readDirs, c.files = 0, 0, nil
}

// countingLocker counts the store locks taken.
type countingLocker struct {
	mu    sync.Mutex
	locks int
}

func (l *countingLocker) Lock(context.Context) (func(), error) {
	l.mu.Lock()
	l.locks++
	l.mu.Unlock()
	return func() {}, nil
}

func (l *countingLocker) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locks
}

// dueStep is a session step that moves an unset session to contained,
// with due as its schedule.
func dueStep(due time.Time) *channels.SessionStep {
	return &channels.SessionStep{
		Decide: func(c channels.Channel) (channels.SessionMove, bool) {
			if !c.Session.IsZero() {
				return channels.SessionMove{}, false
			}
			return channels.SessionMove{From: "detected", To: "contained"}, true
		},
		NoticeType: "session", NoticeBy: channels.Author{User: "system:realtime"}, TriggeredBy: "session:room",
		Due: func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{NextDueAt: due} },
	}
}

// TestMetaWithoutScheduleReadsEmpty pins that a meta.yaml written before
// timers, with a session and no schedule keys, reads with no due time and
// no restart anchors.
func TestMetaWithoutScheduleReadsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, SegmentSize)
	require.NoError(t, err)
	m := msg("one")
	m.Session = dueStep(time.Time{})
	_, err = s.Append(ctx, key, m)
	require.NoError(t, err)
	path := "/channels/" + enc("TKT-1") + "/" + none + "/room/" + none + "/" + metaFile
	data, err := mfs.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), "next_due", "a session without a due time writes no schedule keys")
	require.NoError(t, mfs.WriteFile(path, []byte("state: open\ncreated_by: {user: alice}\n"+
		"created_at: 2026-10-12T09:30:00Z\nlast_seq: 2\nrev: 2\nsession_state: contained\nsession_gen: 1\n"+
		"session_entered_at: 2026-10-12T09:30:00Z\n"), 0o600))
	head, err := s.Head(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "contained", head.Session.State)
	require.True(t, head.Session.NextDueAt.IsZero())
	require.Nil(t, head.Session.Restarts)
}

// TestListSessionsDue_Budget pins the cost of the due listing (TKT-INM3AZ
// AC 9): one call for two timer types makes one tree walk, reading the
// root once, each entity and face directory once, each listed type
// directory once and one meta.yaml per channel of the listed types. It
// reads no log segment, no audit file and no meta.yaml of another type,
// and takes no store lock.
func TestListSessionsDue_Budget(t *testing.T) {
	t.Parallel()
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			cfs := &countingFS{FS: storage.NewMemFS()}
			locker := &countingLocker{}
			s, err := newStore(cfs, "/channels", locker, SegmentSize)
			require.NoError(t, err)
			due := channelstest.T0.Add(time.Hour)
			for i := range n {
				a := channels.Anchor{ID: fmt.Sprintf("TKT-%d", i)}
				for _, typ := range []string{"breach", "incident", "room"} {
					m := msg("x")
					if i%2 == 0 {
						m.Session = dueStep(due)
					}
					_, err = s.Append(ctx, channels.Key{Anchor: a, Type: typ}, m)
					require.NoError(t, err)
				}
			}
			cfs.reset()
			locksBefore := locker.count()

			got, err := s.ListSessionsDue(ctx, []string{"breach", "incident"}, due, 1000)
			require.NoError(t, err)
			require.Len(t, got, n) // half the entities, two types each

			require.Equal(t, 1, cfs.walks, "one walk of the root")
			// Per entity: its directory, its face directory and two type
			// directories.
			require.Equal(t, 4*n, cfs.readDirs)
			require.Len(t, cfs.files, 2*n, "one meta.yaml per channel of the listed types")
			for _, p := range cfs.files {
				require.True(t, strings.HasSuffix(p, "/"+metaFile), "read %s", p)
				require.NotContains(t, p, "/room/", "read another type's meta.yaml")
			}
			require.Equal(t, locksBefore, locker.count(), "the listing takes no lock")
		})
	}
}

// TestListSessionsDue_DamagedMeta pins that a damaged meta.yaml is skipped
// and the other channels are still listed.
func TestListSessionsDue_DamagedMeta(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, SegmentSize)
	require.NoError(t, err)
	due := channelstest.T0.Add(time.Hour)
	for _, id := range []string{"TKT-1", "TKT-2", "TKT-3"} {
		m := msg("x")
		m.Session = dueStep(due)
		_, err = s.Append(ctx, channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}, m)
		require.NoError(t, err)
	}
	path := "/channels/" + enc("TKT-2") + "/" + none + "/room/" + none + "/" + metaFile
	require.NoError(t, mfs.WriteFile(path, []byte("state: [open\n"), 0o600))

	got, err := s.ListSessionsDue(ctx, []string{"room"}, due, 10)
	require.NoError(t, err)
	ids := make([]string, len(got))
	for i, c := range got {
		ids[i] = c.Key.ID
	}
	require.Equal(t, []string{"TKT-1", "TKT-3"}, ids)
}

// restoreFailFS injects the failures a crash during a restore would
// cause: with failMove the rename that moves a batch back out of
// ~detached fails; with failLive every write of a live meta.yaml does.
type restoreFailFS struct {
	storage.FS
	failMove, failLive bool
}

func (f *restoreFailFS) Rename(from, to string) error {
	if f.failMove && strings.Contains(from, detachedDir) {
		return errors.New("injected crash before the move")
	}
	return f.FS.Rename(from, to)
}

func (f *restoreFailFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	if f.failLive && strings.HasSuffix(p, "/"+metaFile) && !strings.Contains(p, detachedDir) {
		return errors.New("injected crash after the move")
	}
	return f.FS.WriteFile(p, data, perm)
}

// TestUntombstone_ResumeBeforeMove pins that the resumed schedule is
// written while the batch is still filed: a restore that stops before the
// move leaves the channel tombstoned, and a restore writes no live
// meta.yaml after the move, so no crash brings the channel back without
// its restart anchors. It covers a whole-entity batch and a face batch.
func TestUntombstone_ResumeBeforeMove(t *testing.T) {
	t.Parallel()
	for _, face := range []entity.Face{"", "draft"} {
		t.Run("face="+string(face), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ffs := &restoreFailFS{FS: storage.NewMemFS()}
			s, err := newStore(ffs, "/channels", filelock.ProcessPrivate{}, SegmentSize)
			require.NoError(t, err)
			k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1", Face: face}, Type: "room"}
			m := msg("x")
			m.Session = dueStep(channelstest.T0.Add(time.Hour))
			_, err = s.Append(ctx, k, m)
			require.NoError(t, err)
			h, err := s.Tombstone(ctx, "TKT-1", face, channelstest.T0, nil)
			require.NoError(t, err)

			resume := func(at time.Time) channels.ScheduleFunc {
				return func(channels.Channel) channels.SessionSchedule {
					return channels.SessionSchedule{
						NextDueAt: at.Add(time.Hour), Restarts: map[string]time.Time{"stale": at},
					}
				}
			}
			first := channelstest.T0.Add(2 * time.Hour)
			ffs.failMove = true
			require.Error(t, s.Untombstone(ctx, h, first, resume(first)))
			_, err = s.Head(ctx, k)
			require.ErrorIs(t, err, channels.ErrNotFound, "a failed restore leaves the channel tombstoned")

			ffs.failMove, ffs.failLive = false, true
			second := channels.NormTime(channelstest.T0.Add(3 * time.Hour))
			require.NoError(t, s.Untombstone(ctx, h, second, resume(second)),
				"the restore writes no live meta.yaml after the move")
			ffs.failLive = false
			head, err := s.Head(ctx, k)
			require.NoError(t, err)
			require.Equal(t, map[string]time.Time{"stale": second}, head.Session.Restarts)
			require.Equal(t, second.Add(time.Hour), head.Session.NextDueAt)
		})
	}
}
