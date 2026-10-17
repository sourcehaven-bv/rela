package appbuild

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// channelGuard is the [entitymanager.AnchorGuard] over the channel store. It
// reads each channel type's `on_entity_delete` from the realtime config the
// holder serves:
//
//   - block refuses the delete with [*entitymanager.AnchorBlockedError]
//     before anything is written;
//   - detach tombstones the channels, and a failed delete restores them;
//   - delete is treated as detach until deleting channels with their entity
//     lands (T4), and boot warns about it;
//   - a channel whose type the config does not declare, or a project without
//     realtime.yaml, is detached: the config can no longer say otherwise.
//
// In degraded mode the holder serves the last good config, so the rules that
// held when the channels were written still apply. One delete operation reads
// the config once, in Scope, and judges every entity it removes under it.
//
// A block is best effort against a concurrent first post: a channel created
// after the guard listed the entity's channels is swept up by the post-commit
// alias hook as a detach.
//
// The guard sees only deletes made through the manager. The channels of an
// entity deleted outside it (a file removed by hand or by git, or a write
// to the database that bypasses rela) stay live, and nothing detaches them.
// They are unreachable all the same: every channel request reads the anchor
// entity first, and a missing anchor gets the uniform 404. A create at the
// id through any process's manager tombstones them, through the
// EntityCreated alias hook on the shared channel store (see
// [channelAliases.EntityCreated]), so a new entity never inherits them. An
// entity that comes back outside the manager, as a git checkout or stash
// puts a file back, gets them back unchanged.
type channelGuard struct {
	store  channels.Store
	config func() *realtimecfg.Compiled
	now    func() time.Time
}

var _ entitymanager.AnchorGuard = (*channelGuard)(nil)

// newChannelGuard returns the guard over cs, reading the realtime config from
// rt. A nil cs guards nothing.
//
// Nil: cs is accepted (no channel store); rt is accepted (realtime off).
func newChannelGuard(cs channels.Store, rt *RealtimeConfig) entitymanager.AnchorGuard {
	if cs == nil {
		return entitymanager.NopAnchorGuard{}
	}
	return &channelGuard{
		store:  cs,
		config: func() *realtimecfg.Compiled { return rt.Load().Config },
		now:    time.Now,
	}
}

// channelGuardFor checks the realtime config against the channel types the
// store has recorded (see checkChannelTypeRecords) and returns the guard
// over cs. A mismatch fails the assembly, so the server does not start.
func channelGuardFor(base *SharedBase, cs channels.Store) (entitymanager.AnchorGuard, error) {
	if err := checkChannelTypeRecords(context.Background(), cs, base.realtime.Load().Config); err != nil {
		return nil, err
	}
	return newChannelGuard(cs, base.realtime), nil
}

// Active implements [entitymanager.AnchorGuard]: a guard over a channel
// store can always act.
func (g *channelGuard) Active() bool { return true }

// Scope implements [entitymanager.AnchorGuard]. It lists the channels of
// each id once and loads the config once; the scope's calls reuse both. The
// error does not name an id, since ids may include entities the caller did
// not address.
func (g *channelGuard) Scope(ctx context.Context, ids []string) (entitymanager.AnchorScope, error) {
	lists := make(map[string][]channels.Channel, len(ids))
	for _, id := range ids {
		list, err := g.store.ListEntity(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("channels: list channels before delete: %w", err)
		}
		if len(list) > 0 {
			lists[id] = list
		}
	}
	if len(lists) == 0 {
		return nil, nil //nolint:nilnil // no channels: the interface's "nothing to guard"
	}
	return &channelScope{guard: g, cfg: g.config(), lists: lists}, nil
}

// channelScope is one delete operation's [entitymanager.AnchorScope]: the
// config it is judged under and the channels Scope listed.
type channelScope struct {
	guard *channelGuard
	cfg   *realtimecfg.Compiled
	lists map[string][]channels.Channel
}

