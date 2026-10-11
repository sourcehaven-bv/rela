package pgstore

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// sweepAdvisoryLockKey gates the version-reconciliation sweep to a single runner
// across a multi-process deployment. Distinct from migrateAdvisoryLockKey — the
// two locks must never alias. "RELV" (RELA Versions).
const sweepAdvisoryLockKey int64 = 0x52_45_4c_56

// ProjectionProvider and SweepConfig are ALIASES for the store-package types,
// not redefinitions.
//
// Aliases specifically: a named type would make pgstore.SweepConfig and
// store.SweepConfig non-interchangeable, so a caller holding one could not pass
// it where the other is wanted — the coupling this promotion removed, back in a
// subtler form. With aliases they are the same type, so existing call sites
// keep compiling and a second backend can satisfy the capability without
// importing pgstore (TKT-L3FNEN).
type (
	ProjectionProvider = store.ProjectionProvider
	SweepConfig        = store.SweepConfig
)

// sweepDefaults fills the zero fields of cfg with pgstore's production cadence.
//
// A function rather than a method because SweepConfig is now an alias for the
// store-package type, and Go does not allow methods on a non-local type. That
// is the right split anyway: the FIELDS are a backend-neutral contract, but the
// DEFAULTS are this backend's tuning and should not be imposed on another.
func sweepDefaults(c SweepConfig) SweepConfig {
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
	// beforeCapture, when set, runs between the candidate query and the
	// captures. Tests use it to write a row the query has already read.
	beforeCapture func()
	// more is set by a tick that filled its batch and made progress, so run
	// starts the next tick at once rather than an Interval later. Touched only
	// from the run goroutine.
	more bool
	// consecutiveSkips counts ticks that could not take the lock, so sustained
	// starvation escalates to a warning. Touched only from the run goroutine
	// (tick is never called concurrently), so it needs no locking.
	consecutiveSkips int
}

// sweepSkipWarnAfter is how many consecutive lock-acquisition failures turn the
// routine debug line into a warning.
//
// A skipped tick is normally harmless — another process is sweeping the same
// schema and covers the same rows — so a handful is not worth reporting. But a
// silent skip is exactly what hid the database-global lock bug: this schema's
// create/update versions simply stopped being captured, with no error, no
// warning and no metric. Sustained skipping means this schema IS being starved
// (a runner that never finishes, or a lock that is not as scoped as intended),
// which is worth surfacing.
const sweepSkipWarnAfter = 10

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
		cfg:      sweepDefaults(cfg),
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
			// A full batch means more rows are waiting: a backlog, such as
			// every row after the content_hash migration, drains at once
			// rather than Batch rows per Interval.
			for first := true; first || (s.more && ctx.Err() == nil); first = false {
				s.more = false
				if err := s.tick(ctx); err != nil && ctx.Err() == nil {
					slog.Warn("pgstore: version sweep tick failed", "error", err)
				}
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
		// Another process is sweeping THIS schema — skip; its run covers the same
		// rows, and candidate selection is state-based, so the next tick retries
		// and nothing is lost. Non-fatal because same-schema multi-process
		// contention is the legitimate case this lock exists for.
		//
		// Escalate when it persists: a skip that never resolves means this
		// schema's versions are silently not being captured, which is the failure
		// mode a database-global lock produced. See sweepSkipWarnAfter.
		s.consecutiveSkips++
		if s.consecutiveSkips >= sweepSkipWarnAfter {
			slog.WarnContext(ctx, "pgstore: version sweep starved; another runner has held the lock "+
				"for many consecutive ticks, so this schema's create/update versions are not being captured",
				"consecutive_skips", s.consecutiveSkips)
		} else {
			slog.DebugContext(ctx, "pgstore: version sweep tick skipped; another runner holds the lock")
		}
		return nil
	}
	s.consecutiveSkips = 0
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
	if s.beforeCapture != nil {
		s.beforeCapture()
	}
	resolved := 0
	for _, c := range candidates {
		if capErr := s.captureOne(ctx, conn, c, hash, projJSON); capErr != nil {
			// Best-effort per entity: log and continue so one bad row doesn't
			// abort the whole tick. Next tick retries (idempotent via dedup).
			slog.Warn("pgstore: version sweep capture failed", "id", c.id, "error", capErr)
			continue
		}
		resolved++
	}
	s.noteBatch(ctx, "entity", len(candidates), resolved)

	// Relations are swept AFTER entities within the same locked tick (a fixed,
	// deterministic order — no interleaving). Relation create/update capture
	// mirrors the entity path; rename/delete are captured synchronously (see the
	// entitymanager version hook and the cascade-delete path in Store.DeleteEntity).
	relCandidates, err := s.selectRelationCandidates(ctx, conn)
	if err != nil {
		return err
	}
	resolved = 0
	for _, rc := range relCandidates {
		if capErr := s.captureRelation(ctx, conn, rc, hash, projJSON); capErr != nil {
			slog.Warn("pgstore: relation version sweep capture failed",
				"from", rc.from, "type", rc.relType, "to", rc.to, "error", capErr)
			continue
		}
		resolved++
	}
	s.noteBatch(ctx, "relation", len(relCandidates), resolved)
	return nil
}

