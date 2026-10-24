package appbuild

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

var chanAuthor = channels.Author{User: "alice"}

func chanKey(id string, face entity.Face, typ string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id, Face: face}, Type: typ}
}

func newChannelAliases(t *testing.T) (*channelAliases, channels.Store, *memstore.MemStore) {
	t.Helper()
	cs, err := filechannels.New(storage.NewMemFS(), "/p/.rela/channels", filelock.ProcessPrivate{})
	require.NoError(t, err)
	st := memstore.New()
	tombs, err := newTombstoneConfirmer(cs)
	require.NoError(t, err)
	a, err := channelAliasesFor(cs, st, tombs, nil)
	require.NoError(t, err)
	a.now = func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) }
	return a, cs, st
}

func appendTo(t *testing.T, cs channels.Store, k channels.Key) {
	t.Helper()
	appendCtx(context.Background(), t, cs, k)
}

func appendCtx(ctx context.Context, t *testing.T, cs channels.Store, k channels.Key) {
	t.Helper()
	_, err := cs.Append(ctx, k, channels.NewMessage{
		Type: "text", Author: chanAuthor, At: time.Now(), Props: map[string]any{"body": k.String()},
	})
	require.NoError(t, err)
}

func entityKeys(t *testing.T, cs channels.Store, id string) []string {
	t.Helper()
	list, err := cs.ListEntity(context.Background(), id)
	require.NoError(t, err)
	out := []string{}
	for _, c := range list {
		out = append(out, c.Key.String())
	}
	return out
}

// A rename moves the channels; channels already on the new id belong to an
// earlier entity and are detached rather than merged.
func TestChannelAliases_Rename(t *testing.T) {
	ctx := context.Background()
	a, cs, _ := newChannelAliases(t)
	appendTo(t, cs, chanKey("OLD-1", "", "room"))
	appendTo(t, cs, chanKey("OLD-1", "draft", "room"))
	appendTo(t, cs, chanKey("NEW-1", "", "stale"))

	require.NoError(t, a.EntityRenamed(ctx, "OLD-1", "NEW-1"))
	require.Empty(t, entityKeys(t, cs, "OLD-1"))
	require.Equal(t, []string{"NEW-1/room", "NEW-1@draft/room"}, entityKeys(t, cs, "NEW-1"))

	// A rename of an id with no channels does nothing.
	require.NoError(t, a.EntityRenamed(ctx, "NONE-1", "NONE-2"))
}

// rekeyRacer stands in for appends racing a rename: the first conflicts
// Rekey calls fail as if an append landed on the target, and a straggler
// is appended under the old id after the move succeeds.
type rekeyRacer struct {
	channels.Store
	t         *testing.T
	conflicts int
	straggle  bool
	calls     int
	tombs     []string
}

func (r *rekeyRacer) Rekey(ctx context.Context, oldID, newID string, at time.Time) error {
	r.calls++
	if r.calls <= r.conflicts {
		return channels.ErrRekeyConflict
	}
	if err := r.Store.Rekey(ctx, oldID, newID, at); err != nil {
		return err
	}
	if r.straggle {
		appendCtx(ctx, r.t, r.Store, chanKey(oldID, "", "late"))
	}
	return nil
}

func (r *rekeyRacer) Tombstone(
	ctx context.Context, id string, face entity.Face, at time.Time, f channels.Filer,
) (channels.TombstoneHandle, error) {
	h, err := r.Store.Tombstone(ctx, id, face, at, f)
	if !h.IsZero() {
		r.tombs = append(r.tombs, entity.FormatStateRef(id, face))
	}
	return h, err
}

func TestChannelAliases_RenameRaces(t *testing.T) {
	ctx := context.Background()
	a, cs, _ := newChannelAliases(t)
	appendTo(t, cs, chanKey("OLD-1", "", "room"))
	appendTo(t, cs, chanKey("NEW-1", "", "stale"))
	racer := &rekeyRacer{Store: cs, t: t, conflicts: 1}
	a.store = racer
	require.NoError(t, a.EntityRenamed(ctx, "OLD-1", "NEW-1"))
	require.Equal(t, []string{"NEW-1"}, racer.tombs, "the target is cleared before the retry")
	require.Equal(t, []string{"NEW-1/room"}, entityKeys(t, cs, "NEW-1"))

	// A channel created under the old id during the rename is detached.
	appendTo(t, cs, chanKey("OLD-2", "", "room"))
	racer = &rekeyRacer{Store: cs, t: t, straggle: true}
	a.store = racer
	require.NoError(t, a.EntityRenamed(ctx, "OLD-2", "NEW-2"))
	require.Equal(t, []string{"OLD-2"}, racer.tombs, "only the straggler is swept")
	require.Empty(t, entityKeys(t, cs, "OLD-2"))
	require.Equal(t, []string{"NEW-2/room"}, entityKeys(t, cs, "NEW-2"))

	// A failing store surfaces the error to the hook.
	a.store = failingChannels{Store: cs}
	require.Error(t, a.EntityRenamed(ctx, "A-1", "B-1"))
	appendTo(t, cs, chanKey("A-1", "draft", "room"))
	require.Error(t, a.EntityFaceDeleted(ctx, "A-1", "draft"))
}