// BeforeDelete implements [entitymanager.AnchorScope].
func (s *channelScope) BeforeDelete(
	ctx context.Context, d entitymanager.AnchorDelete,
) (entitymanager.AnchorUndo, error) {
	list := s.lists[d.ID]
	if len(list) == 0 {
		return nil, nil //nolint:nilnil // no channels: nothing to undo
	}
	if blocked := blocking(s.cfg, list, d); blocked != "" {
		if !d.Purge {
			return nil, &entitymanager.AnchorBlockedError{ChannelType: blocked}
		}
		slog.Warn("channels: purging a soft-deleted entity whose channel type blocks deletes; "+
			"detaching its channels", "id", d.ID, "channel_type", blocked)
	}
	face := d.Face
	if d.Detach {
		face = ""
	} else if !hasFaceChannel(list, face) {
		return nil, nil //nolint:nilnil // the face has no channels: nothing to undo
	}
	g := s.guard
	h, err := g.tombstone(ctx, d.ID, face)
	if err != nil {
		return nil, fmt.Errorf("channels: detach %s before delete: %w", entity.FormatStateRef(d.ID, face), err)
	}
	if h.IsZero() {
		return nil, nil //nolint:nilnil // nothing was tombstoned: nothing to undo
	}
	return tombstoneUndo{store: g.store, h: h, now: g.now}, nil
}

// CheckDelete implements [entitymanager.AnchorScope].
func (s *channelScope) CheckDelete(_ context.Context, d entitymanager.AnchorDelete) error {
	if blocked := blocking(s.cfg, s.lists[d.ID], d); blocked != "" {
		return &entitymanager.AnchorBlockedError{ChannelType: blocked}
	}
	return nil
}

// tombstone tombstones id@face. A store can fail after moving part of the
// batch and still return its handle; that part is put back before the error
// is returned, so a failed detach leaves nothing detached.
func (g *channelGuard) tombstone(ctx context.Context, id string, face entity.Face) (channels.TombstoneHandle, error) {
	h, err := g.store.Tombstone(ctx, id, face, g.now())
	if err == nil || h.IsZero() {
		return h, err
	}
	uctx, cancel := undoContext(ctx)
	defer cancel()
	if uErr := g.store.Untombstone(uctx, h, g.now()); uErr != nil {
		slog.Error("channels: cannot restore a detach that failed part way",
			"tombstone", h.String(), "error", uErr)
	}
	return channels.TombstoneHandle{}, err
}

// BeforeRename implements [entitymanager.AnchorGuard]. Channels on newID
// belong to an earlier entity with that id; once free confirms the id can be
// taken, they are detached, so the renamed entity does not inherit them. The
// undo puts them back when the rename then fails.
func (g *channelGuard) BeforeRename(
	ctx context.Context, newID string, free func(context.Context) error,
) (entitymanager.AnchorUndo, error) {
	list, err := g.store.ListEntity(ctx, newID)
	if err != nil {
		return nil, fmt.Errorf("channels: list %s before rename: %w", newID, err)
	}
	if len(list) == 0 {
		return nil, nil //nolint:nilnil // no channels: nothing to undo
	}
	if freeErr := free(ctx); freeErr != nil {
		return nil, freeErr
	}
	h, err := g.tombstone(ctx, newID, "")
	if err != nil {
		return nil, fmt.Errorf("channels: clear %s before rename: %w", newID, err)
	}
	if h.IsZero() {
		return nil, nil //nolint:nilnil // nothing was tombstoned: nothing to undo
	}
	return tombstoneUndo{store: g.store, h: h, now: g.now}, nil
}

// tombstoneUndo is the [entitymanager.AnchorUndo] of one tombstone batch.
type tombstoneUndo struct {
	store channels.Store
	h     channels.TombstoneHandle
	now   func() time.Time
}

