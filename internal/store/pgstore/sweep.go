package pgstore

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// sweepAdvisoryLockKey gates the version-reconciliation sweep to a single runner
// across a multi-process deployment. Distinct from migrateAdvisoryLockKey — the
// two locks must never alias. "RELV" (RELA Versions).
const sweepAdvisoryLockKey int64 = 0x52_45_4c_56

// ProjectionProvider yields the current render-schema projection (its content
// hash and its JSON form) for stamping onto swept create/update versions. It is
// supplied by the wiring layer, which holds the metamodel; the store itself is
// metamodel-agnostic. Called once per sweep tick, not per entity, so a hot edit
// to the metamodel is picked up on the next tick.
type ProjectionProvider interface {
	Projection() (hash string, projectionJSON []byte)
}

// SweepConfig tunes the reconciliation sweep. Zero values fall back to
// defaults (see defaultSweepConfig).
type SweepConfig struct {
	// Interval is how often a tick runs. Default 5m.
	Interval time.Duration
	// Idle is how long an entity must be un-touched (updated_at older than
	// now-Idle) before its settled state is snapshotted — the debounce. Default 5m.
	Idle time.Duration
	// MaxStaleness forces a snapshot of a continuously-edited entity whose
	// latest version is older than this, even if it never settles. Default 1h.
	MaxStaleness time.Duration
	// Batch caps how many entities one tick processes, so a bulk-import burst
	// drains across ticks instead of running unboundedly. Default 500.
	Batch int
}

func (c SweepConfig) withDefaults() SweepConfig {
	if c.Interval <= 0 {
		c.Interval = 5 * time.Minute
	}
	if c.Idle <= 0 {
		c.Idle = 5 * time.Minute
	}
	if c.MaxStaleness <= 0 {
		c.MaxStaleness = time.Hour
	}
	if c.Batch <= 0 {
		c.Batch = 500
	}
	return c
}

// sweep is the periodic version-reconciliation goroutine. It captures
// create/update versions for entities that have settled (or exceeded the
// staleness ceiling), under a non-blocking advisory lock so only one process in
// the deployment sweeps at a time. rename/delete are captured synchronously
// elsewhere (see the entitymanager version hook) — the sweep never sees them as
// distinct ops.
type sweep struct {
	pool     *pgxpool.Pool
	provider ProjectionProvider
	cfg      SweepConfig
	cancel   context.CancelFunc
	done     chan struct{}
}

// StartVersionSweep starts the periodic version-reconciliation sweep for this
// store, capturing create/update versions for settled entities. The wiring
// layer calls this after construction, supplying a ProjectionProvider derived
// from the active metamodel (which the store itself does not hold). The sweep
// is stopped by Store.Close.
//
// It requires the store's handle to be a *pgxpool.Pool (so a tick can acquire a
// single connection for the session-scoped advisory lock). If it is not — e.g.
// a store built over a bare handle in a unit test — the sweep does not start and
// this is a no-op, so create/update versions simply won't be captured in that
// configuration. Calling it more than once replaces the previous sweep.
func (s *Store) StartVersionSweep(provider ProjectionProvider, cfg SweepConfig) {
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok || provider == nil {
		// A production deployment always injects a pool and a provider; hitting
		// this branch silently disables create/update version capture, so log it
		// loudly enough to diagnose a misconfiguration (unit-test stores over a
		// bare handle land here by design and are expected).
		slog.Warn("pgstore: version sweep not started",
			"pool", ok, "provider", provider != nil)
		return
	}
	s.mu.Lock()
	prev := s.sweep
	s.sweep = startSweep(pool, provider, cfg)
	s.mu.Unlock()
	prev.stop()
}

