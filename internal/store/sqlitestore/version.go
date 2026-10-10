package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// VersionStore is sqlitestore's content-versioning service: the SQLite
// implementation of [store.VersionService].
//
// It shares the Store's database handle rather than opening its own, so the
// history a caller reads can never be from a different connection than the
// writes it describes — and, more practically, because sqlitestore is
// single-process with one pool and a second handle would contend with itself.
type VersionStore struct {
	db *sql.DB
	// store is the owning Store, consulted for live-row state during purge.
	store *Store
}

// VersionStore returns the versioning service backed by this store's handle.
//
// Returns the [store.VersionService] INTERFACE rather than the concrete type so
// the capability is satisfiable without importing this package (TKT-L3FNEN),
// matching pgstore. Never nil: the wrapper holds only handles and cannot fail
// to construct — a nil here would box into a non-nil interface and defer the
// failure to the first write (see appbuild's nonNilCapability).
func (s *Store) VersionStore() store.VersionService {
	return &VersionStore{db: s.db, store: s}
}

// timestampNow renders a capture timestamp in the store's on-disk format.
//
// Version rows carry their own created_at rather than defaulting in SQL,
// because SQLite's CURRENT_TIMESTAMP has second granularity and renders in a
// format that is not sqlitedb.TimeFormat. Two versions captured in the same second must
// still be distinguishable and parseable by the same code that reads
// entities.updated_at.
func timestampNow() string { return sqlitedb.FormatTime(time.Now()) }

// --- Write ----------------------------------------------------------------

// WriteVersion implements [store.VersionWriter]: it persists one synchronously
// captured version (rename or delete).
//
// The schema-dedup insert and the entity_versions insert run in ONE transaction
// so the version row is all-or-nothing — a crash between them would otherwise
// leave a version row referencing a schema projection that was never stored,
// and the foreign key would then reject the row that already committed.
func (v *VersionStore) WriteVersion(ctx context.Context, in store.VersionInput) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlitestore: begin version write: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	if _, err := insertVersion(ctx, tx, in, contentHashOf(in)); err != nil {
		return err
	}
	return tx.Commit()
}

// contentHashOf computes the dedup hash of a captured snapshot. The face is
// part of the hashed entity, so two faces holding identical bytes do not dedup
// against one another.
func contentHashOf(in store.VersionInput) string {
	e := entity.Entity{
		ID:         in.EntityID,
		Face:       in.Face,
		Type:       in.Type,
		Properties: in.Properties,
		Content:    in.Content,
	}
	return canonical.HashEntity(e)
}

// ensureSchemaVersion stores the render-schema projection if its hash is not
// already present. Deduped by primary key, so an unchanged metamodel across
// many captures stores exactly one row.
func ensureSchemaVersion(ctx context.Context, q querier, hash string, projection []byte, now string) error {
	const ins = `INSERT INTO schema_versions (hash, projection, captured_at)
	             VALUES (?, ?, ?) ON CONFLICT (hash) DO NOTHING`
	// An EMPTY projection is refused rather than defaulted to `{}`.
	//
	// Every real caller supplies one — the sweep from its ProjectionProvider,
	// the synchronous hook from the request — so an empty one means the
	// projection was never wired, which is a bug at the CALL site. Writing
	// `{}` would accept it, and the version row would then render as an entity
	// with no declared properties: wrong output, discovered at read time, long
	// after the write that caused it. Failing here names the write instead.
	//
	// pgstore reaches the same outcome by letting the NOT NULL constraint
	// reject it; this says so in a sentence the caller can act on.
	if len(projection) == 0 {
		return fmt.Errorf(
			"sqlitestore: version for schema hash %q carries no render projection "+
				"(the caller must supply one; a version with no projection cannot be rendered)",
			hash)
	}
	proj := string(projection)
	if _, err := q.ExecContext(ctx, ins, hash, proj, now); err != nil {
		return fmt.Errorf("sqlitestore: ensure schema_version: %w", err)
	}
	return nil
}

