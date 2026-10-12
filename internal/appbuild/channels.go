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
		return filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
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
// A whole-entity delete leaves the channels in place: whether they are
// deleted, kept for retention or detached is the delete policy's call, not
// this hook's. The create hook is what makes keeping them safe.
type channelAliases struct {
	store channels.Store
	// families reads the stored faces of an id, to tell a new entity from a
	// new face of an existing one.
	families store.EntityLister
	now      func() time.Time
}

// channelAliasesFor returns the alias subscriber for cs.
//
// Nil: returns a nil *channelAliases when cs is nil, which newAliasFanout
// drops.
func channelAliasesFor(cs channels.Store, st store.EntityLister) *channelAliases {
	if cs == nil {
		return nil
	}
	return &channelAliases{store: cs, families: st, now: time.Now}
}

// EntityRenamed moves the channels of oldID to newID. Channels already on
// newID belong to an earlier entity with that id, so they are tombstoned
// first. A sweep then tombstones any channel an append created under oldID
// while the rename ran.
func (a *channelAliases) EntityRenamed(ctx context.Context, oldID, newID string) error {
	at := a.now()
	if err := a.store.Tombstone(ctx, newID, "", at); err != nil {
		return fmt.Errorf("channels: clear %s before rename: %w", newID, err)
	}
	err := a.store.Rekey(ctx, oldID, newID, at)
	if errors.Is(err, channels.ErrRekeyConflict) {
		// An append landed on newID between the tombstone and the rekey.
		if err = a.store.Tombstone(ctx, newID, "", at); err == nil {
			err = a.store.Rekey(ctx, oldID, newID, at)
		}
	}
	if err != nil {
		return fmt.Errorf("channels: rename %s to %s: %w", oldID, newID, err)
	}
	if err := a.sweep(ctx, oldID, at); err != nil {
		return fmt.Errorf("channels: sweep %s after rename: %w", oldID, err)
	}
	return nil
}

// sweep tombstones channels created under oldID while the rename ran. They
// cannot be moved: newID now holds the renamed entity's channels, so a
// second Rekey would conflict.
func (a *channelAliases) sweep(ctx context.Context, oldID string, at time.Time) error {
	left, err := a.store.ListEntity(ctx, oldID)
	if err != nil || len(left) == 0 {
		return err
	}
	return a.store.Tombstone(ctx, oldID, "", at)
}

// EntityDeleted does nothing; see the type doc.
func (a *channelAliases) EntityDeleted(context.Context, string) error { return nil }

// EntityFaceDeleted detaches the channels anchored to the deleted face.
// Channels anchored to the whole entity stay.
func (a *channelAliases) EntityFaceDeleted(ctx context.Context, id string, face entity.Face) error {
	if face == "" {
		return nil
	}
	if err := a.store.Tombstone(ctx, id, face, a.now()); err != nil {
		return fmt.Errorf("channels: detach %s: %w", entity.FormatStateRef(id, face), err)
	}
	return nil
}

// EntityCreated detaches channels a new entity would otherwise inherit. When
// the created face is the only stored face, the entity is new and every
// channel under the id is left over from a deleted one. When other faces
// exist, only channels anchored to the created face can be left over, from
// a deleted face of that name.
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
	switch {
	case len(headers) <= 1:
		err = a.store.Tombstone(ctx, id, "", a.now())
	case face != "":
		err = a.store.Tombstone(ctx, id, face, a.now())
	}
	if err != nil {
		return fmt.Errorf("channels: clear %s on create: %w", entity.FormatStateRef(id, face), err)
	}
	return nil
}