type failingChannels struct{ channels.Store }

func (failingChannels) Tombstone(
	context.Context, string, entity.Face, time.Time, channels.Filer,
) (channels.TombstoneHandle, error) {
	return channels.TombstoneHandle{}, errDiskFull
}

func (failingChannels) Rekey(context.Context, string, string, time.Time) error { return errDiskFull }

func (failingChannels) Untombstone(context.Context, channels.TombstoneHandle, time.Time, channels.ScheduleFunc) error {
	return errDiskFull
}

var errDiskFull = errors.New("disk full")

// A face delete detaches only that face's channels. A whole-entity delete
// sweeps what the guard left, which is a channel a concurrent first post
// created after the guard ran.
func TestChannelAliases_Deletes(t *testing.T) {
	ctx := context.Background()
	a, cs, _ := newChannelAliases(t)
	appendTo(t, cs, chanKey("T-1", "", "room"))
	appendTo(t, cs, chanKey("T-1", "draft", "room"))
	require.NoError(t, a.EntityFaceDeleted(ctx, "T-1", "draft"))
	require.NoError(t, a.EntityFaceDeleted(ctx, "T-1", ""))
	require.Equal(t, []string{"T-1/room"}, entityKeys(t, cs, "T-1"))
	require.NoError(t, a.EntityDeleted(ctx, "T-1"))
	require.Empty(t, entityKeys(t, cs, "T-1"))
	require.NoError(t, a.EntityDeleted(ctx, "T-1"), "nothing left to sweep")

	a.store = failingChannels{Store: cs}
	appendTo(t, cs, chanKey("T-2", "", "room"))
	require.Error(t, a.EntityDeleted(ctx, "T-2"))
}

// A new entity on a reused id starts without channels. A new face of an
// existing entity loses only the leftovers of a deleted face of that name.
func TestChannelAliases_Create(t *testing.T) {
	ctx := context.Background()
	a, cs, st := newChannelAliases(t)
	appendTo(t, cs, chanKey("T-1", "", "room"))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "task"}))
	require.NoError(t, a.EntityCreated(ctx, "T-1", ""))
	require.Empty(t, entityKeys(t, cs, "T-1"), "leftovers of a deleted entity are detached")

	appendTo(t, cs, chanKey("T-1", "", "room"))
	appendTo(t, cs, chanKey("T-1", "draft", "room"))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "task", Face: "draft"}))
	require.NoError(t, a.EntityCreated(ctx, "T-1", "draft"))
	require.Equal(t, []string{"T-1/room"}, entityKeys(t, cs, "T-1"))

	a.store = failingChannels{Store: cs}
	appendTo(t, cs, chanKey("T-9", "", "room"))
	require.Error(t, a.EntityCreated(ctx, "T-9", ""))
}

// The fanout forwards a create only to subscribers that want it.
func TestAliasFanout_EntityCreated(t *testing.T) {
	ctx := context.Background()
	a, cs, st := newChannelAliases(t)
	appendTo(t, cs, chanKey("T-1", "", "room"))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "task"}))
	fan := newAliasFanout(a, nopRewriter{})
	n, ok := fan.(entitymanager.CreateNotifier)
	require.True(t, ok)
	require.NoError(t, n.EntityCreated(ctx, "T-1", ""))
	require.Empty(t, entityKeys(t, cs, "T-1"))

	a.store = failingChannels{Store: cs}
	appendTo(t, cs, chanKey("T-1", "", "room"))
	require.Error(t, n.EntityCreated(ctx, "T-1", ""))
	none, err := channelAliasesFor(nil, st, nil, nil)
	require.NoError(t, err)
	require.Nil(t, none)
}

type nopRewriter struct{}

func (nopRewriter) EntityRenamed(context.Context, string, string) error          { return nil }
func (nopRewriter) EntityDeleted(context.Context, string) error                  { return nil }
func (nopRewriter) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }

func TestBuildChannels(t *testing.T) {
	st := memstore.New()
	override, err := filechannels.New(storage.NewMemFS(), "/x", filelock.ProcessPrivate{})
	require.NoError(t, err)
	got, err := buildChannels(storage.NewMemFS(), nil, st, override)
	require.NoError(t, err)
	require.Same(t, override, got)

	got, err = buildChannels(nil, nil, st, nil)
	require.NoError(t, err)
	require.NotNil(t, got, "no cache directory still yields an in-process store")

	paths := &project.Context{CacheDir: "/p/.rela"}
	got, err = buildChannels(storage.NewMemFS(), paths, st, nil)
	require.NoError(t, err)
	require.NotNil(t, got)

	if storeChannelsFor(st) != nil {
		return // the postgres build never reaches the cache directory
	}
	disk := &project.Context{CacheDir: filepath.Join(t.TempDir(), ".rela")}
	got, err = buildChannels(storage.NewSafeFS(storage.NewOsFS()), disk, st, nil)
	require.NoError(t, err)
	appendTo(t, got, chanKey("T-1", "", "room"))
	require.FileExists(t, filepath.Join(disk.CacheDir, channelsLockFile))
	require.DirExists(t, filepath.Join(disk.CacheDir, channelsDir))
}
