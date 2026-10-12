package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
// held when the channels were written still apply.
//
// A block is best effort against a concurrent first post: a channel created
// after the guard listed the entity's channels is swept up by the post-commit
// alias hook as a detach.
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

// BeforeDelete implements [entitymanager.AnchorGuard].
func (g *channelGuard) BeforeDelete(
	ctx context.Context, d entitymanager.AnchorDelete,
) (func(context.Context) error, error) {
	list, err := g.store.ListEntity(ctx, d.ID)
	if err != nil {
		return nil, fmt.Errorf("channels: list %s before delete: %w", d.ID, err)
	}
	if len(list) == 0 {
		return nil, nil //nolint:nilnil // no channels: nothing to undo
	}
	if blocked := g.blocking(list, d); blocked != "" {
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
	h, err := g.store.Tombstone(ctx, d.ID, face, g.now())
	if err != nil {
		return nil, fmt.Errorf("channels: detach %s before delete: %w", entity.FormatStateRef(d.ID, face), err)
	}
	if h.IsZero() {
		return nil, nil //nolint:nilnil // nothing was tombstoned: nothing to undo
	}
	return func(ctx context.Context) error {
		return g.store.Untombstone(ctx, h, g.now())
	}, nil
}

// CheckDelete implements [entitymanager.AnchorGuard].
func (g *channelGuard) CheckDelete(ctx context.Context, d entitymanager.AnchorDelete) error {
	list, err := g.store.ListEntity(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("channels: list %s before delete: %w", d.ID, err)
	}
	if blocked := g.blocking(list, d); blocked != "" {
		return &entitymanager.AnchorBlockedError{ChannelType: blocked}
	}
	return nil
}

// BeforeRename implements [entitymanager.AnchorGuard]. Channels on newID
// belong to an earlier entity with that id; once free confirms the id can be
// taken, they are detached, so the renamed entity does not inherit them.
// Nothing is undone when the rename then fails: those channels had no entity
// to serve them anyway.
func (g *channelGuard) BeforeRename(ctx context.Context, newID string, free func(context.Context) error) error {
	list, err := g.store.ListEntity(ctx, newID)
	if err != nil {
		return fmt.Errorf("channels: list %s before rename: %w", newID, err)
	}
	if len(list) == 0 {
		return nil
	}
	if err := free(ctx); err != nil {
		return err
	}
	if _, err := g.store.Tombstone(ctx, newID, "", g.now()); err != nil {
		return fmt.Errorf("channels: clear %s before rename: %w", newID, err)
	}
	return nil
}

// blocking returns the type of the first channel in list that d removes and
// whose type blocks deletes, or "" when none does. A whole-entity delete
// removes every channel. A face delete removes the face's own channels, and
// counts the entity's family channels when d.Block says the caller's last
// face goes.
func (g *channelGuard) blocking(list []channels.Channel, d entitymanager.AnchorDelete) string {
	cfg := g.config()
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

// channelReconcileBuffer is the event buffer of the reconciler. A burst of
// external deletes larger than this drops events; the channels of a dropped
// delete stay where they are until EntityCreated clears them for a new
// entity with the same id.
const channelReconcileBuffer = 256

// startChannelReconciler detaches the channels of entities deleted outside
// the manager, which on the file backend means a file removed by hand or by
// git. The store reports such a removal as an entity-deleted event, as it
// reports the manager's own deletes. The reconciler acts only when the event
// still holds once it runs: the face or family is gone and not soft-deleted
// (a soft-deleted entity keeps its channels until it is purged). For a delete
// the manager made, the guard has already detached everything, so the
// reconciler finds nothing or a concurrent first post, which the alias hook
// would sweep too.
//
// Events are handled on a goroutine of their own, never on the store's
// emitting path. The returned stop cancels the subscription and waits for
// the goroutine.
//
// Nil: cs is accepted and starts nothing.
func startChannelReconciler(st store.Store, cs channels.Store) func() {
	if cs == nil {
		return func() {}
	}
	events, cancel := st.Subscribe(channelReconcileBuffer)
	r := channelReconciler{store: st, channels: cs, now: time.Now}
	var wg sync.WaitGroup
	wg.Go(func() {
		for ev := range events {
			if ev.Op == store.EventEntityDeleted {
				r.handle(context.Background(), ev.EntityID, ev.Face)
			}
		}
	})
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			wg.Wait()
		})
	}
}

// channelReconciler is the state of startChannelReconciler's goroutine.
type channelReconciler struct {
	store    store.Store
	channels channels.Store
	now      func() time.Time
}

// handle detaches the channels of id, or of id@face when other faces of the
// family survive. Failures are logged; the next create at the id clears
// whatever is left.
func (r channelReconciler) handle(ctx context.Context, id string, face entity.Face) {
	if err := r.reconcile(ctx, id, face); err != nil {
		slog.Error("channels: detach the channels of an entity deleted outside rela",
			"id", id, "face", face, "error", err)
	}
}

func (r channelReconciler) reconcile(ctx context.Context, id string, face entity.Face) error {
	headers, err := store.FamilyHeaders(ctx, r.store, id)
	if err != nil {
		return err
	}
	target := entity.Face("")
	if len(headers) > 0 {
		if face == "" {
			return nil // the family is back, or only another face went
		}
		for _, h := range headers {
			if h.Face == face {
				return nil // the face is back
			}
		}
		target = face
	} else if marked, mErr := isMarked(ctx, r.store, id); mErr != nil || marked {
		return mErr
	}
	list, err := r.channels.ListEntity(ctx, id)
	if err != nil || len(list) == 0 {
		return err
	}
	if target != "" && !hasFaceChannel(list, target) {
		return nil
	}
	_, err = r.channels.Tombstone(ctx, id, target, r.now())
	return err
}

// isMarked reports whether id is soft-deleted in st.
func isMarked(ctx context.Context, st store.Store, id string) (bool, error) {
	sd, ok := st.(store.SoftDeleteProvider)
	if !ok {
		return false, nil
	}
	marked, err := sd.SoftDelete().ListMarked(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, me := range marked {
		if me.ID == id {
			return true, nil
		}
	}
	return false, nil
}
