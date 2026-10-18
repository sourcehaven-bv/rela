package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// channelsDir and channelsLockFile are the file backend's directory and its
// cross-process lock, in the project's cache directory.
const (
	channelsDir      = "channels"
	channelsLockFile = "channels.lock"
)

// buildChannels returns the channel store for st. In order: the store a
// recipe supplied (sqlite, from rela.db), the store's own database backend
// (postgres, see storeChannelsFor), the file backend in the cache
// directory, and an in-memory file backend when the project has none.
func buildChannels(
	fs storage.FS, paths *project.Context, st store.Store, override channels.Store,
) (channels.Store, error) {
	if override != nil {
		return override, nil
	}
	if s := storeChannelsFor(st); s != nil {
		return s, nil
	}
	if fs == nil || paths == nil || paths.CacheDir == "" {
		slog.Warn("channels: no cache directory; channels are kept in this process only")
		return memoryChannels(), nil
	}
	var locker filechannels.Locker = filelock.ProcessPrivate{}
	if _, mem := fs.(*storage.MemFS); !mem {
		l, err := filelock.New(filepath.Join(paths.CacheDir, channelsLockFile))
		if err != nil {
			return nil, fmt.Errorf("channels: %w", err)
		}
		locker = l
	}
	return filechannels.New(fs, filepath.Join(paths.CacheDir, channelsDir), locker)
}

// memoryChannels returns a file backend over an in-memory filesystem: the
// channels live as long as the process.
func memoryChannels() channels.Store {
	s, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	if err != nil { // coverage-ignore: unreachable: both collaborators are non-nil
		panic(err)
	}
	return s
}

// Channels returns the channel store of s. It is a package function rather
// than a method because [Services] is at its plimsoll cap.
//
// Nil: returned only for a Services built by [NewFromCollaborators].
func Channels(s *Services) channels.Store { return s.channels }

// channelAliases keeps channels attached to their entity across a rename,
// and keeps channels left behind by a deleted entity or face away from a
// new entity that reuses the id. It is an alias subscriber (see
// newAliasFanout) and a [entitymanager.CreateNotifier].
//
// It is the second line of defense. The first is channelGuard, which the
// manager calls before the write: it applies the delete policy and clears
// newID before a rename. These hooks run after the commit and sweep up
// channels a concurrent first post created while the write ran.
//
// Every batch these hooks tombstone is filed confirmed, with the purge time
// the realtime config gives it (see [realtime.Filer]): the write that
// caused it has committed. The delete hooks also confirm the guard's pending
// batches and wake the realtime worker.
type channelAliases struct {
	store channels.Store
	// families reads the stored faces of an id, to tell a new entity from a
	// new face of an existing one.
	families store.EntityLister
	tombs    *tombstoneConfirmer
	rt       *RealtimeConfig
	now      func() time.Time
}

// channelAliasesFor returns the alias subscriber for cs.
//
// Nil: returns a nil *channelAliases when cs is nil, which newAliasFanout
// drops. st and tombs are rejected with cs; rt is accepted (realtime off).
func channelAliasesFor(
	cs channels.Store, st store.EntityLister, tombs *tombstoneConfirmer, rt *RealtimeConfig,
) (*channelAliases, error) {
	if cs == nil {
		return nil, nil //nolint:nilnil // no channel store: newAliasFanout drops a nil subscriber
	}
	if st == nil || tombs == nil {
		return nil, errors.New("channels: the alias hooks need an entity lister and a tombstone confirmer")
	}
	return &channelAliases{store: cs, families: st, tombs: tombs, rt: rt, now: time.Now}, nil
}

// tombstone files the channels of id@face as one confirmed batch for
// policy (see [realtime.Filer]), or does nothing when there are none.
func (a *channelAliases) tombstone(
	ctx context.Context, id string, face entity.Face, at time.Time, policy channels.TombstonePolicy,
) error {
	_, err := a.store.Tombstone(ctx, id, face, at, realtime.Filer(a.rt.Load().Config, policy, at, false))
	return err
}