// noteBatch records what one phase of a tick did with a full batch.
//
// A full batch with progress means more rows are waiting, so run ticks again
// at once. A full batch with no progress means every selected row failed to
// capture: those rows stay candidates and keep the rows behind them from being
// reached, which is BUG-1DWMYO's starvation with a different cause. It cannot
// be fixed here, so it is reported once per tick rather than only per row.
func (s *sweep) noteBatch(ctx context.Context, kind string, selected, resolved int) {
	if selected < s.cfg.Batch {
		return
	}
	if resolved > 0 {
		s.more = true
		return
	}
	slog.WarnContext(ctx, "pgstore: version sweep stalled; every row in a full batch failed to capture, "+
		"so rows behind them are not being versioned", "kind", kind, "batch", selected)
}

// sweepCandidate is one entity the sweep may snapshot: its current state plus
// the content hash of its latest existing version (empty if none), so
// captureOne can dedup without a second query. editorUser/editorTool carry the
// row's last_edited_by_* columns (nil = no recorded editor) so the captured
// version is attributed to the real author (TKT-ZIRMGM).
type sweepCandidate struct {
	id string
	// face is the face this candidate row is; zero = the default face.
	// It keys the version alongside the id (TKT-C1XUA8).
	face       string
	typ        string
	content    string
	props      []byte
	latestHash string
	// latestVseq/latestOp identify the row latestHash came from (0/"" when
	// there is none). The sweep ignores them; capture-now for a version tag
	// tags that row when it already holds the live content.
	latestVseq int64
	latestOp   string
	hasVersion bool
	// xmin is the row version the candidate query read; see
	// writeBackEntityHash.
	xmin       string
	editorUser *string
	editorTool *string
	// origin carries the row's origin_* columns (all nil = a direct edit), so
	// the captured version records HOW the bytes got there. The sweep cannot
	// reconstruct this — a copy's write context is long gone by the time a
	// tick runs — which is exactly why the write boundary stamps the row
	// (see migration 0013).
	origin originCols
}

