package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// legacyMarkerKey is where the migration record lived before TKT-XCJ0Y2 moved
// it out of state.KV and into a per-backend store.
const legacyMarkerKey = "migration/state.json"

// legacyMarker is the pre-TKT-XCJ0Y2 on-disk shape.
//
// Only the fields the upgrade needs are declared; the rest of the old document
// is ignored. Its `applied` was a bare name list, which is why [AppliedEntry]
// timestamps are synthesized rather than recovered on the way across.
type legacyMarker struct {
	ShapeHash  string          `json:"shape_hash"`
	Projection json.RawMessage `json:"projection"`
	Applied    []string        `json:"applied,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// LegacyBridge adapts a [StateStore] to a store that still has to interoperate
// with the pre-TKT-XCJ0Y2 state.KV marker, in both directions:
//
//   - READ: when the new store has no record but the legacy marker does, the
//     marker is adopted. That is the upgrade path, and it must not be a
//     re-baseline — a store whose marker says three migrations have run would
//     otherwise be treated as brand new and replay all three.
//   - WRITE: every save also writes the legacy marker. That is the DOWNGRADE
//     path. Rolling a binary back is a normal response to an unrelated problem,
//     and an older rela that found no marker would silently baseline the store
//     against the live schema and strand every pending migration.
//
// The legacy write is best-effort and never fails a save: it is a courtesy to
// a version that may never run again, and failing the authoritative write to
// protect a hypothetical rollback would be the wrong trade. It is logged.
//
// This is a transitional wrapper. Once no supported version reads the legacy
// key, delete this file and unwrap at the wiring site.
type LegacyBridge struct {
	next StateStore
	kv   state.KV
	now  func() time.Time
}

// NewLegacyBridge wraps next so it also reads and maintains the legacy marker.
//
// Nil: kv may be nil, in which case the bridge is a pass-through — a tier with
// no state.KV has no legacy marker to reconcile. next is required.
func NewLegacyBridge(next StateStore, kv state.KV) (StateStore, error) {
	if next == nil {
		return nil, errors.New("datamigration: NewLegacyBridge: next store is required")
	}
	if kv == nil {
		return next, nil
	}
	return &LegacyBridge{next: next, kv: kv, now: time.Now}, nil
}

// Load returns the new store's record, falling back to the legacy marker.
func (b *LegacyBridge) Load(ctx context.Context) (*State, error) {
	st, err := b.next.Load(ctx)
	if err != nil || st != nil {
		return st, err
	}
	return b.loadLegacy(ctx)
}

// loadLegacy reads the pre-TKT-XCJ0Y2 marker, or (nil, nil) when there is none.
//
// A corrupt legacy marker is treated as ABSENT with a warning rather than as an
// error, which is how the old loader behaved. The reasoning still holds here
// and is narrower than it looks: this path is only reached when the new store
// has nothing, so the fallback is the same bootstrap the operator would get if
// the old marker had never existed.
func (b *LegacyBridge) loadLegacy(ctx context.Context) (*State, error) {
	data, err := b.kv.Get(ctx, legacyMarkerKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil // (nil, nil) = un-bootstrapped, the StateStore contract
		}
		return nil, nil //nolint:nilnil // an unreadable legacy marker falls back to bootstrap, as the old loader did
	}
	var m legacyMarker
	if err := json.Unmarshal(data, &m); err != nil {
		slog.Warn("datamigration.legacy_marker_corrupt", "key", legacyMarkerKey, "error", err)
		return nil, nil //nolint:nilnil // ditto: corrupt legacy state is absent, not fatal
	}
	if m.ShapeHash == "" || len(m.Projection) == 0 {
		return nil, nil //nolint:nilnil // an incomplete legacy marker carries nothing to adopt
	}

	// The legacy applied list had no timestamps. UpdatedAt is the closest
	// honest stand-in — it is when the marker last moved, which for the final
	// entry is exactly right and for earlier ones is an upper bound. Inventing
	// time.Now() would claim every historical migration ran during the upgrade.
	entries := make([]AppliedEntry, 0, len(m.Applied))
	for _, name := range m.Applied {
		if _, err := ParseMigrationName(name); err != nil {
			// A legacy name predates the timestamp scheme (0001-foo.yaml), so
			// it cannot be carried across as-is. Dropping it would replay the
			// migration; the operator is told to baseline instead.
			slog.Warn("datamigration.legacy_name_unconvertible",
				"name", name,
				"hint", "this migration predates the timestamp naming scheme — rename the file and run "+
					"`rela migrate baseline` to record the applied set explicitly")
			continue
		}
		entries = append(entries, AppliedEntry{Name: name, AppliedAt: m.UpdatedAt})
	}

	slog.Info("datamigration.legacy_marker_adopted", "applied", len(entries))
	return &State{
		FormatVersion: StateFormatVersion,
		Applied:       entries,
		Projection:    m.Projection,
		UpdatedAt:     m.UpdatedAt,
	}, nil
}

// Save writes the new record, then mirrors it to the legacy marker.
func (b *LegacyBridge) Save(ctx context.Context, s *State) error {
	if err := b.next.Save(ctx, s); err != nil {
		return err
	}
	b.mirror(ctx, s)
	return nil
}

// mirror best-effort writes the legacy marker so a rollback finds a CURRENT
// one. Failures are logged, never returned: the authoritative write already
// succeeded, and refusing it to protect a rollback that may never happen would
// trade a real success for a hypothetical one.
func (b *LegacyBridge) mirror(ctx context.Context, s *State) {
	proj, err := s.ShapeProjection()
	if err != nil {
		slog.Warn("datamigration.legacy_mirror_failed", "error", err)
		return
	}
	data, err := json.Marshal(legacyMarker{
		ShapeHash:  proj.Hash(),
		Projection: s.Projection,
		Applied:    s.AppliedNames(),
		UpdatedAt:  s.UpdatedAt,
	})
	if err != nil {
		slog.Warn("datamigration.legacy_mirror_failed", "error", err)
		return
	}
	if err := b.kv.Put(ctx, legacyMarkerKey, data); err != nil {
		slog.Warn("datamigration.legacy_mirror_failed", "error", err)
	}
}

// Interface check.
var _ StateStore = (*LegacyBridge)(nil)
