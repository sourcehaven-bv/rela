package sqlitestore

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The version-reconciliation sweep (the SQLite half of TKT-9INY0Y).
//
// Capture is HYBRID, and the split is not arbitrary. Rename and delete are
// captured SYNCHRONOUSLY at the entitymanager boundary because they carry
// information a later snapshot cannot reconstruct — the old→new id, and the
// pre-delete state of a row that no longer exists. Create and update are
// captured HERE, by a debounced sweep, so a burst of edits collapses into one
// version instead of one per keystroke.
//
// # Why there is no advisory lock
//
// pgstore runs its whole tick under a session-scoped pg_try_advisory_lock,
// because several rela-server processes may share one database and must not
// sweep the same rows concurrently. sqlitedb.Open takes an exclusive sidecar
// lock and REFUSES a second process, so cross-process exclusion is already
// guaranteed — by construction, before any code in this file runs.
//
// What remains is in-process exclusion against a purge, which is a real hazard
// (a purge racing a capture-insert loses the erasure). That is Store.versionMu,
// taken by both this sweep and purge.go. The mechanism is cheaper than
// pgstore's; the guarantee is the same.
//
// Stating this matters because an ABSENT lock and a FORGOTTEN lock look
// identical in a diff.

// sweepDefaults fills zero fields with production cadence, so the zero
// SweepConfig is valid and means "use the defaults".
func sweepDefaults(c store.SweepConfig) store.SweepConfig {
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

// sweep is the periodic version-reconciliation goroutine.
type sweep struct {
	store    *Store
	provider store.ProjectionProvider
	cfg      store.SweepConfig
	cancel   context.CancelFunc
	done     chan struct{}
	// beforeCapture, when set, runs between the candidate query and the
	// captures. Tests use it to write a row the query has already read.
	beforeCapture func()
	// more is set by a tick that filled its batch and made progress, so run
	// starts the next tick at once rather than an Interval later. Touched only
	// from the run goroutine.
	more bool
}

// StartVersionSweep implements [store.VersionSweeper]: it starts the periodic
// reconciliation sweep that captures create/update versions for settled rows.
//
// The wiring layer calls this after construction, supplying a ProjectionProvider
// derived from the active metamodel (which the store deliberately does not
// hold). The sweep is stopped by Store.Close. Calling it again replaces the
// previous sweep.
//
// A nil provider disables capture rather than stamping versions with no schema
// projection — logged loudly, because silently not capturing history is the
// failure mode hardest to notice.
func (s *Store) StartVersionSweep(provider store.ProjectionProvider, cfg store.SweepConfig) {
	if provider == nil {
		slog.Warn("sqlitestore: version sweep not started (no projection provider)")
		return
	}
	sctx, cancel := context.WithCancel(context.Background())
	sw := &sweep{
		store:    s,
		provider: provider,
		cfg:      sweepDefaults(cfg),
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	s.sweepMu.Lock()
	prev := s.sweep
	s.sweep = sw
	s.sweepMu.Unlock()

	prev.stop()
	go sw.run(sctx)
}

// stop signals the loop to exit and waits for it. Idempotent and nil-safe, so
// Close can call it unconditionally.
//
// The wait is UNBOUNDED, and that is a deliberate trade on the shutdown path.
// Canceling the context stops the loop from starting another tick, but a
// statement already in flight runs to completion, so Close can block for as
// long as one selectCandidates takes. Waiting is still the right choice:
// returning early would let a tick issue its remaining inserts against a
// database the caller is about to close, which turns a slow shutdown into a
// spurious error on every run — and, with the purge path, into a capture
// landing after an erasure.
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
					slog.Warn("sqlitestore: version sweep tick failed", "error", err)
				}
			}
		}
	}
}

