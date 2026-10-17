package realtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// anchorChunk is how many ids one [AnchorExistence.Faces] call asks about.
const anchorChunk = 500

// maxRetentionPasses bounds the purge passes of one [Sweeper.RunRetention].
// A pass that purged something is followed by another, so a batch filed
// while the pass ran is not left for the next hour.
const maxRetentionPasses = 8

// DeletePolicy returns the policy of a batch an entity delete through rela
// tombstones: [channels.PolicyDelete] when a channel in chs has a type that
// says `on_entity_delete: delete` under cfg, else [channels.PolicyDetach].
//
// Nil: cfg is accepted and means realtime is off, so nothing is deleted.
func DeletePolicy(cfg *realtimecfg.Compiled, chs []channels.Channel) channels.TombstonePolicy {
	if cfg == nil {
		return channels.PolicyDetach
	}
	for _, c := range chs {
		if ct, ok := cfg.ChannelType(c.Key.Type); ok && ct.OnEntityDelete == realtimecfg.OnDeleteDelete {
			return channels.PolicyDelete
		}
	}
	return channels.PolicyDetach
}

// FilingFor returns the filing of a batch of chs tombstoned at `at` for
// policy, with the purge time [PurgeAfter] computes.
//
// Nil: cfg is accepted (see [PurgeAfter]).
func FilingFor(
	cfg *realtimecfg.Compiled, policy channels.TombstonePolicy, at time.Time, chs []channels.Channel, pending bool,
) channels.Filing {
	return channels.Filing{Policy: policy, PurgeAfter: PurgeAfter(cfg, policy, at, chs), Pending: pending}
}

// PurgeAfter returns the earliest time a batch of chs tombstoned at `at`
// for policy may be erased: the latest of its channels' times. A channel's
// time is
//
//   - at itself for a batch of an entity deleted under
//     `on_entity_delete: delete`, when the channel's type says delete;
//   - otherwise at plus the type's detached retention, or the default
//     [realtimelimits.DetachedRetention] for a type cfg does not declare;
//   - and never before lifecycle.delete.retention has run from the
//     channel's close, or from at when it was never closed.
//
// Nil: cfg is accepted and means realtime is off; every channel then takes
// the default detached retention.
func PurgeAfter(
	cfg *realtimecfg.Compiled, policy channels.TombstonePolicy, at time.Time, chs []channels.Channel,
) time.Time {
	at = channels.NormTime(at)
	types := map[string]*realtimecfg.ChannelType{}
	lookup := func(name string) *realtimecfg.ChannelType {
		if ct, seen := types[name]; seen {
			return ct
		}
		var found *realtimecfg.ChannelType
		if cfg != nil {
			if ct, ok := cfg.ChannelType(name); ok {
				found = &ct
			}
		}
		types[name] = found
		return found
	}
	latest := at
	for _, c := range chs {
		due := channelDue(lookup(c.Key.Type), policy, at, c)
		if due.After(latest) {
			latest = due
		}
	}
	return latest
}

// channelDue is the purge time of one channel; see [PurgeAfter].
//
// Nil: ct is accepted and means the type is not declared.
func channelDue(ct *realtimecfg.ChannelType, policy channels.TombstonePolicy, at time.Time, c channels.Channel) time.Time {
	if ct == nil {
		return realtimecfg.DefaultDetachedRetention().AddTo(at)
	}
	due := ct.DetachedRetention.AddTo(at)
	if policy == channels.PolicyDelete && ct.OnEntityDelete == realtimecfg.OnDeleteDelete {
		due = at
	}
	if op := ct.Lifecycle.Delete; op != nil && !op.Retention.IsZero() {
		from := c.ClosedAt
		if from.IsZero() {
			from = at
		}
		if r := op.Retention.AddTo(from); r.After(due) {
			due = r
		}
	}
	return due
}

// RetentionReport is what one [Sweeper.RunRetention] did.
type RetentionReport struct {
	// Purged counts the batches erased.
	Purged int
	// Confirmed counts the pending batches confirmed because their entity
	// is gone: the delete committed but its confirmation was lost.
	Confirmed int
	// Restored counts the batches put back: a pending batch whose entity
	// still exists after [realtimelimits.OrphanGrace], whose delete
	// therefore failed, and an orphan batch whose entity came back.
	Restored int
	// Detached counts the orphans tombstoned: channels whose entity has
	// been missing for [realtimelimits.OrphanGrace].
	Detached int
	// TypesCleared counts the type records dropped for types the config no
	// longer declares and no channel uses.
	TypesCleared int
}