// startSweep launches the sweep goroutine. It runs until stop() is called. The
// goroutine uses a detached lifetime context (cancelled by stop), independent
// of any request ctx.
func startSweep(pool *pgxpool.Pool, provider ProjectionProvider, cfg SweepConfig) *sweep {
	sctx, cancel := context.WithCancel(context.Background())
	s := &sweep{
		pool:     pool,
		provider: provider,
		cfg:      cfg.withDefaults(),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go s.run(sctx)
	return s
}

// stop signals the loop to exit and waits for it. Idempotent and nil-safe.
func (s *sweep) stop() {
	if s == nil {
		return
	}
	s.cancel()
	<-s.done
}

func (s *sweep) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.tick(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("pgstore: version sweep tick failed", "error", err)
			}
		}
	}
}

// tick runs one reconciliation pass. The ENTIRE tick — advisory lock, the
// candidate select, every version insert, and the unlock — runs on ONE acquired
// connection, because pg_try_advisory_lock is session-scoped: it only gates the
// connection that holds it. Issuing the inserts via the pool (different
// sessions) would let two processes both write concurrently despite each
// "holding" the lock, voiding the single-writer guarantee.
func (s *sweep) tick(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	locked, err := tryAdvisoryLock(ctx, conn, sweepAdvisoryLockKey)
	if err != nil {
		return err
	}
	if !locked {
		// Another process is sweeping — skip this tick.
		return nil
	}
	// Unlock on a context detached from cancellation: when Close cancels the
	// tick ctx mid-tick, the deferred unlock must still run its statement, or
	// the session-scoped advisory lock rides the pooled connection back into the
	// pool and locks out OTHER processes until that connection is recycled
	// (pgxpool does not reset session state on Release).
	defer advisoryUnlock(context.WithoutCancel(ctx), conn, sweepAdvisoryLockKey)

	hash, projJSON := s.provider.Projection()
	if hash == "" {
		// No projection available (metamodel not wired) — nothing safe to stamp.
		return nil
	}

	candidates, err := s.selectCandidates(ctx, conn)
	if err != nil {
		return err
	}
	for _, c := range candidates {
		if err := s.captureOne(ctx, conn, c, hash, projJSON); err != nil {
			// Best-effort per entity: log and continue so one bad row doesn't
			// abort the whole tick. Next tick retries (idempotent via dedup).
			slog.Warn("pgstore: version sweep capture failed", "id", c.id, "error", err)
		}
	}
	return nil
}

// sweepCandidate is one entity the sweep may snapshot: its current state plus
// the content hash of its latest existing version (empty if none), so
// captureOne can dedup without a second query.
type sweepCandidate struct {
	id         string
	typ        string
	content    string
	props      []byte
	latestHash string
	hasVersion bool
}

