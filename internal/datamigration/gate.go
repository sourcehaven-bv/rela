package datamigration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// GateStatus is the outcome of one gate evaluation.
type GateStatus string

const (
	// StatusInSync: marker hash equals the live shape hash; nothing to do.
	StatusInSync GateStatus = "in-sync"
	// StatusBootstrapped: no marker existed; the live shape was adopted as
	// the baseline (existing projects join the system without ceremony).
	StatusBootstrapped GateStatus = "bootstrapped"
	// StatusAdopted: the shape changed compatibly (additive and/or drift)
	// and the marker was moved to the live shape.
	StatusAdopted GateStatus = "adopted"
	// StatusNeedsMigration: the shape changed incompatibly; the marker was
	// NOT moved. The operator must run `rela migrate gen` / `rela migrate
	// data` (writes keep today's soft-warning behavior meanwhile).
	StatusNeedsMigration GateStatus = "needs-migration"
)

// Verdict is one published gate evaluation. The GC engine reads the LATEST
// verdict each tick and skips entirely while Status is needs-migration —
// GC must never delete data a pending migration would transform.
//
// A published Verdict is IMMUTABLE: it is shared across goroutines by
// pointer (atomic publication), so readers must never mutate Report.Deltas
// or any other field — copy first.
type Verdict struct {
	Status      GateStatus
	StoreHash   string // marker hash before evaluation ("" when bootstrapping)
	LiveHash    string
	Report      metamodel.ShapeReport // empty when hashes match
	EvaluatedAt time.Time
}

// Gate compares the store's applied shape (the state.KV marker) against the
// live metamodel and adopts compatible changes. It runs at startup AND on
// every metamodel hot-reload (amendment A4): wire Evaluate into the reload
// path, then read the latest result via Verdict(). Publication uses an
// atomic.Pointer per the state-publish rule — readers never lock.
//
// Adoption is a marker (and ledger) write; run Evaluate only from
// write-capable processes. Racing adopters write identical content, so the
// absence of a state.KV compare-and-swap is benign here; the migration
// RUNNER is the path that needs real locking and takes it itself.
type Gate struct {
	kv      state.KV
	lock    MigrationLock
	verdict atomic.Pointer[Verdict]
	now     func() time.Time
}

// NewGate builds a gate over the store's state KV. lock is OPTIONAL (nil =
// unserialized adoption): gate-vs-gate races write identical content, so the
// lock only matters against a concurrently-running migration/GC — and on
// contention the gate SKIPS persisting rather than blocking or failing
// startup (the holder is actively moving the marker; the next evaluation
// re-adopts). Pass the lock from [LockFor] wherever one is wired.
func NewGate(kv state.KV, lock MigrationLock) (*Gate, error) {
	if kv == nil {
		return nil, errors.New("datamigration: NewGate: state KV is required")
	}
	return &Gate{kv: kv, lock: lock, now: time.Now}, nil
}

// Verdict returns the most recently published evaluation, or nil before the
// first Evaluate completes.
func (g *Gate) Verdict() *Verdict {
	return g.verdict.Load()
}

// Evaluate compares the marker against the live metamodel, adopts the live
// shape when the change is compatible (recording drift in the GC ledger),
// publishes the verdict, and returns it. Notices are logged here — once per
// evaluation, not per request.
func (g *Gate) Evaluate(ctx context.Context, meta *metamodel.Metamodel) (*Verdict, error) {
	if meta == nil {
		return nil, errors.New("datamigration: Evaluate: metamodel is required")
	}
	live := meta.ShapeProjection()
	liveHash := live.Hash()
	now := g.now().UTC()

	marker, err := LoadMarker(ctx, g.kv)
	if err != nil {
		return nil, err
	}

	v := &Verdict{LiveHash: liveHash, EvaluatedAt: now}
	switch {
	case marker == nil:
		if err := g.adopt(ctx, live, nil, now); err != nil {
			return nil, err
		}
		v.Status = StatusBootstrapped
		slog.Info("datamigration.bootstrapped", "shape_hash", liveHash)

	case marker.ShapeHash == liveHash:
		v.StoreHash = marker.ShapeHash
		v.Status = StatusInSync

	default:
		v.StoreHash = marker.ShapeHash
		stored, perr := marker.ShapeProjection()
		if perr != nil {
			// A marker whose projection no longer parses is as good as
			// absent: re-baseline rather than wedging startup.
			slog.Warn("datamigration.marker_projection_unreadable", "error", perr)
			if err := g.adopt(ctx, live, nil, now); err != nil {
				return nil, err
			}
			v.Status = StatusBootstrapped
			break
		}
		v.Report = metamodel.CompareShapes(stored, live)
		if !v.Report.Compatible() {
			v.Status = StatusNeedsMigration
			for _, d := range v.Report.ByTier(metamodel.TierMigration) {
				slog.Warn("datamigration.needs_migration", "subject", d.Subject, "detail", d.Detail)
			}
			slog.Warn("datamigration.gate_blocked",
				"store_hash", marker.ShapeHash, "live_hash", liveHash,
				"hint", "run `rela migrate gen` to draft a migration, then `rela migrate data`")
			break
		}
		if err := g.adoptWithDrift(ctx, live, marker.Applied, v.Report, now); err != nil {
			return nil, err
		}
		v.Status = StatusAdopted
		for _, d := range v.Report.ByTier(metamodel.TierDrift) {
			slog.Warn("datamigration.drift_adopted", "subject", d.Subject, "detail", d.Detail)
		}
	}

	g.verdict.Store(v)
	return v, nil
}