// tick runs one reconciliation pass: entities first, then relations, in a fixed
// deterministic order with no interleaving.
//
// The whole tick holds versionMu, so a purge cannot land mid-pass and have its
// erasure undone by a capture that was already in flight.
func (s *sweep) tick(ctx context.Context) error {
	s.store.versionMu.Lock()
	defer s.store.versionMu.Unlock()

	hash, projJSON := s.provider.Projection()
	if hash == "" {
		// No projection available (metamodel not wired) — nothing safe to stamp,
		// and a version row with an unresolvable schema_hash would violate the
		// foreign key anyway.
		return nil
	}

	candidates, err := s.selectCandidates(ctx)
	if err != nil {
		return err
	}
	if s.beforeCapture != nil {
		s.beforeCapture()
	}
	resolved := 0
	for _, c := range candidates {
		if capErr := s.captureOne(ctx, c, hash, projJSON); capErr != nil {
			// Best-effort per row: log and continue, so one bad row does not
			// abort the tick. The next tick retries, idempotently via the dedup.
			slog.Warn("sqlitestore: version sweep capture failed",
				"id", c.id, "face", c.face, "error", capErr)
			continue
		}
		resolved++
	}
	s.noteBatch(ctx, "entity", len(candidates), resolved)

	relCandidates, err := s.selectRelationCandidates(ctx)
	if err != nil {
		return err
	}
	resolved = 0
	for _, rc := range relCandidates {
		if capErr := s.captureRelation(ctx, rc, hash, projJSON); capErr != nil {
			slog.Warn("sqlitestore: relation version sweep capture failed",
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
	slog.WarnContext(ctx, "sqlitestore: version sweep stalled; every row in a full batch failed to capture, "+
		"so rows behind them are not being versioned", "kind", kind, "batch", selected)
}

// --- Entities -------------------------------------------------------------

// sweepCandidate is one entity row the sweep may snapshot: its current state
// plus the content hash of its latest version in the CURRENT lifecycle (empty
// when there is none), so captureOne can dedup without a second query.
type sweepCandidate struct {
	id string
	// face is the content-state coordinate; zero is the default face. It keys
	// the version alongside the id.
	face       string
	typ        string
	content    string
	props      string
	latestHash string
	// latestVseq/latestOp identify the row latestHash came from (0/"" when
	// there is none). The sweep ignores them; capture-now for a version tag
	// tags that row when it already holds the live content.
	latestVseq int64
	latestOp   string
	hasVersion bool
	// liveHash is the row's content_hash column; nil means not known.
	liveHash *string
	// editorUser/editorTool are the row's last_edited_by_* columns; nil means
	// the last write carried no attribution.
	editorUser *string
	editorTool *string
	// origin is the row's origin_* columns; all nil means the last write was
	// a direct edit.
	origin originCols
}

// selectCandidates returns up to Batch entities that differ from their latest
// version and have SETTLED (updated_at older than now-Idle) or whose latest
// version has aged past MaxStaleness.
//
// pgstore expresses the two "latest version" probes as LEFT JOIN LATERAL;
// SQLite has no LATERAL, so they are correlated scalar subqueries. Each is still
// a per-row index probe on entity_versions(entity_id, face, vseq DESC), not a
// whole-table aggregate.
//
// Two different notions of "latest" are needed, and conflating them is a bug:
//
//   - lv (latest of ANY op): its created_at drives the staleness ceiling, and
//     whether it is op='delete' tells us the live row is a RE-CREATION after a
//     delete, i.e. a new lifecycle.
//   - lvc (latest SINCE the last delete): the current lifecycle's newest
//     snapshot, whose content_hash is what we dedup against. Deduping against a
//     PRE-delete version would wrongly skip re-creating an entity with identical
//     bytes, leaving its timeline ending in `delete` while the row is live.
//
// Every subquery is scoped `face = e.face`. That is load-bearing rather than
// tidy: unscoped, the published capture would dedup against the draft's
// identical content_hash and be silently dropped — a MISSING version, not a
// duplicate — and one face's delete would reset another face's lifecycle
// boundary.
//
// # Only changed rows are candidates, or a backlog starves
//
// The dedup happens in Go: captureOne compares content hashes and skips the
// write when they match. A row the query selects and the dedup skips changes
// nothing the query keys on, so it is selected again on every tick, and once
// Batch such rows exist the rows behind them are never reached (BUG-1DWMYO).
// Ordering does not prevent that: an edit to a row whose capture is newer than
// Batch others still waits behind them forever.
//
// So the WHERE clause selects exactly what the dedup would capture: no version
// in this lifecycle, or a stored content_hash that is NULL or differs from that
// version's. captureOne writes back the hash it computed, and a trigger clears
// it when a hashed column changes (contentHashDDL), so a stored hash always
// describes the row's current content. A purge tombstone carries the live hash,
// so a purged row is clean too. The one exception is a row whose capture keeps
// failing: it stays a candidate, and noteBatch reports a full batch of them.
//
// Timestamps would not do. A capture is written after the write it snapshots,
// so lv_created is normally NEWER than updated_at, and an `lv_created <
// e.updated_at` gate excludes an edit landing in the same clock tick as its
// predecessor's capture permanently (measured: updated_at …:28.741109Z against
// lv_created …:28.741797Z, for an edit that needed capturing).
//
// The ORDER BY puts never-captured rows first and then the longest-uncaptured
// ones, so a large backlog drains in a stable order.
func (s *sweep) selectCandidates(ctx context.Context) ([]sweepCandidate, error) {
	const q = candidateSelect + `
		WHERE (c.updated_at < ?
		       OR (c.lv_created IS NOT NULL AND c.lv_created < ?))
		  AND (lc.content_hash IS NULL OR c.content_hash IS NULL OR c.content_hash <> lc.content_hash)
		ORDER BY c.lv_created IS NOT NULL, c.lv_created ASC, c.updated_at ASC
		LIMIT ?`

	settled, stale := s.windows()
	rows, err := s.store.db.QueryContext(ctx, q, settled, stale, s.cfg.Batch)
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

// candidateSelect reads entity rows (aliased c) with what a capture needs: the
// live content, its attribution and origin columns, the "latest version"
// probes described on selectCandidates, and the stored content_hash. Shared by the sweep and by
// capture-now for a version tag, so both decide dedup and op the same way.
// The caller appends the WHERE clause over c.
//
// The latest current-lifecycle version is found once, as a vseq, and joined
// for its hash and op, rather than repeating the delete-fence probe in two
// more correlated subqueries.
const candidateSelect = `
		SELECT c.id, c.face, c.type, c.content, c.properties,
		       c.last_edited_by_user, c.last_edited_by_tool,
		       c.origin_kind, c.origin_source, c.origin_source_face,
		       c.origin_source_type, c.origin_definition,
		       lc.content_hash, c.latest_vseq, lc.op,
		       c.lv_vseq, c.lv_op, c.content_hash
		FROM (
		    SELECT e.id, e.face, e.type, e.content, e.properties, e.updated_at,
		           e.last_edited_by_user, e.last_edited_by_tool,
		           e.origin_kind, e.origin_source, e.origin_source_face,
		           e.origin_source_type, e.origin_definition, e.content_hash,
		           (SELECT ev.vseq FROM entity_versions ev
		             WHERE ev.entity_id = e.id AND ev.face = e.face
		               AND ev.vseq > COALESCE((SELECT max(d.vseq) FROM entity_versions d
		                                        WHERE d.entity_id = e.id AND d.face = e.face
		                                          AND d.op = 'delete'), 0)
		             ORDER BY ev.vseq DESC LIMIT 1) AS latest_vseq,
		           (SELECT ev.vseq FROM entity_versions ev
		             WHERE ev.entity_id = e.id AND ev.face = e.face
		             ORDER BY ev.vseq DESC LIMIT 1) AS lv_vseq,
		           (SELECT ev.op FROM entity_versions ev
		             WHERE ev.entity_id = e.id AND ev.face = e.face
		             ORDER BY ev.vseq DESC LIMIT 1) AS lv_op,
		           (SELECT ev.created_at FROM entity_versions ev
		             WHERE ev.entity_id = e.id AND ev.face = e.face
		             ORDER BY ev.vseq DESC LIMIT 1) AS lv_created
		    FROM entities e
		) c
		LEFT JOIN entity_versions lc ON lc.vseq = c.latest_vseq`

// scanCandidate scans one candidateSelect row.
func scanCandidate(row scanner) (sweepCandidate, error) {
	var (
		c          sweepCandidate
		latestHash *string
		latestVseq *int64
		latestOp   *string
		lvVseq     *int64
		lvOp       *string
	)
	dest := make([]any, 0, 13+originColumnCount)
	dest = append(dest, &c.id, &c.face, &c.typ, &c.content, &c.props, &c.editorUser, &c.editorTool)
	dest = append(dest, c.origin.scanTargets()...)
	dest = append(dest, &latestHash, &latestVseq, &latestOp, &lvVseq, &lvOp, &c.liveHash)
	if err := row.Scan(dest...); err != nil {
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
	// hasVersion decides create-vs-update, and it means "this lifecycle
	// already has a version". A lineage whose newest row is a delete is a
	// re-creation, so the next capture is a CREATE again.
	c.hasVersion = lvVseq != nil && lvOp != nil && *lvOp != string(store.VersionOpDelete)
	return c, nil
}

// windows renders the two time thresholds as on-disk timestamps.
//
// The comparison happens in SQL as a STRING compare, which is correct only
// because every row and threshold goes through sqlitedb.FormatTime: UTC and
// fixed-width, so string order is time order (BUG-HEIAVS). Formatting the
// thresholds in Go (rather than using SQLite's datetime()) keeps one time source
// and avoids the format mismatch that would silently make every comparison
// false.
func (s *sweep) windows() (settled, stale string) {
	now := time.Now()
	return sqlitedb.FormatTime(now.Add(-s.cfg.Idle)), sqlitedb.FormatTime(now.Add(-s.cfg.MaxStaleness))
}

// captureOne captures one sweep candidate and writes its hash back.
func (s *sweep) captureOne(
	ctx context.Context, c sweepCandidate, schemaHash string, projJSON []byte,
) error {
	_, contentHash, err := captureLive(ctx, s.store.db, c, schemaHash, projJSON)
	if err != nil {
		return err
	}
	if c.liveHash != nil && *c.liveHash == contentHash {
		return nil
	}
	// Write the hash back so the row stops being a candidate until it changes,
	// whether or not a version was captured: a row whose stored hash is NULL
	// while its content matches the latest version would otherwise be selected,
	// and skipped, on every tick. The content guard makes it a no-op when a
	// hashed column changed since the candidate query read the row; stored on
	// newer content, the hash would mark an uncaptured edit as clean.
	_, err = s.store.db.ExecContext(ctx, `UPDATE entities SET content_hash = ?
		WHERE id = ? AND face = ? AND type = ? AND properties = ? AND content = ?`,
		contentHash, c.id, c.face, c.typ, c.props, c.content)
	return err
}

// captureLive snapshots one entity if its content actually changed. It
// returns the new row's vseq (0 when dedup skipped it) and the content hash.
//
// Shared by the sweep and by capture-now for a version tag (TKT-VO6VG9), so a
// tagged capture is the version the sweep would have written: same op,
// attribution, origin and hash. Capture-now does not write the hash back; the
// next sweep finds the row clean against the new version and does.
func captureLive(
	ctx context.Context, q querier, c sweepCandidate, schemaHash string, projJSON []byte,
) (vseq int64, contentHash string, err error) {
	in, contentHash, err := c.versionInput(schemaHash, projJSON)
	if err != nil {
		return 0, "", err
	}

	// Dedup only within the CURRENT lifecycle: latestHash is empty when there is
	// no post-delete version, so a re-creation with identical bytes still
	// records rather than being swallowed.
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
	user, tool := store.SweptPrincipal(c.editorUser, c.editorTool)
	in := store.VersionInput{
		EntityID:      c.id,
		Face:          entity.Face(c.face),
		Type:          c.typ,
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalUser: user,
		PrincipalTool: tool,
		// Copied off the live row, never guessed; not part of the content
		// hash, as in pgstore's sweep.
		Origin: scanOrigin(c.origin),
		Op:     store.VersionOpCreate,
	}
	if c.hasVersion {
		in.Op = store.VersionOpUpdate
	}
	return in, contentHashOf(in), nil
}

// --- Relations ------------------------------------------------------------

// relationSweepCandidate is one relation row the sweep may snapshot.
type relationSweepCandidate struct {
	recordID int64
	from     string
	// fromFace is the state-specific tail of the edge; zero is the default.
	fromFace   string
	relType    string
	to         string
	content    string
	props      string
	latestHash string
	hasVersion bool
	// liveHash, editorUser and editorTool mirror sweepCandidate's.
	liveHash   *string
	editorUser *string
	editorTool *string
}

// selectRelationCandidates is [sweep.selectCandidates] for relations.
//
// This side needs less care than the entity side: rel_record_id is minted per
// ROW and each tail is its own row, so lineages are already fenced per face and
// cannot merge. There is correspondingly no delete-fence subquery — a deleted
// relation's row is gone, so it cannot be a candidate at all.
func (s *sweep) selectRelationCandidates(ctx context.Context) ([]relationSweepCandidate, error) {
	const q = `
		SELECT r.rel_record_id, r.from_id, r.from_face, r.rel_type, r.to_id,
		       r.content, r.properties, r.last_edited_by_user, r.last_edited_by_tool,
		       (SELECT rv.content_hash FROM relation_versions rv
		         WHERE rv.rel_record_id = r.rel_record_id
		         ORDER BY rv.vseq DESC LIMIT 1) AS latest_hash,
		       (SELECT rv.created_at FROM relation_versions rv
		         WHERE rv.rel_record_id = r.rel_record_id
		         ORDER BY rv.vseq DESC LIMIT 1) AS lv_created,
		       r.content_hash
		FROM relations r
		WHERE (r.updated_at < ?
		       OR (lv_created IS NOT NULL AND lv_created < ?))
		  AND (latest_hash IS NULL OR r.content_hash IS NULL OR r.content_hash <> latest_hash)
		ORDER BY lv_created IS NOT NULL, lv_created ASC, r.updated_at ASC
		LIMIT ?`

	settled, stale := s.windows()
	rows, err := s.store.db.QueryContext(ctx, q, settled, stale, s.cfg.Batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []relationSweepCandidate
	for rows.Next() {
		var (
			c          relationSweepCandidate
			latestHash *string
			lvCreated  *string
		)
		if err := rows.Scan(&c.recordID, &c.from, &c.fromFace, &c.relType, &c.to,
			&c.content, &c.props, &c.editorUser, &c.editorTool, &latestHash, &lvCreated,
			&c.liveHash); err != nil {
			return nil, err
		}
		if latestHash != nil {
			c.latestHash = *latestHash
			c.hasVersion = true
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// captureRelation snapshots one relation if its content actually changed.
func (s *sweep) captureRelation(
	ctx context.Context, c relationSweepCandidate, schemaHash string, projJSON []byte,
) error {
	props, err := unmarshalProps(c.props)
	if err != nil {
		return err
	}
	user, tool := store.SweptPrincipal(c.editorUser, c.editorTool)
	in := store.RelationVersionInput{
		RecordID:      c.recordID,
		Key:           entity.RelationKey{From: c.from, FromFace: entity.Face(c.fromFace), Type: c.relType, To: c.to},
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalUser: user,
		PrincipalTool: tool,
	}
	contentHash := contentHashOfRelation(in)
	if c.latestHash == "" || contentHash != c.latestHash {
		if c.hasVersion {
			in.Op = store.VersionOpUpdate
		} else {
			in.Op = store.VersionOpCreate
		}
		if insErr := insertRelationVersion(ctx, s.store.db, in, contentHash); insErr != nil {
			return insErr
		}
	}
	if c.liveHash != nil && *c.liveHash == contentHash {
		return nil
	}
	// As in captureOne. The key is the row's current one, so a rename since
	// the read leaves nothing to match.
	_, err = s.store.db.ExecContext(ctx, `UPDATE relations SET content_hash = ?
		WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ?
		  AND properties = ? AND content = ?`,
		contentHash, c.from, c.fromFace, c.relType, c.to, c.props, c.content)
	return err
}

// sweepNow runs one tick synchronously. It exists for tests, which must not
// depend on a ticker interval elapsing; production always goes through run.
//
// The sweep it builds is fully formed — a closed done channel and a real
// cancel — even though tick touches neither. A zero-valued sweep works only
// for as long as nothing calls stop() on it, and stop() on a nil done channel
// blocks its caller forever: a latent deadlock one refactor away, for the cost
// of two lines here.
func sweepNow(ctx context.Context, s *Store, provider store.ProjectionProvider, cfg store.SweepConfig) error {
	done := make(chan struct{})
	close(done)
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sw := &sweep{
		store:    s,
		provider: provider,
		cfg:      sweepDefaults(cfg),
		cancel:   cancel,
		done:     done,
	}
	return sw.tick(sctx)
}

// ensure the store satisfies the sweeper capability at compile time.
var _ store.VersionSweeper = (*Store)(nil)