// selectCandidates returns up to Batch entities that have settled (updated_at
// older than now-Idle) OR whose latest version has aged past MaxStaleness, and
// whose current content differs from their latest version's. The LATERAL
// subquery is a per-entity index probe on entity_versions(entity_id, vseq DESC),
// not a whole-table aggregate. Ordered by updated_at so a backlog drains
// oldest-first across ticks.
func (s *sweep) selectCandidates(ctx context.Context, conn *pgxpool.Conn) ([]sweepCandidate, error) {
	// Eligibility: the entity has SETTLED (updated_at older than the idle
	// window) OR its latest version has aged past the staleness ceiling (so a
	// continuously-edited entity is still captured eventually). A brand-new
	// entity with no version is captured only once it has settled — it debounces
	// like any other, rather than being snapshotted the instant it is created.
	//
	// Two LATERALs, because delete and content-dedup need different "latest":
	//   - `lv` is the latest version of ANY op — its created_at drives the
	//     staleness ceiling, and whether it is op=delete tells us the current
	//     live entity is a RE-CREATION after a delete (a new lifecycle).
	//   - `lvc` is the latest version SINCE the last delete (i.e. the current
	//     lifecycle's newest snapshot) — its content_hash is what we dedup
	//     against. Deduping against a pre-delete version would wrongly skip
	//     re-creating an entity with identical bytes, leaving its timeline
	//     ending in `delete` while it is live.
	const q = `
		SELECT e.id, e.type, e.content, e.properties,
		       lvc.content_hash,
		       (lv.vseq IS NOT NULL AND lv.op <> 'delete') AS live_lineage
		FROM entities e
		LEFT JOIN LATERAL (
		    SELECT vseq, op, created_at FROM entity_versions ev
		    WHERE ev.entity_id = e.id ORDER BY ev.vseq DESC LIMIT 1
		) lv ON true
		LEFT JOIN LATERAL (
		    SELECT content_hash FROM entity_versions ev
		    WHERE ev.entity_id = e.id
		      AND ev.vseq > COALESCE(
		          (SELECT max(vseq) FROM entity_versions d
		           WHERE d.entity_id = e.id AND d.op = 'delete'), 0)
		    ORDER BY ev.vseq DESC LIMIT 1
		) lvc ON true
		WHERE (e.updated_at < now() - make_interval(secs => $1)
		       OR (lv.vseq IS NOT NULL AND lv.created_at < now() - make_interval(secs => $2)))
		ORDER BY e.updated_at ASC
		LIMIT $3`
	// Pass the windows as float seconds via make_interval, not Duration.String()
	// as ::interval — Go renders sub-millisecond durations with the micro sign
	// ("500µs"), which Postgres interval cannot parse.
	rows, err := conn.Query(ctx, q,
		s.cfg.Idle.Seconds(), s.cfg.MaxStaleness.Seconds(), s.cfg.Batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sweepCandidate
	for rows.Next() {
		var (
			c          sweepCandidate
			latestHash *string // NULL when there is no version in the current lifecycle
		)
		if err := rows.Scan(&c.id, &c.typ, &c.content, &c.props, &latestHash, &c.hasVersion); err != nil {
			return nil, err
		}
		if latestHash != nil {
			c.latestHash = *latestHash
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// captureOne snapshots a single candidate as a create/update version, unless its
// content is unchanged from the current lifecycle's latest version (dedup). op
// is create when the current lifecycle has no non-delete version yet (a fresh or
// re-created entity), else update. c.hasVersion is "the entity has a live
// lineage" — a version exists AND the latest op is not delete.
func (s *sweep) captureOne(
	ctx context.Context, conn *pgxpool.Conn, c sweepCandidate, schemaHash string, projJSON []byte,
) error {
	props, err := unmarshalProps(c.props)
	if err != nil {
		return err
	}
	in := store.VersionInput{
		EntityID:      c.id,
		Type:          c.typ,
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalTool: "version-sweep",
	}
	contentHash := contentHashOf(in)
	// Dedup only within the current lifecycle: latestHash is empty when there is
	// no post-delete version, so a re-creation with identical bytes still records.
	if c.latestHash != "" && contentHash == c.latestHash {
		return nil
	}
	if c.hasVersion {
		in.Op = store.VersionOpUpdate
	} else {
		in.Op = store.VersionOpCreate
	}
	return insertVersion(ctx, conn, in, contentHash)
}

// tryAdvisoryLock takes a non-blocking session advisory lock on conn, returning
// whether it was acquired. Session-scoped: released by advisoryUnlock or when
// the connection closes.
func tryAdvisoryLock(ctx context.Context, conn *pgxpool.Conn, key int64) (bool, error) {
	var ok bool
	err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

func advisoryUnlock(ctx context.Context, conn *pgxpool.Conn, key int64) {
	// The caller passes a cancellation-detached ctx so this statement runs even
	// during shutdown, explicitly releasing the session-scoped lock. If the
	// connection is already closing (shutdown race), the lock releases with the
	// connection anyway — that specific failure is expected, so don't warn on it.
	_, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, key)
	if err == nil || strings.Contains(err.Error(), "conn closed") {
		// A closing connection releases the lock with itself — expected during
		// shutdown, not worth a warning.
		return
	}
	slog.Warn("pgstore: version sweep advisory unlock failed", "error", err)
}