// withLock runs one persist section (marker and/or ledger writes) under the
// migration lock when one is wired. With a contended lock the section is
// SKIPPED (logged): the holder — a migration or GC run — owns that state
// right now, and this evaluation's verdict is still published from what was
// read. The whole section runs under ONE acquisition so the gate can never
// interleave a ledger write into a GC run that holds the lock.
func (g *Gate) withLock(ctx context.Context, fn func() error) error {
	if g.lock != nil {
		release, err := g.lock.TryAcquire(ctx)
		if errors.Is(err, ErrLockHeld) {
			slog.Info("datamigration: adoption skipped — another migration or GC run holds the lock")
			return nil
		}
		if err != nil {
			return err
		}
		defer release()
	}
	return fn()
}

// adopt moves the marker to the live shape, carrying the applied list over.
func (g *Gate) adopt(ctx context.Context, live metamodel.ShapeProjection, applied []string, now time.Time) error {
	return g.withLock(ctx, func() error {
		return g.writeMarker(ctx, live, applied, now)
	})
}

func (g *Gate) writeMarker(
	ctx context.Context, live metamodel.ShapeProjection, applied []string, now time.Time,
) error {
	m, err := NewMarker(live, applied, now)
	if err != nil {
		return err
	}
	return SaveMarker(ctx, g.kv, m)
}

// adoptWithDrift adopts AND records the transition's deletion drift in the
// GC ledger, pruning entries the live schema resurrects — marker and ledger
// under a single lock acquisition (the ledger is exactly the state a
// concurrent GC apply is rewriting). Ledger persistence failing must not
// block adoption (GC bookkeeping is rebuildable via `rela migrate gc
// --scan`); it is logged instead.
func (g *Gate) adoptWithDrift(
	ctx context.Context, live metamodel.ShapeProjection, applied []string,
	report metamodel.ShapeReport, now time.Time,
) error {
	return g.withLock(ctx, func() error {
		if err := g.writeMarker(ctx, live, applied, now); err != nil {
			return err
		}
		ledger, err := LoadLedger(ctx, g.kv)
		if err != nil {
			slog.Warn("datamigration.ledger_load_failed", "error", err)
			return nil
		}
		ledger.RecordDrift(report, now)
		ledger.PruneAgainst(live)
		if err := SaveLedger(ctx, g.kv, ledger); err != nil {
			slog.Warn("datamigration.ledger_save_failed", "error", err)
		}
		return nil
	})
}

// Describe renders a verdict as a short human-readable summary for the
// startup log or `rela migrate status`.
func (v *Verdict) Describe() string {
	switch v.Status {
	case StatusInSync:
		return fmt.Sprintf("data schema in sync (shape %s)", short(v.LiveHash))
	case StatusBootstrapped:
		return fmt.Sprintf("data schema baseline adopted (shape %s)", short(v.LiveHash))
	case StatusAdopted:
		return fmt.Sprintf("schema change adopted %s → %s (%d additive, %d drift)",
			short(v.StoreHash), short(v.LiveHash),
			len(v.Report.ByTier(metamodel.TierAdditive)), len(v.Report.ByTier(metamodel.TierDrift)))
	case StatusNeedsMigration:
		return fmt.Sprintf("schema changed incompatibly %s → %s (%d deltas need migration) — run `rela migrate gen`",
			short(v.StoreHash), short(v.LiveHash), len(v.Report.ByTier(metamodel.TierMigration)))
	}
	return string(v.Status)
}

// shortHashLen is how much of a shape hash human-facing output shows.
const shortHashLen = 12

func short(hash string) string {
	if len(hash) > shortHashLen {
		return hash[:shortHashLen]
	}
	if hash == "" {
		return "(none)"
	}
	return hash
}