// insertVersion writes one entity_versions row through q (a pool handle or a
// transaction) and returns its vseq. The caller supplies the content hash.
func insertVersion(ctx context.Context, q querier, in store.VersionInput, contentHash string) (int64, error) {
	props, err := marshalProps(in.Properties)
	if err != nil {
		return 0, err
	}
	// ONE timestamp for the whole insert: the schema projection's captured_at
	// and the version's created_at describe the same capture, and two
	// time.Now() reads can straddle a second boundary and disagree about when
	// it happened.
	now := timestampNow()
	if schemaErr := ensureSchemaVersion(ctx, q, in.SchemaHash, in.Projection, now); schemaErr != nil {
		return 0, schemaErr
	}
	// prev_id is meaningful only on a rename row; leaving it NULL elsewhere is
	// what makes the partial index on it small and the lineage walk cheap.
	var prev *string
	if in.Op == store.VersionOpRename && in.PrevID != "" {
		prev = &in.PrevID
	}
	o := originColumns(in.Origin)
	const ins = `
		INSERT INTO entity_versions
		    (entity_id, face, op, prev_id, type, content, properties, content_hash,
		     schema_hash, principal_user, principal_tool, triggered_by,
		     origin_kind, origin_source, origin_source_face, origin_source_type,
		     origin_definition, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := q.ExecContext(ctx, ins,
		in.EntityID, string(in.Face), string(in.Op), prev, in.Type, in.Content,
		props, contentHash, in.SchemaHash,
		in.PrincipalUser, in.PrincipalTool, in.TriggeredBy,
		o.kind, o.source, o.sourceFace, o.sourceType, o.definition,
		now)
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: insert entity version: %w", err)
	}
	// vseq is the INTEGER PRIMARY KEY, i.e. the rowid, so the last insert id
	// is the new row's vseq.
	vseq, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sqlitestore: read new version vseq: %w", err)
	}
	return vseq, nil
}

// --- Origin columns -------------------------------------------------------

// originCols is a [store.Origin] as nullable SQL values. Grouped in a struct
// rather than returned as five bare *string results: five same-typed returns at
// a call site are a swap waiting to happen, and these columns travel together
// through every query here.
type originCols struct {
	kind       *string
	source     *string
	sourceFace *string
	sourceType *string
	definition *string
}

// originColumnCount is how many columns an originCols occupies, for sizing scan
// argument slices.
const originColumnCount = 5

// originColumns splits a [store.Origin] into its nullable SQL values. A zero
// Origin is all-NULL — the "direct edit" encoding — and an empty component of a
// NON-zero origin also stays NULL, so "not applicable" and "empty string" never
// collide.
func originColumns(o store.Origin) originCols {
	if o.IsZero() {
		return originCols{}
	}
	var c originCols
	k := string(o.Kind)
	c.kind = &k
	if o.Source != "" {
		c.source = &o.Source
	}
	if o.SourceFace != "" {
		c.sourceFace = &o.SourceFace
	}
	if o.SourceType != "" {
		c.sourceType = &o.SourceType
	}
	if o.Definition != "" {
		c.definition = &o.Definition
	}
	return c
}

// scanTargets returns the scan destinations for the origin columns, in the
// order originColumns emits them.
func (c *originCols) scanTargets() []any {
	return []any{&c.kind, &c.source, &c.sourceFace, &c.sourceType, &c.definition}
}

// scanOrigin assembles a [store.Origin] from its nullable columns. All-NULL
// yields the zero Origin, i.e. a direct edit.
func scanOrigin(c originCols) store.Origin {
	var o store.Origin
	if c.kind != nil {
		o.Kind = store.OriginKind(*c.kind)
	}
	if c.source != nil {
		o.Source = *c.source
	}
	if c.sourceFace != nil {
		o.SourceFace = *c.sourceFace
	}
	if c.sourceType != nil {
		o.SourceType = *c.sourceType
	}
	if c.definition != nil {
		o.Definition = *c.definition
	}
	return o
}

// --- Lineage --------------------------------------------------------------

// lineageCTE is the recursive term producing (entity_id, lo, hi) rows: each is
// one segment of the queried id's history, fenced to the vseq window in which
// that id actually belonged to THIS entity.
//
// The fencing is the whole point, and a flat `WHERE entity_id = ?` is the bug
// it exists to prevent. Ids can be renamed away and later reused, so the same
// string can name two unrelated entities at different times; reading flat
// merges their histories into one timeline. Each segment therefore carries
// [lo, hi): lo is the vseq of the most recent rename that moved this id AWAY
// (everything at or before that belonged to someone else), hi is the rename
// that moved it INTO the current lineage (exclusive; NULL = still current).
//
// Parameters: ?1 = entity id, ?2 = face. The face scopes every subselect —
// without it one face's rename would set another face's fence.
//
// Ported from pgstore and verified against SQLite before use, including the
// id-reuse case: an A→B→C chain with a second, unrelated entity later created
// as 'A' returns exactly the first A's rows and excludes the reused one.
const lineageCTE = `
	WITH RECURSIVE lin(entity_id, lo, hi) AS (
	    SELECT CAST(? AS TEXT),
	           COALESCE((SELECT max(vseq) FROM entity_versions
	                     WHERE prev_id = ? AND op = 'rename' AND face = CAST(? AS TEXT)), 0),
	           CAST(NULL AS INTEGER)
	    UNION
	    SELECT r.prev_id,
	           COALESCE((SELECT max(vseq) FROM entity_versions
	                     WHERE prev_id = r.prev_id AND op = 'rename' AND vseq < r.vseq
	                       AND face = CAST(? AS TEXT)), 0),
	           r.vseq
	    FROM lin
	    JOIN entity_versions r
	      ON r.entity_id = lin.entity_id
	     AND r.face = CAST(? AS TEXT)
	     AND r.op = 'rename'
	     AND r.prev_id IS NOT NULL
	     AND (lin.hi IS NULL OR r.vseq < lin.hi)
	)`

// lineageArgs returns the CTE's leading bind values. lineageCTE references the
// id twice and the face three times; passing them positionally at every call
// site is how a query silently ends up fencing on the wrong face.
func lineageArgs(id string, face entity.Face) []any {
	f := string(face)
	// Full-slice expression: callers append their own trailing binds to this,
	// and capping the capacity makes the copy unconditional rather than
	// conditional on this function continuing to return a fresh literal.
	args := []any{id, id, f, f, f}
	return args[:len(args):len(args)]
}

// lineageJoin attaches entity_versions rows (aliased ev) to their fenced
// segment: a row belongs if its id matches a segment and its vseq falls inside
// that segment's (lo, hi) window. The face is re-checked here because a segment
// is only meaningful within one face.
//
// The caller must DEDUP BY ROW: a rename diamond can match one row twice.
// SELECT DISTINCT does that where vseq is projected; where it is not, GROUP BY
// ev.vseq is the equivalent (see GetVersion, which explains why the two
// are not interchangeable there).
const lineageJoin = `
		JOIN lin ON lin.entity_id = ev.entity_id
		        AND ev.face = CAST(? AS TEXT)
		        AND ev.vseq > lin.lo
		        AND (lin.hi IS NULL OR ev.vseq < lin.hi)`

// --- Read -----------------------------------------------------------------

// ListVersions implements [store.HistoryReader]: the fenced lineage walk of
// one face. Ref{ID: id} is the implicit face of a faceless type. Each row
// carries the current lifecycle's version tags.
func (v *VersionStore) ListVersions(ctx context.Context, ref entity.Ref) ([]store.VersionMeta, error) {
	rows, err := listLineage(ctx, v.db, ref)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	tags, err := readTags(ctx, v.db, storeutil.LineageVseqs(rows))
	if err != nil {
		return nil, err
	}
	storeutil.ApplyTags(rows, tags)
	return storeutil.Metas(rows), nil
}

// listLineage reads the fenced lineage of one face through q, oldest first,
// with ordinals assigned. Each row also carries its vseq, the id it was
// captured under and its segment's upper fence, which the tag lifecycle rule
// needs.
func listLineage(ctx context.Context, q querier, ref entity.Ref) ([]storeutil.LineageRow, error) {
	id, p := ref.ID, ref.Face
	// GROUP BY vseq dedups the rename diamond by row identity (see
	// GetVersion). A row matched by two segments gets the smaller fence;
	// DISTINCT over (vseq, hi) would return it twice with different fences.
	sel := lineageCTE + `
		SELECT ev.vseq, ev.op, ev.prev_id, ev.type, ev.content_hash, ev.schema_hash,
		       ev.principal_user, ev.principal_tool, ev.triggered_by, ev.face,
		       ev.origin_kind, ev.origin_source, ev.origin_source_face,
		       ev.origin_source_type, ev.origin_definition,
		       ev.created_at, ev.entity_id, min(lin.hi)
		FROM entity_versions ev` + lineageJoin + `
		GROUP BY ev.vseq
		ORDER BY ev.vseq ASC`

	args := append(lineageArgs(id, p), string(p))
	rows, err := q.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list versions for %s: %w", id, err)
	}
	defer rows.Close()

	var out []storeutil.LineageRow
	for rows.Next() {
		r, err := scanLineageRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Ordinals are assigned at READ time over the fenced lineage, never
	// stored: storing them would need an app-side max+1 that races.
	return storeutil.FinishLineage(out), nil
}

// readTags returns the tag rows on any of vseqs.
func readTags(ctx context.Context, q querier, vseqs []int64) ([]storeutil.TagRow, error) {
	if len(vseqs) == 0 {
		return nil, nil
	}
	ph, args := idPlaceholders(vseqs)
	rows, err := q.QueryContext(ctx,
		`SELECT vseq, name FROM version_tags WHERE vseq IN (`+ph+`) ORDER BY vseq, name`, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: read version tags: %w", err)
	}
	defer rows.Close()
	var out []storeutil.TagRow
	for rows.Next() {
		var t storeutil.TagRow
		if err := rows.Scan(&t.Vseq, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetVersion implements [store.HistoryReader].
//
// version is a 1-based ordinal over the face's fenced lineage ordered by vseq,
// so version 1 of draft and version 1 of published are different snapshots.
// The ordinal is only meaningful relative to a ListVersions read taken at the
// same time — the lineage is append-only, so an ordinal a caller already holds
// stays valid, but callers should treat it as a cursor into a specific list
// result.
func (v *VersionStore) GetVersion(
	ctx context.Context, ref entity.Ref, version int,
) (*store.VersionSnapshot, error) {
	id, p := ref.ID, ref.Face
	if version < 1 {
		return nil, store.ErrNotFound
	}
	sel := lineageCTE + `
		SELECT ev.op, ev.prev_id, ev.type, ev.content_hash, ev.schema_hash,
		       ev.principal_user, ev.principal_tool, ev.triggered_by, ev.face,
		       ev.origin_kind, ev.origin_source, ev.origin_source_face,
		       ev.origin_source_type, ev.origin_definition,
		       ev.created_at, ev.content, ev.properties, sv.projection
		FROM entity_versions ev` + lineageJoin + `
		JOIN schema_versions sv ON sv.hash = ev.schema_hash
		GROUP BY ev.vseq
		ORDER BY ev.vseq ASC
		LIMIT 1 OFFSET ?`
	// GROUP BY ev.vseq rather than the SELECT DISTINCT lineageJoin's doc asks
	// for, and the difference is not stylistic. vseq is NOT in this projection
	// (the caller gets a snapshot, not a row id), so DISTINCT would dedup on
	// the projected columns instead — which is a different question, and the
	// wrong one: two genuinely distinct versions holding identical content
	// would collapse into one and shift every later version's ordinal.
	//
	// Grouping on vseq dedups the rename diamond by ROW IDENTITY, which is
	// what the doc means. vseq is the primary key, so each group holds exactly
	// one row and the bare columns are unambiguous.

	args := append(lineageArgs(id, p), string(p), version-1)
	row := v.db.QueryRowContext(ctx, sel, args...)

	var (
		snap    store.VersionSnapshot
		op      string
		prev    *string
		face    string
		props   string
		oc      originCols
		created string
	)
	scanArgs := make([]any, 0, 13+originColumnCount)
	scanArgs = append(scanArgs, &op, &prev, &snap.Type, &snap.ContentHash, &snap.SchemaHash,
		&snap.PrincipalUser, &snap.PrincipalTool, &snap.TriggeredBy, &face)
	scanArgs = append(scanArgs, oc.scanTargets()...)
	scanArgs = append(scanArgs, &created, &snap.Content, &props, &snap.Projection)

	err := row.Scan(scanArgs...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: get version %d of %s: %w", version, id, err)
	}

	snap.Version = version
	snap.Face = entity.Face(face)
	snap.Op = store.VersionOp(op)
	snap.Origin = scanOrigin(oc)
	if prev != nil {
		snap.PrevID = *prev
	}
	if snap.CreatedAt, err = parseTime(created); err != nil {
		return nil, fmt.Errorf("sqlitestore: parse version created_at for %s: %w", id, err)
	}
	if snap.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return &snap, nil
}

// scanLineageRow scans one row of listLineage: the version metadata, then the
// row's entity id and its segment's upper fence (NULL for the head).
func scanLineageRow(row scanner) (storeutil.LineageRow, error) {
	var (
		r       storeutil.LineageRow
		op      string
		prev    *string
		face    string
		oc      originCols
		created string
		hi      *int64
	)
	m := &r.Meta
	scanArgs := make([]any, 0, 13+originColumnCount)
	scanArgs = append(scanArgs, &r.Vseq, &op, &prev, &m.Type, &m.ContentHash, &m.SchemaHash,
		&m.PrincipalUser, &m.PrincipalTool, &m.TriggeredBy, &face)
	scanArgs = append(scanArgs, oc.scanTargets()...)
	scanArgs = append(scanArgs, &created, &r.EntityID, &hi)
	if err := row.Scan(scanArgs...); err != nil {
		return storeutil.LineageRow{}, err
	}
	m.Op = store.VersionOp(op)
	m.Face = entity.Face(face)
	m.Origin = scanOrigin(oc)
	if prev != nil {
		m.PrevID = *prev
	}
	if hi != nil {
		r.Hi = *hi
	}
	var err error
	if m.CreatedAt, err = parseTime(created); err != nil {
		return storeutil.LineageRow{}, fmt.Errorf("sqlitestore: parse version created_at: %w", err)
	}
	return r, nil
}
