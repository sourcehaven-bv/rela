package datamigration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// GateStatus is the outcome of one gate evaluation.
type GateStatus string

const (
	// StatusInSync: the recorded shape equals the live shape; nothing to do.
	StatusInSync GateStatus = "in-sync"
	// StatusBootstrapped: nothing was recorded and the project has no
	// migrations, so the live shape is a safe baseline (an existing project
	// joins the system without ceremony).
	StatusBootstrapped GateStatus = "bootstrapped"
	// StatusAdopted: the shape changed compatibly (additive and/or drift), so
	// the record moves to the live shape when persisted.
	StatusAdopted GateStatus = "adopted"
	// StatusNeedsMigration: the shape changed incompatibly; the recorded
	// state was NOT moved. The operator must run `rela migrate gen` /
	// `rela migrate data` (writes keep today's soft-warning behavior
	// meanwhile).
	StatusNeedsMigration GateStatus = "needs-migration"
	// StatusUnbaselined: no state is recorded AND migrations/ is non-empty,
	// so what the stored content conforms to is genuinely unknown.
	//
	// This is the one case where adopting the live shape would be a guess
	// with consequences: the committed migrations may be exactly the ones
	// this store still needs, and silently baselining marks them applied
	// forever. A clone of a repo whose schema.yaml moved ahead of its
	// entities is the everyday way to reach it. The operator resolves it
	// explicitly with `rela migrate data` (run them) or `rela migrate
	// baseline` (declare them already applied) — the Flyway
	// baselineOnMigrate role.
	StatusUnbaselined GateStatus = "unbaselined"
)

// Verdict is one published gate evaluation. The GC engine reads the LATEST
// verdict each tick and skips entirely while Status is needs-migration —
// GC must never delete data a pending migration would transform.
//
// A published Verdict is IMMUTABLE: it is shared across goroutines by
// face (atomic publication), so readers must never mutate Report.Deltas
// or any other field — copy first.
type Verdict struct {
	Status      GateStatus
	StoreHash   string // recorded shape hash before evaluation ("" when unrecorded)
	LiveHash    string
	Report      metamodel.ShapeReport // empty when hashes match
	EvaluatedAt time.Time

	// pendingShape and pendingApplied carry what [Gate.Persist] would write,
	// so evaluation and persistence can be separate calls without the caller
	// reassembling them (and without Persist re-reading state that may have
	// moved underneath it).
	pendingShape   metamodel.ShapeProjection
	pendingApplied []AppliedEntry
}

// Gate compares the shape the store's content conforms to against the live
// metamodel and adopts compatible changes. It runs at startup AND on every
// metamodel hot-reload (amendment A4): wire Evaluate into the reload path,
// then read the latest result via Verdict(). Publication uses an
// atomic.Pointer per the state-publish rule — readers never lock.
//
// # Evaluating is not persisting
//
// Evaluate CLASSIFIES and publishes a verdict; it never writes. Persisting an
// adoption is [Gate.Persist], which the CLI calls and long-running servers do
// not (TKT-XCJ0Y2).
//
// The split exists because the record moved into the project directory, where
// on the filesystem tier it is a git-tracked file. A server adopting at boot
// would dirty an operator's working tree, and would need a writable project
// directory on a deploy box that may not have one. It also removes the
// concurrent-start hazard outright rather than mitigating it: with no server
// writing, there is nothing for N booting instances to race over. The cost is
// that a server may serve with an unrecorded ADDITIVE change, which is
// harmless by construction — additive deltas cannot invalidate stored content
// — and the next CLI invocation records it.
type Gate struct {
	migState StateStore
	kv       state.KV
	lock     MigrationLock
	verdict  atomic.Pointer[Verdict]
	now      func() time.Time
	// hasMigrations reports whether migrations/ holds any files. It decides
	// the un-baselined case: with no state recorded, a project that has no
	// migrations can safely adopt the live shape, while one that HAS them
	// cannot (those files may be exactly what this store still needs).
	hasMigrations func(context.Context) (bool, error)
}

// GateDeps are the gate's collaborators.
type GateDeps struct {
	// MigState is the per-store migration record. Required.
	MigState StateStore
	// State is the drift ledger's home. Required.
	State state.KV
	// Lock is OPTIONAL (nil = unserialized persistence). On contention
	// [Gate.Persist] SKIPS rather than blocking or failing: the holder is a
	// migration or GC run that is actively moving this state.
	Lock MigrationLock
	// HasMigrations reports whether the project has any migration files.
	// OPTIONAL: nil means "none", which keeps the bootstrap path silent for
	// callers that have no project directory to consult.
	HasMigrations func(context.Context) (bool, error)
}