// RunRetention erases the detached batches that are due, settles the
// pending ones, detaches orphans and clears unused type records.
//
// A batch is erased only once confirmed, and only at its recorded purge
// time. A batch filed without one (by a release before retention) is
// erased only under a present, non-degraded config that declares every
// type in it, with the purge time [PurgeAfter] computes for a detach.
//
// A pending batch was tombstoned before its delete committed. If its
// entity still exists after [realtimelimits.OrphanGrace] the delete failed
// and the batch goes back; if the entity is gone the delete committed and
// the batch is confirmed.
//
// An orphan is a live channel whose entity id has been missing from the
// store for [realtimelimits.OrphanGrace], deleted outside rela. It is
// tombstoned with [channels.PolicyOrphan] and the type's detached
// retention, and put back if the id reappears before it is erased. Only a
// missing id counts; a channel on a missing face of an existing entity is
// left alone. Orphans and type records are handled only under a present,
// non-degraded config.
func (s *Sweeper) RunRetention(ctx context.Context) (RetentionReport, error) {
	var rep RetentionReport
	snap := s.cfg.Load()
	var errs []error
	for range maxRetentionPasses {
		purged, err := s.retentionPass(ctx, snap, &rep)
		if err != nil {
			errs = append(errs, err)
			break
		}
		if purged == 0 {
			break
		}
	}
	if snap.Config != nil && !snap.Degraded {
		errs = append(errs, s.detachOrphans(ctx, snap.Config, &rep), s.clearTypeRecords(ctx, snap.Config, &rep))
	}
	return rep, errors.Join(errs...)
}

// retentionPass handles every batch filed before now once and returns how
// many it purged.
func (s *Sweeper) retentionPass(ctx context.Context, snap Snapshot, rep *RetentionReport) (int, error) {
	now := s.clock()
	batches, err := s.store.Tombstones(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("realtime: list tombstones: %w", err)
	}
	var ids []string
	for _, b := range batches {
		if b.Filing.Pending || b.Filing.Policy == channels.PolicyOrphan {
			ids = append(ids, b.Handle.Anchor().ID)
		}
	}
	faces, err := s.faces(ctx, ids)
	if err != nil {
		return 0, err
	}
	purged := 0
	var errs []error
	for _, b := range batches {
		did, err := s.settleBatch(ctx, snap, b, anchorExists(faces, b.Handle.Anchor()), now, rep)
		if did {
			purged++
		}
		errs = append(errs, err)
	}
	return purged, errors.Join(errs...)
}

// settleBatch restores, confirms or purges one batch, and reports whether
// it purged it.
func (s *Sweeper) settleBatch(
	ctx context.Context, snap Snapshot, b channels.TombstoneBatch, exists bool, now time.Time, rep *RetentionReport,
) (bool, error) {
	h := b.Handle
	if b.Filing.Pending {
		if exists {
			if now.Sub(b.At) < realtimelimits.OrphanGrace {
				return false, nil
			}
			return false, s.restore(ctx, b, rep)
		}
		if err := s.store.ConfirmTombstone(ctx, h); err != nil {
			return false, ignoreGone(err, "confirm", h)
		}
		rep.Confirmed++
	}
	if b.Filing.Policy == channels.PolicyOrphan && exists {
		return false, s.restore(ctx, b, rep)
	}
	due, ok := batchDue(snap, b)
	if !ok || due.After(now) {
		return false, nil
	}
	if err := s.store.Purge(ctx, h, systemAuthor(), now); err != nil {
		return false, ignoreGone(err, "purge", h)
	}
	rep.Purged++
	return true, nil
}

// batchDue returns when b may be purged, and false when the sweep cannot
// tell: an unrecorded batch under a missing or degraded config, or one
// holding a type the config does not declare.
func batchDue(snap Snapshot, b channels.TombstoneBatch) (time.Time, bool) {
	if b.Filing.Recorded() {
		return b.Filing.PurgeAfter, true
	}
	if snap.Config == nil || snap.Degraded {
		return time.Time{}, false
	}
	for _, c := range b.Channels {
		if _, ok := snap.Config.ChannelType(c.Key.Type); !ok {
			return time.Time{}, false
		}
	}
	return PurgeAfter(snap.Config, channels.PolicyDetach, b.At, b.Channels), true
}