// selectCandidates returns up to Batch entities whose current content differs
// from their latest version's and that have settled (updated_at older than
// now-Idle) OR whose latest version has aged past MaxStaleness. The LATERAL
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
	// AND its stored content_hash is NULL (BUG-1DWMYO, TASK-Y73Y9 in Atlas).
	// A non-NULL hash means the current lifecycle's latest version has that
	// hash, so the row has nothing to capture. The sweep writes the hash back
	// after it captures or skips the row (writeBackEntityHash), and triggers
	// clear it whenever that could stop being true: a hashed column changes
	// (migration 0020), or a version is inserted with another hash, is a
	// delete, or is purged, or a soft-deleted row is restored (migration
	// 0021). The gate must not select rows the dedup in captureOne then skips:
	// such a row is selected again on every tick, and once Batch of them exist
	// newer edits are never reached. A skipped row gets its hash written back,
	// so it drops out. The one exception is a row whose capture keeps failing:
	// it stays a candidate, and noteBatch reports a full batch of them.
	// Timestamps would not do: a capture is written after the write it
	// snapshots, so an edit racing a capture can be older than that capture's
	// created_at. The partial index entities_unhashed_idx serves this scan, so
	// a tick costs in proportion to the rows that changed.
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
	// PER-STATE (TKT-C1XUA8): the Step-1 skip (`e.face = ''`) is gone —
	// every face is swept, and entity_versions now keys
	// (entity_id, face, vseq) so faces cannot interleave in one lineage.
	//
	// BOTH LATERALs and the delete-fence subselect are scoped
	// `ev.face = e.face`, and that is the load-bearing detail rather
	// than a tidiness one. Scoping only the outer row would leave the inner
	// probes answering from ANY face: the published capture would dedup
	// against the draft's identical content_hash and be silently dropped —
	// a MISSING version, not a duplicate — and one face's delete would reset
	// another face's lifecycle boundary.
	//
	// The content hash also folds in the face (contentHashOf), so two
	// faces with byte-identical content hash differently. That is the
	// structural half of the same guarantee; the SQL scoping is the other.
	const q = candidateSelect + `
		WHERE e.content_hash IS NULL
		  AND (e.updated_at < now() - make_interval(secs => $1)
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
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// candidateSelect reads entity rows with what a capture needs: the live
// content, its attribution and origin columns, the two "latest version"
// probes described on selectCandidates, and the xmin that writeBackEntityHash
// uses. Shared by the sweep and by capture-now for a version tag, so both
// decide dedup and op the same way. The caller appends the WHERE clause.
const candidateSelect = `
		SELECT e.id, e.face, e.type, e.content, e.properties,
		       e.last_edited_by_user, e.last_edited_by_tool,
		       e.origin_kind, e.origin_source, e.origin_source_face,
		       e.origin_source_type, e.origin_definition,
		       lvc.content_hash, lvc.vseq, lvc.op,
		       (lv.vseq IS NOT NULL AND lv.op <> 'delete') AS live_lineage,
		       e.xmin::text
		FROM entities e
		LEFT JOIN LATERAL (
		    SELECT vseq, op, created_at FROM entity_versions ev
		    WHERE ev.entity_id = e.id AND ev.face = e.face
		    ORDER BY ev.vseq DESC LIMIT 1
		) lv ON true
		LEFT JOIN LATERAL (
		    SELECT vseq, op, content_hash FROM entity_versions ev
		    WHERE ev.entity_id = e.id AND ev.face = e.face
		      AND ev.vseq > COALESCE(
		          (SELECT max(vseq) FROM entity_versions d
		           WHERE d.entity_id = e.id AND d.face = e.face
		             AND d.op = 'delete'), 0)
		    ORDER BY ev.vseq DESC LIMIT 1
		) lvc ON true`

// scanCandidate scans one candidateSelect row.
func scanCandidate(row scanner) (sweepCandidate, error) {
	var (
		c          sweepCandidate
		latestHash *string // NULL when there is no version in the current lifecycle
		latestVseq *int64
		latestOp   *string
	)
	scanArgs := make([]any, 0, 12+originColumnCount)
	scanArgs = append(scanArgs, &c.id, &c.face, &c.typ, &c.content, &c.props,
		&c.editorUser, &c.editorTool)
	scanArgs = append(scanArgs, c.origin.scanTargets()...)
	scanArgs = append(scanArgs, &latestHash, &latestVseq, &latestOp, &c.hasVersion, &c.xmin)
	if err := row.Scan(scanArgs...); err != nil {
		return sweepCandidate{}, err
	}
	if latestHash != nil {
		c.latestHash = *latestHash
	}
	if latestVseq != nil {
		c.latestVseq = *latestVseq
	}
	if latestOp != nil {
		c.latestOp = *latestOp
	}
	return c, nil
}

// captureOne captures one sweep candidate and writes its hash back.
func (s *sweep) captureOne(
	ctx context.Context, conn *pgxpool.Conn, c sweepCandidate, schemaHash string, projJSON []byte,
) error {
	_, contentHash, err := captureLive(ctx, conn, c, schemaHash, projJSON)
	if err != nil {
		return err
	}
	return writeBackEntityHash(ctx, conn, c, contentHash)
}

// captureLive snapshots a candidate as a create/update version, unless its
// content is unchanged from the current lifecycle's latest version (dedup).
// It returns the new row's vseq (0 when dedup skipped the capture) and the
// content hash. op is create when the current lifecycle has no non-delete
// version yet (a fresh or re-created entity), else update. c.hasVersion is
// "the entity has a live lineage": a version exists AND the latest op is not
// delete.
//
// Shared by the sweep and by capture-now for a version tag (TKT-VO6VG9), so a
// tagged capture is the version the sweep would have written: same op,
// attribution, origin and hash. Capture-now does not write the hash back. A
// row with a NULL hash stays a candidate: the next sweep dedups it against the
// new version and writes the hash back then. A row with a stored hash keeps
// it, which stays true: that hash is the live content's, and the new version
// carries the same hash, so the version trigger leaves it alone.
func captureLive(
	ctx context.Context, q DBTX, c sweepCandidate, schemaHash string, projJSON []byte,
) (vseq int64, contentHash string, err error) {
	in, contentHash, err := c.versionInput(schemaHash, projJSON)
	if err != nil {
		return 0, "", err
	}
	// Dedup only within the current lifecycle: latestHash is empty when there is
	// no post-delete version, so a re-creation with identical bytes still records.
	//
	// ORIGIN IS DELIBERATELY NOT IN THE HASH. History records content changes,
	// not write events, and folding provenance in would mint a version whose
	// content is byte-identical to its predecessor purely because the writer
	// differed. The visible consequence is that a NO-OP copy (target already
	// equals source) records no version — correct under this contract, and the
	// audit log (audit.OpCopyState) is where every copy INVOCATION is recorded
	// whether or not it changed anything.
	if c.latestHash != "" && contentHash == c.latestHash {
		return 0, contentHash, nil
	}
	vseq, err = insertVersion(ctx, q, in, contentHash)
	return vseq, contentHash, err
}

// versionInput builds the version a capture of c writes, and its content
// hash. Attribution comes from the row's last_edited_by_* columns, never from
// whoever triggers the capture: they did not write these bytes.
func (c sweepCandidate) versionInput(schemaHash string, projJSON []byte) (store.VersionInput, string, error) {
	props, err := unmarshalProps(c.props)
	if err != nil {
		return store.VersionInput{}, "", err
	}
	principalUser, principalTool := store.SweptPrincipal(c.editorUser, c.editorTool)
	in := store.VersionInput{
		EntityID:      c.id,
		Face:          entity.Face(c.face),
		Type:          c.typ,
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalUser: principalUser,
		PrincipalTool: principalTool,
		// Copied verbatim off the live row, never guessed: all-NULL columns
		// mean the last write was a direct edit, and that stays the zero
		// Origin rather than becoming a literal "manual".
		Origin: scanOrigin(c.origin),
		Op:     store.VersionOpCreate,
	}
	if c.hasVersion {
		in.Op = store.VersionOpUpdate
	}
	return in, contentHashOf(in), nil
}

// writeBackEntityHash stores the hash the sweep computed on the live row, so
// the row stops being a candidate until a trigger clears it again.
//
// It runs whether or not a version was captured: a row whose content matches
// the latest version would otherwise be selected, and skipped, on every tick.
//
// Two guards make it a no-op when storing the hash would break what a stored
// hash promises, that the latest version has it:
//
//   - xmin: the row was written after the candidate query read it. The hash
//     describes the content that was read; stored on newer content it would
//     mark an uncaptured edit as clean. A concurrent writer still in flight is
//     waited for, and the guard is then checked against its row.
//   - the latest version: a version written outside the sweep since the read,
//     such as a delete followed by a re-create, would otherwise be followed by
//     a hash the version triggers never get to clear, because the row's hash
//     was still NULL when that version was written.
func writeBackEntityHash(ctx context.Context, conn *pgxpool.Conn, c sweepCandidate, hash string) error {
	_, err := conn.Exec(ctx, `UPDATE entities SET content_hash = $1
		WHERE id = $2 AND face = $3 AND xmin = $4::xid
		  AND EXISTS (SELECT 1 FROM (
		      SELECT op, content_hash FROM entity_versions
		      WHERE entity_id = $2 AND face = $3 ORDER BY vseq DESC LIMIT 1) l
		    WHERE l.op <> 'delete' AND l.content_hash = $1)`, hash, c.id, c.face, c.xmin)
	return err
}

// relationSweepCandidate is one relation the sweep may snapshot: its current
// state plus the surrogate rel_record_id off the live row and the content hash
// of its latest existing version in that lineage (empty if none). editorUser/
// editorTool mirror sweepCandidate's attribution columns.
type relationSweepCandidate struct {
	recordID int64
	from     string
	// fromFace is the state-specific TAIL of the edge; zero = default.
	fromFace   string
	relType    string
	to         string
	content    string
	props      []byte
	latestHash string
	hasVersion bool
	// xmin mirrors sweepCandidate's.
	xmin       string
	editorUser *string
	editorTool *string
}

// selectRelationCandidates returns up to Batch relations that have settled (or
// whose latest version aged past MaxStaleness) and whose stored content_hash is
// NULL, for the reason given at selectCandidates.
//
// Unlike the entity query this needs only ONE LATERAL: the relation carries its
// stable rel_record_id on the row, and a delete+recreate mints a FRESH
// rel_record_id, so "the latest version for THIS lineage" (matched by
// rel_record_id) is already delete-fenced by construction — no second
// since-last-delete probe is needed. A relation with no version in its lineage
// is a create; otherwise an update.
func (s *sweep) selectRelationCandidates(
	ctx context.Context, conn *pgxpool.Conn,
) ([]relationSweepCandidate, error) {
	// PER-STATE (TKT-C1XUA8): the Step-1 skip (`r.from_face = ''`) is
	// gone, symmetric with the entity scan above.
	//
	// This side needed less than the entity side: rel_record_id is minted
	// per ROW and each tail is its own row since 0011, so lineages were
	// already fenced per face and could not merge. What the tail is needed
	// for is the rename STITCH, which matches a predecessor by the triple
	// (prev_from, rel_type, prev_to) and cannot otherwise tell a
	// state-tailed edge from a default-tail one — hence from_face on
	// relation_versions and in the stitch's predicate.
	const q = `
		SELECT r.rel_record_id, r.from_id, r.from_face, r.rel_type, r.to_id,
		       r.content, r.properties,
		       r.last_edited_by_user, r.last_edited_by_tool,
		       lv.content_hash,
		       (lv.vseq IS NOT NULL) AS has_version,
		       r.xmin::text
		FROM relations r
		LEFT JOIN LATERAL (
		    SELECT vseq, content_hash, created_at FROM relation_versions rv
		    WHERE rv.rel_record_id = r.rel_record_id ORDER BY rv.vseq DESC LIMIT 1
		) lv ON true
		WHERE r.content_hash IS NULL
		  AND (r.updated_at < now() - make_interval(secs => $1)
		       OR (lv.vseq IS NOT NULL AND lv.created_at < now() - make_interval(secs => $2)))
		ORDER BY r.updated_at ASC
		LIMIT $3`
	rows, err := conn.Query(ctx, q,
		s.cfg.Idle.Seconds(), s.cfg.MaxStaleness.Seconds(), s.cfg.Batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []relationSweepCandidate
	for rows.Next() {
		var (
			c          relationSweepCandidate
			latestHash *string // NULL when the lineage has no version yet
		)
		if err := rows.Scan(&c.recordID, &c.from, &c.fromFace, &c.relType, &c.to,
			&c.content, &c.props, &c.editorUser, &c.editorTool,
			&latestHash, &c.hasVersion, &c.xmin); err != nil {
			return nil, err
		}
		if latestHash != nil {
			c.latestHash = *latestHash
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// captureRelation snapshots a single relation candidate as a create/update
// version, unless its content is unchanged from the lineage's latest version
// (dedup). op is create when the lineage has no version yet, else update.
func (s *sweep) captureRelation(
	ctx context.Context, conn *pgxpool.Conn, c relationSweepCandidate, schemaHash string, projJSON []byte,
) error {
	props, err := unmarshalProps(c.props)
	if err != nil {
		return err
	}
	principalUser, principalTool := store.SweptPrincipal(c.editorUser, c.editorTool)
	in := store.RelationVersionInput{
		RecordID:      c.recordID,
		Key:           entity.RelationKey{From: c.from, FromFace: entity.Face(c.fromFace), Type: c.relType, To: c.to},
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalUser: principalUser,
		PrincipalTool: principalTool,
	}
	contentHash := contentHashOfRelation(in)
	if c.latestHash == "" || contentHash != c.latestHash {
		if c.hasVersion {
			in.Op = store.VersionOpUpdate
		} else {
			in.Op = store.VersionOpCreate
		}
		if insErr := insertRelationVersion(ctx, conn, in, contentHash); insErr != nil {
			return insErr
		}
	}
	// As writeBackEntityHash. The key is the row's current one: a rename
	// re-keys it in place, which the xmin guard catches as well. The relation
	// dedup does not fence on a delete version, so neither does this guard.
	_, err = conn.Exec(ctx, `UPDATE relations SET content_hash = $1
		WHERE from_id = $2 AND from_face = $3 AND rel_type = $4 AND to_id = $5
		  AND xmin = $6::xid
		  AND $1 = (SELECT content_hash FROM relation_versions
		            WHERE rel_record_id = $7 ORDER BY vseq DESC LIMIT 1)`,
		contentHash, c.from, c.fromFace, c.relType, c.to, c.xmin, c.recordID)
	return err
}

// tryAdvisoryLock takes a non-blocking session advisory lock on conn, returning
// whether it was acquired. Session-scoped: released by advisoryUnlock or when
// the connection closes.
//
// key is always sweepAdvisoryLockKey — there is exactly ONE version lock, shared
// by the reconciliation sweep and version purge so they are mutually exclusive
// (a purge racing a sweep capture-insert would lose the erasure). It is a
// parameter only so the lock/unlock pair reads symmetrically.
//
// The lock is SCHEMA-SCOPED via the two-key form
// pg_try_advisory_lock(key, hashtext(current_schema())): advisory locks are
// database-GLOBAL, but many schemas can share one database (the conformance
// harness and the postgres e2e both run isolated schemas on one DB), and a
// per-schema sweep must not starve another schema's sweep. Folding the schema
// hash into the second key makes each schema's version lock independent while
// keeping sweep⇔purge mutual exclusion WITHIN a schema. purge must use the
// identical two-key form (see purge.go) or the two would stop excluding.
func tryAdvisoryLock(ctx context.Context, conn *pgxpool.Conn, key int64) (bool, error) {
	var ok bool
	err := conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock($1::int, hashtext(current_schema()))`, key).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		// coverage-ignore: defensive: SELECT pg_try_advisory_lock always returns exactly one row, so QueryRow.Scan
		// never yields pgx.ErrNoRows
		// here.
		return false, nil
	}
	return ok, err
}

func advisoryUnlock(ctx context.Context, conn *pgxpool.Conn, key int64) {
	// The caller passes a cancellation-detached ctx so this statement runs even
	// during shutdown, explicitly releasing the session-scoped lock. If the
	// connection is already closing (shutdown race), the lock releases with the
	// connection anyway — that specific failure is expected, so don't warn on it.
	// Two-key form must match tryAdvisoryLock's (schema-scoped).
	_, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1::int, hashtext(current_schema()))`, key)
	if err == nil || strings.Contains(err.Error(), "conn closed") {
		// A closing connection releases the lock with itself — expected during
		// shutdown, not worth a warning.
		return
	}
	slog.Warn("pgstore: version sweep advisory unlock failed", "error", err)
}
