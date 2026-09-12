package sqlitestore

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
// Cancelling the context stops the loop from starting another tick, but a
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
			if err := s.tick(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("sqlitestore: version sweep tick failed", "error", err)
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
	for _, c := range candidates {
		if capErr := s.captureOne(ctx, c, hash, projJSON); capErr != nil {
			// Best-effort per row: log and continue, so one bad row does not
			// abort the tick. The next tick retries, idempotently via the dedup.
			slog.Warn("sqlitestore: version sweep capture failed",
				"id", c.id, "face", c.face, "error", capErr)
		}
	}

	relCandidates, err := s.selectRelationCandidates(ctx)
	if err != nil {
		return err
	}
	for _, rc := range relCandidates {
		if capErr := s.captureRelation(ctx, rc, hash, projJSON); capErr != nil {
			slog.Warn("sqlitestore: relation version sweep capture failed",
				"from", rc.from, "type", rc.relType, "to", rc.to, "error", capErr)
		}
	}
	return nil
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
	hasVersion bool
}

// selectCandidates returns up to Batch entities that have SETTLED (updated_at
// older than now-Idle) or whose latest version has aged past MaxStaleness.
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
// # The ORDER BY is what drains a backlog, and it is not cosmetic
//
// `lv_created IS NOT NULL, lv_created ASC` puts never-captured rows first and
// then the longest-uncaptured ones. Ordering by `e.updated_at ASC` instead —
// the obvious choice, and pgstore's — starves a backlog larger than Batch.
//
// The reason is that the dedup happens in Go: captureOne compares content
// hashes and returns without writing when they match. A settled, already
// captured, unchanged row is therefore selected, examined and skipped, leaving
// nothing that the WHERE clause keys on any different — so the next tick
// returns the identical first Batch rows, and everything behind them is never
// reached. Not "eventually": never, for as long as those rows stay quiet.
//
// Ordering by the capture time fixes it without excluding anything, which is
// why it is done here rather than in the WHERE clause. A tick that captures
// rows moves their lv_created to now, so they sort to the BACK and the next
// tick necessarily advances. A tick that skips them changes nothing, but they
// were already the oldest captures, so the batch is still the right one to
// look at.
//
// Filtering instead — an `AND lv_created < e.updated_at` "dirty gate" — is the
// tempting version and is WRONG. A capture is written after the write it
// snapshots, so lv_created is normally NEWER than updated_at, and an edit
// landing in the same clock tick as its predecessor's capture would be
// excluded permanently. Measured while writing this: updated_at
// …:28.741109Z against lv_created …:28.741797Z, for an edit that genuinely
// needed capturing. The version rows carry no source-updated_at column to
// compare against honestly, and adding one to this backend alone would diverge
// the two schemas over a bug both share.
//
// pgstore has the same starvation and its comment claims a backlog "drains
// oldest-first across ticks", which holds only where capture advances
// something the query reads. Worth carrying back; it is not fixed here.
func (s *sweep) selectCandidates(ctx context.Context) ([]sweepCandidate, error) {
	const q = `
		SELECT e.id, e.face, e.type, e.content, e.properties,
		       (SELECT ev.content_hash FROM entity_versions ev
		         WHERE ev.entity_id = e.id AND ev.face = e.face
		           AND ev.vseq > COALESCE((SELECT max(d.vseq) FROM entity_versions d
		                                    WHERE d.entity_id = e.id AND d.face = e.face
		                                      AND d.op = 'delete'), 0)
		         ORDER BY ev.vseq DESC LIMIT 1) AS latest_hash,
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
		WHERE e.updated_at < ?
		   OR (lv_created IS NOT NULL AND lv_created < ?)
		ORDER BY lv_created IS NOT NULL, lv_created ASC, e.updated_at ASC
		LIMIT ?`

	settled, stale := s.windows()
	rows, err := s.store.db.QueryContext(ctx, q, settled, stale, s.cfg.Batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sweepCandidate
	for rows.Next() {
		var (
			c          sweepCandidate
			latestHash *string
			lvVseq     *int64
			lvOp       *string
			lvCreated  *string
		)
		if err := rows.Scan(&c.id, &c.face, &c.typ, &c.content, &c.props,
			&latestHash, &lvVseq, &lvOp, &lvCreated); err != nil {
			return nil, err
		}
		if latestHash != nil {
			c.latestHash = *latestHash
		}
		// hasVersion decides create-vs-update, and it means "this lifecycle
		// already has a version". A lineage whose newest row is a delete is a
		// re-creation, so the next capture is a CREATE again.
		c.hasVersion = lvVseq != nil && lvOp != nil && *lvOp != string(store.VersionOpDelete)
		out = append(out, c)
	}
	return out, rows.Err()
}

// windows renders the two time thresholds as on-disk timestamps.
//
// The comparison happens in SQL as a STRING compare, which is correct only
// because timeFmt is RFC3339Nano in UTC: fixed-width, zero-padded, and
// lexicographically ordered the same as chronologically. Formatting the
// thresholds in Go (rather than using SQLite's datetime()) keeps one time source
// and avoids the format mismatch that would silently make every comparison
// false.
func (s *sweep) windows() (settled, stale string) {
	now := time.Now().UTC()
	return now.Add(-s.cfg.Idle).Format(timeFmt), now.Add(-s.cfg.MaxStaleness).Format(timeFmt)
}

// captureOne snapshots one entity if its content actually changed.
func (s *sweep) captureOne(
	ctx context.Context, c sweepCandidate, schemaHash string, projJSON []byte,
) error {
	props, err := unmarshalProps(c.props)
	if err != nil {
		return err
	}
	in := store.VersionInput{
		EntityID:   c.id,
		Face:       entity.Face(c.face),
		Type:       c.typ,
		Content:    c.content,
		Properties: props,
		SchemaHash: schemaHash,
		Projection: projJSON,
		// sqlitestore's live rows carry no last_edited_by_* columns, so a swept
		// capture has no author to copy and takes the system principal. This is
		// the documented fallback, NOT a guess: attributing the sweep's own
		// write to a real user it never observed would be worse than saying
		// "version-sweep" plainly. The editing principal remains recoverable
		// from the audit log.
		PrincipalTool: sweepPrincipalTool,
	}
	contentHash := contentHashOf(in)

	// Dedup only within the CURRENT lifecycle: latestHash is empty when there is
	// no post-delete version, so a re-creation with identical bytes still
	// records rather than being swallowed.
	if c.latestHash != "" && contentHash == c.latestHash {
		return nil
	}
	if c.hasVersion {
		in.Op = store.VersionOpUpdate
	} else {
		in.Op = store.VersionOpCreate
	}
	return insertVersion(ctx, s.store.db, in, contentHash)
}

// sweepPrincipalTool is the system principal stamped on sweep-captured
// create/update versions.
const sweepPrincipalTool = "version-sweep"

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
		       r.content, r.properties,
		       (SELECT rv.content_hash FROM relation_versions rv
		         WHERE rv.rel_record_id = r.rel_record_id
		         ORDER BY rv.vseq DESC LIMIT 1) AS latest_hash,
		       (SELECT rv.created_at FROM relation_versions rv
		         WHERE rv.rel_record_id = r.rel_record_id
		         ORDER BY rv.vseq DESC LIMIT 1) AS lv_created
		FROM relations r
		WHERE r.updated_at < ?
		   OR (lv_created IS NOT NULL AND lv_created < ?)
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
			&c.content, &c.props, &latestHash, &lvCreated); err != nil {
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
	in := store.RelationVersionInput{
		RecordID:      c.recordID,
		From:          c.from,
		FromFace:      entity.Face(c.fromFace),
		Type:          c.relType,
		To:            c.to,
		Content:       c.content,
		Properties:    props,
		SchemaHash:    schemaHash,
		Projection:    projJSON,
		PrincipalTool: sweepPrincipalTool,
	}
	contentHash := contentHashOfRelation(in)
	if c.latestHash != "" && contentHash == c.latestHash {
		return nil
	}
	if c.hasVersion {
		in.Op = store.VersionOpUpdate
	} else {
		in.Op = store.VersionOpCreate
	}
	return insertRelationVersion(ctx, s.store.db, in, contentHash)
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