// restore puts b back. A batch whose anchor has live channels again stays
// tombstoned; it is erased when due.
func (s *Sweeper) restore(ctx context.Context, b channels.TombstoneBatch, rep *RetentionReport) error {
	err := s.store.Untombstone(ctx, b.Handle, s.clock())
	var occupied *channels.OccupiedError
	switch {
	case err == nil:
		rep.Restored++
		s.marker.Mark(batchKeys(b.Channels)...)
		return nil
	case errors.As(err, &occupied):
		return nil
	default:
		return ignoreGone(err, "restore", b.Handle)
	}
}

// ignoreGone drops the errors that mean another process settled the batch
// first, and wraps the rest.
func ignoreGone(err error, op string, h channels.TombstoneHandle) error {
	if errors.Is(err, channels.ErrNotFound) || errors.Is(err, channels.ErrConflict) {
		return nil
	}
	return fmt.Errorf("realtime: %s %s: %w", op, h, err)
}

// detachOrphans tombstones the channels of ids missing for OrphanGrace.
func (s *Sweeper) detachOrphans(ctx context.Context, cfg *realtimecfg.Compiled, rep *RetentionReport) error {
	keys, err := s.store.Keys(ctx)
	if err != nil {
		return fmt.Errorf("realtime: list channels: %w", err)
	}
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.ID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	faces, err := s.faces(ctx, ids)
	if err != nil {
		return err
	}
	now := s.clock()
	due := s.trackMissing(ids, faces, now)
	var errs []error
	for _, id := range due {
		list, err := s.store.ListEntity(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("realtime: list orphan %s: %w", id, err))
			continue
		}
		if len(list) == 0 {
			continue
		}
		f := FilingFor(cfg, channels.PolicyOrphan, now, list, false)
		if _, err := s.store.Tombstone(ctx, id, "", now, f); err != nil {
			errs = append(errs, fmt.Errorf("realtime: detach orphan %s: %w", id, err))
			continue
		}
		rep.Detached++
		s.marker.Mark(batchKeys(list)...)
	}
	return errors.Join(errs...)
}

// trackMissing records when each id in ids was first found missing and
// returns the ids missing for OrphanGrace, forgetting them. An id that
// exists, or no longer has channels, is forgotten too.
func (s *Sweeper) trackMissing(ids []string, faces map[string][]entity.Face, now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]bool, len(ids))
	var due []string
	for _, id := range ids {
		seen[id] = true
		if _, ok := faces[id]; ok {
			delete(s.missing, id)
			continue
		}
		first, ok := s.missing[id]
		switch {
		case !ok:
			s.missing[id] = now
		case now.Sub(first) >= realtimelimits.OrphanGrace:
			due = append(due, id)
			delete(s.missing, id)
		}
	}
	for id := range s.missing {
		if !seen[id] {
			delete(s.missing, id)
		}
	}
	return due
}

// clearTypeRecords drops the record of each type cfg no longer declares.
// A type some channel still uses, live or tombstoned, keeps its record.
func (s *Sweeper) clearTypeRecords(ctx context.Context, cfg *realtimecfg.Compiled, rep *RetentionReport) error {
	records, err := s.store.TypeRecords(ctx)
	if err != nil {
		return fmt.Errorf("realtime: read type records: %w", err)
	}
	var errs []error
	for typ := range records {
		if _, ok := cfg.ChannelType(typ); ok {
			continue
		}
		err := s.store.DropTypeRecord(ctx, typ)
		switch {
		case err == nil:
			rep.TypesCleared++
		case errors.Is(err, channels.ErrTypeInUse):
		default:
			errs = append(errs, fmt.Errorf("realtime: drop type record %s: %w", typ, err))
		}
	}
	return errors.Join(errs...)
}

// faces asks Anchors about ids in chunks of anchorChunk.
func (s *Sweeper) faces(ctx context.Context, ids []string) (map[string][]entity.Face, error) {
	out := map[string][]entity.Face{}
	for chunk := range slices.Chunk(ids, anchorChunk) {
		got, err := s.anchors.Faces(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("realtime: read anchors: %w", err)
		}
		for id, fs := range got {
			out[id] = fs
		}
	}
	return out, nil
}

// anchorExists reports whether a exists in faces: the id for a
// whole-entity anchor, the face for a face anchor.
func anchorExists(faces map[string][]entity.Face, a channels.Anchor) bool {
	fs, ok := faces[a.ID]
	if !ok {
		return false
	}
	return a.Face == "" || slices.Contains(fs, a.Face)
}

func batchKeys(chs []channels.Channel) []channels.Key {
	out := make([]channels.Key, len(chs))
	for i, c := range chs {
		out[i] = c.Key
	}
	return out
}