// NewGate builds a gate.
//
// Nil: MigState and State are required and rejected when nil — a gate with a
// no-op record would classify against nothing and report every store in sync.
func NewGate(deps GateDeps) (*Gate, error) {
	if deps.MigState == nil {
		return nil, errors.New("datamigration: NewGate: MigState is required")
	}
	if deps.State == nil {
		return nil, errors.New("datamigration: NewGate: State is required")
	}
	return &Gate{
		migState:      deps.MigState,
		kv:            deps.State,
		lock:          deps.Lock,
		now:           time.Now,
		hasMigrations: deps.HasMigrations,
	}, nil
}

// Verdict returns the most recently published evaluation, or nil before the
// first Evaluate completes.
func (g *Gate) Verdict() *Verdict {
	return g.verdict.Load()
}

// Evaluate classifies the store's recorded shape against the live metamodel
// and publishes the verdict. Notices are logged here — once per evaluation,
// not per request.
//
// It does NOT write. Call [Gate.Persist] to record a compatible adoption;
// only write-capable, operator-driven paths (the `rela migrate` commands) do,
// so a long-running server never touches a git-tracked file at boot.
func (g *Gate) Evaluate(ctx context.Context, meta *metamodel.Metamodel) (*Verdict, error) {
	if meta == nil {
		return nil, errors.New("datamigration: Evaluate: metamodel is required")
	}
	live := meta.ShapeProjection()
	liveHash := live.Hash()
	now := g.now().UTC()

	st, err := g.migState.Load(ctx)
	if err != nil {
		return nil, err
	}

	v := &Verdict{LiveHash: liveHash, EvaluatedAt: now, pendingShape: live}
	if st == nil {
		if v.Status, err = g.classifyUnrecorded(ctx, liveHash); err != nil {
			return nil, err
		}
	} else if err := classifyAgainst(v, st, live); err != nil {
		return nil, err
	}

	g.verdict.Store(v)
	return v, nil
}

// classifyAgainst fills in the verdict for a store that HAS a recorded shape.
func classifyAgainst(v *Verdict, st *State, live metamodel.ShapeProjection) error {
	stored, err := st.ShapeProjection()
	if err != nil {
		// A recorded projection that no longer parses cannot classify
		// anything. Unlike an absent one it is NOT re-baselined silently:
		// something wrote state this build cannot read, and guessing a
		// baseline over it would mark every pending migration applied.
		return fmt.Errorf("datamigration: recorded migration state is unreadable: %w", err)
	}
	v.StoreHash = stored.Hash()
	if v.StoreHash == v.LiveHash {
		v.Status = StatusInSync
		return nil
	}

	v.Report = metamodel.CompareShapes(stored, live)
	// Cloned, not aliased: a published Verdict is shared across goroutines and
	// documented as immutable, and this slice header points into a *State the
	// gate neither owns nor controls the lifetime of. The backends happen to
	// hand out copies today, but nothing in the StateStore contract requires
	// it, so relying on that would be an aliasing bug waiting on a new backend.
	v.pendingApplied = slices.Clone(st.Applied)
	if !v.Report.Compatible() {
		v.Status = StatusNeedsMigration
		for _, d := range v.Report.ByTier(metamodel.TierMigration) {
			slog.Warn("datamigration.needs_migration", "subject", d.Subject, "detail", d.Detail)
		}
		slog.Warn("datamigration.gate_blocked",
			"store_hash", v.StoreHash, "live_hash", v.LiveHash,
			"hint", "run `rela migrate gen` to draft a migration, then `rela migrate data`")
		return nil
	}

	v.Status = StatusAdopted
	for _, d := range v.Report.ByTier(metamodel.TierDrift) {
		slog.Warn("datamigration.drift_adopted", "subject", d.Subject, "detail", d.Detail)
	}
	return nil
}

// classifyUnrecorded decides what an absent record means.
//
// With no migrations on disk it is an ordinary new store, and the live shape
// is a safe baseline. With migrations present it is genuinely ambiguous — the
// committed files may be the ones this store still needs — so the gate refuses
// to guess and asks the operator. See [StatusUnbaselined].
func (g *Gate) classifyUnrecorded(ctx context.Context, liveHash string) (GateStatus, error) {
	has := false
	if g.hasMigrations != nil {
		var err error
		if has, err = g.hasMigrations(ctx); err != nil {
			return "", err
		}
	}
	if has {
		slog.Warn("datamigration.unbaselined",
			"hint", "this store has no recorded migration state but the project has migrations — "+
				"run `rela migrate data` to apply them, or `rela migrate baseline` to record them as already applied")
		return StatusUnbaselined, nil
	}
	slog.Info("datamigration.bootstrapped", "shape_hash", liveHash)
	return StatusBootstrapped, nil
}