// EntityRenamed moves the channels of oldID to newID. The guard already
// detached what an earlier entity left on newID, before the rename. A sweep
// then tombstones any channel an append created under oldID while the
// rename ran.
//
// A channel on newID at this point was created by a post to the renamed
// entity between the commit and this hook. The rekey then conflicts, and
// that channel is detached so the renamed entity's own channels can move;
// the poster sees the message vanish. Closing the window needs the rename
// and the rekey in one transaction, which the store seam does not offer.
// If the rekey still fails, the channels stay under oldID, where
// EntityCreated detaches them before a new entity at oldID could read them.
func (a *channelAliases) EntityRenamed(ctx context.Context, oldID, newID string) error {
	// The rename committed: the batch the guard filed on newID is final.
	if err := a.tombs.confirm(ctx, newID, "", true); err != nil {
		return err
	}
	at := a.now()
	err := a.store.Rekey(ctx, oldID, newID, at)
	if errors.Is(err, channels.ErrRekeyConflict) {
		if err = a.tombstone(ctx, newID, "", at, channels.PolicyDetach); err == nil {
			err = a.store.Rekey(ctx, oldID, newID, at)
		}
	}
	if err != nil {
		return fmt.Errorf("channels: rename %s to %s: %w", oldID, newID, err)
	}
	// Channels left under oldID were created while the rename ran, and
	// cannot be moved: newID now holds the renamed entity's channels, so a
	// second Rekey would conflict.
	if err := a.tombstone(ctx, oldID, "", at, channels.PolicyDetach); err != nil {
		return fmt.Errorf("channels: sweep %s after rename: %w", oldID, err)
	}
	return nil
}

// EntityDeleted confirms the batches the guard filed for id, then detaches
// channels a concurrent first post created after the guard ran, under the
// same delete policy.
func (a *channelAliases) EntityDeleted(ctx context.Context, id string) error {
	confirmErr := a.tombs.confirm(ctx, id, "", true)
	if err := a.tombstone(ctx, id, "", a.now(), channels.PolicyDelete); err != nil {
		return errors.Join(confirmErr, fmt.Errorf("channels: sweep %s after delete: %w", id, err))
	}
	a.tombs.ping()
	return confirmErr
}

// EntityFaceDeleted confirms the batches the guard filed for the face, then
// detaches the channels still anchored to it. Channels anchored to the
// whole entity stay.
func (a *channelAliases) EntityFaceDeleted(ctx context.Context, id string, face entity.Face) error {
	if face == "" {
		return nil
	}
	confirmErr := a.tombs.confirm(ctx, id, face, false)
	if err := a.tombstone(ctx, id, face, a.now(), channels.PolicyDelete); err != nil {
		return errors.Join(confirmErr, fmt.Errorf("channels: detach %s: %w", entity.FormatStateRef(id, face), err))
	}
	a.tombs.ping()
	return confirmErr
}

// EntityCreated detaches channels a new entity would otherwise inherit. When
// the created face is the only stored face, the entity is new and every
// channel under the id is left over from a deleted one. When other faces
// exist, only channels anchored to the created face can be left over, from
// a deleted face of that name.
//
// It also makes final every batch already filed for what it clears: a
// pending batch of an earlier delete, or an orphan batch, is never put back
// onto the new entity (see [channels.Store.ConfirmTombstones]). A store
// refuses to restore a final batch, so a sweep that judged the batch before
// this hook ran cannot put it back either.
//
// A message appended to the new entity between the create's commit and this
// hook is detached with the leftovers. The window is the hook's own run
// time; closing it needs the create and the tombstone in one transaction,
// which the store seam does not offer.
func (a *channelAliases) EntityCreated(ctx context.Context, id string, face entity.Face) error {
	headers, err := store.FamilyHeaders(ctx, a.families, id)
	if err != nil {
		return fmt.Errorf("channels: read faces of %s: %w", id, err)
	}
	var target entity.Face
	switch {
	case len(headers) <= 1:
		target = ""
	case face != "":
		target = face
	default:
		return nil
	}
	err = a.store.ConfirmTombstones(ctx, channels.Anchor{ID: id, Face: target})
	if err == nil {
		err = a.tombstone(ctx, id, target, a.now(), channels.PolicyDetach)
	}
	if err != nil {
		return fmt.Errorf("channels: clear %s on create: %w", entity.FormatStateRef(id, face), err)
	}
	return nil
}