// Restore implements [entitymanager.AnchorUndo].
func (u tombstoneUndo) Restore(ctx context.Context) error {
	return u.store.Untombstone(ctx, u.h, u.now())
}

// String implements [entitymanager.AnchorUndo]: the handle's text form,
// which [channels.ParseTombstoneHandle] reads back.
func (u tombstoneUndo) String() string { return u.h.String() }

// channelUndoWait bounds the restore of a failed detach.
const channelUndoWait = 30 * time.Second

// undoContext is the context a restore runs on. The restore repairs a write
// that already failed, so the request's cancellation must not reach it.
func undoContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), channelUndoWait)
}

// blocking returns the type of the first channel in list that d removes and
// whose type blocks deletes under cfg, or "" when none does. A whole-entity
// delete removes every channel. A face delete removes the face's own
// channels, and counts the entity's family channels when d.Block says the
// caller's last face goes.
//
// Nil: cfg is accepted and means realtime is off, so nothing blocks.
func blocking(cfg *realtimecfg.Compiled, list []channels.Channel, d entitymanager.AnchorDelete) string {
	if cfg == nil {
		return ""
	}
	for _, c := range list {
		counts := d.Face == "" || c.Key.Face == d.Face || (c.Key.Face == "" && d.Block)
		if !counts {
			continue
		}
		if ct, ok := cfg.ChannelType(c.Key.Type); ok && ct.OnEntityDelete == realtimecfg.OnDeleteBlock {
			return c.Key.Type
		}
	}
	return ""
}

// hasFaceChannel reports whether list holds a channel anchored to face.
func hasFaceChannel(list []channels.Channel, face entity.Face) bool {
	for _, c := range list {
		if c.Key.Face == face {
			return true
		}
	}
	return false
}

// checkChannelTypeRecords refuses a realtime config that changes the anchor
// or cardinality of a channel type the store has already written channels
// for: they are keyed by both, so serving them under the new values would
// strand them. Only recorded types are compared, and nothing is written; the
// channel service records a type on its first channel write.
//
// It also warns about each channel type whose `on_entity_delete: delete` is
// still treated as detach.
//
// Nil: cfg is accepted and means realtime is off.
func checkChannelTypeRecords(ctx context.Context, cs channels.Store, cfg *realtimecfg.Compiled) error {
	if cfg == nil || cs == nil {
		return nil
	}
	for _, ct := range cfg.ChannelTypes() {
		if ct.OnEntityDelete == realtimecfg.OnDeleteDelete {
			slog.Warn("realtime: on_entity_delete: delete detaches channels for now; "+
				"deleting them with their entity is not supported yet", "channel_type", ct.Name)
		}
	}
	recorded, err := cs.TypeRecords(ctx)
	if err != nil {
		return fmt.Errorf("channels: read the recorded channel types: %w", err)
	}
	changes := channels.TypeRecordChanges(recorded, configuredTypeRecords(cfg))
	if len(changes) == 0 {
		return nil
	}
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		parts = append(parts, fmt.Sprintf("%s: anchor %s and cardinality %s are recorded, %s and %s configured",
			c.Type, c.Recorded.Anchor, c.Recorded.Cardinality, c.Configured.Anchor, c.Configured.Cardinality))
	}
	return fmt.Errorf("%s: stored channels cannot change anchor or cardinality (%s); "+
		"restore the recorded values", realtimecfg.FileName, strings.Join(parts, "; "))
}

// configuredTypeRecords returns the type record each channel type of cfg
// would write.
func configuredTypeRecords(cfg *realtimecfg.Compiled) map[string]channels.TypeRecord {
	out := make(map[string]channels.TypeRecord)
	for _, ct := range cfg.ChannelTypes() {
		out[ct.Name] = channels.TypeRecord{
			Anchor:      channels.AnchorMode(ct.Anchor),
			Cardinality: channels.Cardinality(ct.Cardinality),
		}
	}
	return out
}