// Persist records the verdict's adoption, if it has one.
//
// Only StatusBootstrapped and StatusAdopted write; every other status is a
// no-op, so a caller can persist unconditionally after evaluating. This is
// the write half of the split described on [Gate] — the CLI calls it, servers
// do not.
func (g *Gate) Persist(ctx context.Context, v *Verdict) error {
	if v == nil {
		return nil
	}
	// Evaluate read outside the lock, so the record may have moved since — a
	// concurrent `migrate data --apply` recording a file is the realistic case.
	// Writing v's captured applied list over it would UN-record that migration,
	// and on the bootstrapped path would replace a populated list with an empty
	// one. Re-read and bail if anything changed; the next evaluation sees the
	// newer state and classifies against it.
	if changed, err := g.recordMoved(ctx, v); err != nil || changed {
		return err
	}
	switch v.Status {
	case StatusBootstrapped:
		return g.adopt(ctx, v.pendingShape, nil, v.EvaluatedAt)
	case StatusAdopted:
		return g.adoptWithDrift(ctx, v.pendingShape, v.pendingApplied, v.Report, v.EvaluatedAt)
	case StatusInSync, StatusNeedsMigration, StatusUnbaselined:
		return nil
	}
	return nil
}

// recordMoved reports whether the stored record differs from what [Gate.Evaluate]
// saw, which makes v's captured adoption stale and unsafe to write.
//
// A read error is NOT treated as "unchanged": failing to confirm the record is
// where we left it is exactly when a blind overwrite is most dangerous.
func (g *Gate) recordMoved(ctx context.Context, v *Verdict) (bool, error) {
	st, err := g.migState.Load(ctx)
	if err != nil {
		return false, err
	}
	var nowHash string
	if st != nil {
		proj, perr := st.ShapeProjection()
		if perr != nil {
			return false, fmt.Errorf("datamigration: recorded migration state is unreadable: %w", perr)
		}
		nowHash = proj.Hash()
	}
	if nowHash == v.StoreHash {
		return false, nil
	}
	slog.Info("datamigration: adoption skipped — the migration record moved since it was evaluated",
		"evaluated_at_shape", short(v.StoreHash), "current_shape", short(nowHash))
	return true, nil
}

// EvaluateAndPersist is Evaluate followed by Persist — the operator-driven
// path, where recording what was just classified is the point.
func (g *Gate) EvaluateAndPersist(ctx context.Context, meta *metamodel.Metamodel) (*Verdict, error) {
	v, err := g.Evaluate(ctx, meta)
	if err != nil {
		return nil, err
	}
	if err := g.Persist(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// withLock runs one persist section (state and/or ledger writes) under the
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

// adopt moves the recorded shape to live, carrying the applied list over.
func (g *Gate) adopt(
	ctx context.Context, live metamodel.ShapeProjection, applied []AppliedEntry, now time.Time,
) error {
	return g.withLock(ctx, func() error {
		return g.writeState(ctx, live, applied, now)
	})
}

func (g *Gate) writeState(
	ctx context.Context, live metamodel.ShapeProjection, applied []AppliedEntry, now time.Time,
) error {
	st, err := NewState(live, applied, now)
	if err != nil {
		return err
	}
	return g.migState.Save(ctx, st)
}

// adoptWithDrift adopts AND records the transition's deletion drift in the
// GC ledger, pruning entries the live schema resurrects — state and ledger
// under a single lock acquisition (the ledger is exactly the state a
// concurrent GC apply is rewriting). Ledger persistence failing must not
// block adoption (GC bookkeeping is rebuildable via `rela migrate gc
// --scan`); it is logged instead.
func (g *Gate) adoptWithDrift(
	ctx context.Context, live metamodel.ShapeProjection, applied []AppliedEntry,
	report metamodel.ShapeReport, now time.Time,
) error {
	return g.withLock(ctx, func() error {
		if err := g.writeState(ctx, live, applied, now); err != nil {
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
	case StatusUnbaselined:
		// Reports the live shape like every other branch: this is the case
		// where an operator most needs it, to compare against a migration
		// file's to_projection by hand before deciding how to resolve.
		return fmt.Sprintf(
			"no recorded migration state, but this project has migrations (live shape %s) — "+
				"run `rela migrate data` to apply them, or `rela migrate baseline` to record them "+
				"as already applied", short(v.LiveHash))
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
