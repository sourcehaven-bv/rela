package sqlitedb

// versionSchemaSQL is the content-versioning half of the schema (TKT-4NU9ZD),
// the SQLite analogue of pgstore migrations 0004 / 0005 / 0012 / 0013.
//
// Shared between [schemaSQL] (fresh databases) and the v3→v4 migration
// (existing ones), for the same reason projectFilesDDL and stateKVDDL are:
// two copies drift, and a fresh database ending up with a different shape from
// a migrated one is the failure the version stamp exists to prevent.
//
// It is CREATE ... IF NOT EXISTS throughout, so it re-runs harmlessly. The one
// statement that is NOT idempotent — adding rel_record_id to the existing
// relations table — lives in the migration step alone; a fresh database gets
// the column from the relations CREATE TABLE in schemaSQL instead.
//
// # Where this deliberately diverges from pgstore
//
// Three of pgstore's mechanisms have no SQLite counterpart, and each absence is
// a decision rather than an omission:
//
//   - No sequences. pgstore allocates vseq from a dedicated version_seq and
//     rel_record_id from relation_record_seq, both kept away from rela_seq so
//     version rows never burn change-feed watermark values. SQLite has no
//     sequence object, so vseq is an INTEGER PRIMARY KEY AUTOINCREMENT (see the
//     AUTOINCREMENT note on entity_versions — the keyword is load-bearing) and
//     rel_record_id is drawn from a counter table. The separation survives:
//     neither shares a counter with anything the change feed reads.
//
//   - No advisory lock, because there is nothing to coordinate. pgstore's sweep
//     takes pg_try_advisory_lock so that only one of several server processes
//     sharing a database sweeps at a time. [Open] holds an exclusive sidecar
//     lock and REFUSES a second process (see lock.go), so "one sweeper" is
//     guaranteed by construction. This is stated here because its absence is
//     otherwise indistinguishable from having forgotten it.
//
//   - No JSONB. Properties are JSON TEXT, matching how the entities and
//     relations tables already store them.
//
// # What is carried across unchanged, and must stay that way
//
// The lineage fencing. Entity history is walked by a recursive CTE over [lo,hi)
// vseq windows rather than a flat `entity_id = ?`, because ids can be renamed
// away and later reused — a flat read merges two unrelated entities' histories.
// Relations are fenced by the surrogate rel_record_id carried ON the relations
// row, so a delete+recreate of the same triple mints a fresh lineage instead of
// resurrecting the old one. Both were verified against SQLite before this was
// written.
const versionSchemaSQL = `
-- schema_versions is the content-addressed store of render-schema projections.
-- hash is metamodel.RenderProjection().Hash(); projection is that projection as
-- JSON. Deduplicated by hash, so an unchanged metamodel across many writes
-- stores exactly one row. Never pruned: it is tiny after dedup and every
-- version row's schema_hash must stay resolvable, including for versions whose
-- entity has since been deleted.
CREATE TABLE IF NOT EXISTS schema_versions (
	hash        TEXT NOT NULL PRIMARY KEY,
	projection  TEXT NOT NULL,
	captured_at TEXT NOT NULL
) STRICT;

-- entity_versions holds one full snapshot per captured version.
--
-- Keyed by (entity_id, face, vseq). The face joins the key because history is
-- per-face: interleaving a family's faces into one lineage is actively corrupt
-- history, and purge would then have to fence what should never have been
-- merged (pgstore learned this the expensive way — see its migration 0012).
--
-- The human-facing "version N" is a read-time row_number over vseq within a
-- lineage, never stored, so there is no app-side max+1 race.
--
-- AUTOINCREMENT IS LOAD-BEARING, not decoration. A plain INTEGER PRIMARY KEY
-- reuses the rowid of a deleted newest row, and the lineage fence compares vseq
-- values as a monotonic timeline: reuse lets a NEW row sort before an OLD one
-- and silently reorders or mis-fences history. AUTOINCREMENT costs one
-- sqlite_sequence row and buys monotonicity that never regresses.
--
-- Version rows are NOT foreign-keyed to entities: an entity's history must
-- survive its deletion, which is a compliance requirement, not an oversight.
--
-- op is 'create' | 'update' | 'rename' | 'delete'. content_hash is
-- canonical.HashEntity of the snapshot, used to dedup no-op captures. The
-- principal_* / triggered_by columns carry attribution for synchronously
-- captured ops (rename/delete); sweep-captured create/update rows carry the
-- system principal (tool='version-sweep'), and the editing principal for those
-- is recoverable from the audit log.
--
-- The origin_* columns encode store.Origin (pgstore migration 0013): all-NULL
-- is the zero Origin, i.e. a direct edit, so "not applicable" and "empty
-- string" never collide.
CREATE TABLE IF NOT EXISTS entity_versions (
	vseq               INTEGER PRIMARY KEY AUTOINCREMENT,
	entity_id          TEXT NOT NULL,
	face               TEXT NOT NULL DEFAULT '',
	op                 TEXT NOT NULL,
	prev_id            TEXT,
	type               TEXT NOT NULL,
	content            TEXT NOT NULL DEFAULT '',
	properties         TEXT NOT NULL DEFAULT '{}',
	content_hash       TEXT NOT NULL,
	schema_hash        TEXT NOT NULL REFERENCES schema_versions(hash),
	principal_user     TEXT NOT NULL DEFAULT '',
	principal_tool     TEXT NOT NULL DEFAULT '',
	triggered_by       TEXT NOT NULL DEFAULT '',
	origin_kind        TEXT,
	origin_source      TEXT,
	origin_source_face TEXT,
	origin_source_type TEXT,
	origin_definition  TEXT,
	created_at         TEXT NOT NULL
) STRICT;

-- Latest-version-per-face probe: the sweep asks "does this face's newest
-- version differ from its current content?" and reads want newest first. The
-- face sits between entity_id and vseq so the probe is a LIMIT 1 lookup rather
-- than a scan that discards every sibling face's rows.
CREATE INDEX IF NOT EXISTS entity_versions_latest_idx
	ON entity_versions (entity_id, face, vseq DESC);

-- Lineage walk: op='rename' rows are looked up by prev_id to stitch a renamed
-- entity's history back to its former id, scoped to the face so one face's
-- rename cannot set another face's fence. Partial index — only rename rows
-- carry a prev_id, and they are rare.
CREATE INDEX IF NOT EXISTS entity_versions_prev_id_idx
	ON entity_versions (prev_id, face) WHERE prev_id IS NOT NULL;

-- relation_versions holds one full snapshot per captured relation version.
--
-- Keyed by (rel_record_id, vseq), sharing entity_versions' vseq counter so
-- entities and relations interleave in one ordering.
--
-- Lineage is trivial compared to entities: rel_record_id is a stable surrogate
-- living on the relations row, so a lineage is simply "all rows WHERE
-- rel_record_id = ?" — no CTE and no vseq fencing, because a fresh id is minted
-- on delete+recreate and the SAME id is carried across a rename.
--
-- from_id/rel_type/to_id are the composite AS-OF this version; a rename row's
-- prev_from/prev_to carry the pre-rename endpoints. from_face is the
-- state-specific tail, mirroring the relations table — there is deliberately no
-- to_face, because heads stay entity-level. It is needed even though
-- rel_record_id already fences lineages: the rename stitch matches a
-- predecessor by the old triple, which cannot otherwise tell a state-tailed
-- edge from a default-tail one, and would merge them.
--
-- There are deliberately NO from_vseq/to_vseq columns. The endpoints' versions
-- are resolved at READ time under the reader's ACL; storing them would both
-- leak a TO-side oracle and be NULL for most rows, since endpoint creation is
-- debounced by the sweep.
CREATE TABLE IF NOT EXISTS relation_versions (
	vseq               INTEGER PRIMARY KEY AUTOINCREMENT,
	rel_record_id      INTEGER NOT NULL,
	op                 TEXT NOT NULL,
	from_id            TEXT NOT NULL,
	from_face          TEXT NOT NULL DEFAULT '',
	rel_type           TEXT NOT NULL,
	to_id              TEXT NOT NULL,
	prev_from          TEXT,
	prev_to            TEXT,
	content            TEXT NOT NULL DEFAULT '',
	properties         TEXT NOT NULL DEFAULT '{}',
	content_hash       TEXT NOT NULL,
	schema_hash        TEXT NOT NULL REFERENCES schema_versions(hash),
	principal_user     TEXT NOT NULL DEFAULT '',
	principal_tool     TEXT NOT NULL DEFAULT '',
	triggered_by       TEXT NOT NULL DEFAULT '',
	origin_kind        TEXT,
	origin_source      TEXT,
	origin_source_face TEXT,
	origin_source_type TEXT,
	origin_definition  TEXT,
	created_at         TEXT NOT NULL
) STRICT;

-- Latest-version-per-lineage probe, and the ordering for a lineage read.
CREATE INDEX IF NOT EXISTS relation_versions_latest_idx
	ON relation_versions (rel_record_id, vseq DESC);

-- The rename stitch and the key->lineage resolution both match by triple,
-- including the state tail.
CREATE INDEX IF NOT EXISTS relation_versions_triple_idx
	ON relation_versions (from_id, from_face, rel_type, to_id, vseq DESC);

CREATE INDEX IF NOT EXISTS relations_record_id_idx ON relations (rel_record_id);

-- The sweep's settle filter is "WHERE updated_at < ?", which is a full scan on
-- every tick without these.
CREATE INDEX IF NOT EXISTS entities_updated_at_idx  ON entities (updated_at);
CREATE INDEX IF NOT EXISTS relations_updated_at_idx ON relations (updated_at);

-- rel_record_seq is the rel_record_id allocator. A one-row counter table stands
-- in for pgstore's relation_record_seq: SQLite's AUTOINCREMENT only applies to a
-- rowid alias, and rel_record_id is a plain column on relations. Bumped inside
-- the caller's transaction, so allocation is serialized by the single-writer
-- lock the store already holds.
CREATE TABLE IF NOT EXISTS rel_record_seq (
	id   INTEGER PRIMARY KEY CHECK (id = 1),
	next INTEGER NOT NULL
) STRICT;
INSERT OR IGNORE INTO rel_record_seq (id, next) VALUES (1, 1);
`

// relRecordIDColumnDDL adds the surrogate relation-lineage id to an EXISTING
// relations table.
//
// Separate from [versionSchemaSQL] because ALTER TABLE ADD COLUMN is the one
// statement in the versioning schema that is not idempotent: it fails with
// "duplicate column name" on a re-run, which would break the crash-recovery
// property every migration step is supposed to have. A fresh database gets the
// column from the relations CREATE TABLE instead, so this runs on exactly one
// path.
//
// SQLite requires a CONSTANT default, so the column lands as 0 on every
// existing row and [backfillRelRecordIDs] assigns the real ids.
const relRecordIDColumnDDL = `
ALTER TABLE relations ADD COLUMN rel_record_id INTEGER NOT NULL DEFAULT 0;`
