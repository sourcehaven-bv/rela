-- pgstore schema, version 12: per-state content versioning (TKT-C1XUA8).
--
-- Step 1 (TKT-DOFYR1) captured the DEFAULT face only, in four deliberate
-- skips: the two version hooks and the two sweep candidate scans. The
-- reason each gave was the same — entity_versions keys (entity_id, vseq),
-- so capturing a state row would interleave a family's faces in ONE
-- lineage, which is actively corrupt history that purge would then have to
-- fence. Per-state history was to be designed WITH its consumer.
--
-- This is that consumer's migration. The Step-4 copy kernel is the first
-- thing in the system that writes a non-default face, so without per-state
-- history a promoted face would have NO history at all — worse than the
-- skip, which is at least honest about its scope.
--
-- Every existing row is already its own default state, so '' NOT NULL
-- rewrites no data — the same property that made 0011 free, and the same
-- one-convention-everywhere rule: Go zero value, omitted frontmatter key,
-- '' column. COLLATE "C" matches entity_versions.entity_id; the lineage
-- CTE's two recursive terms must agree on collation or Postgres raises
-- 42P21, and the pointer now participates in that walk.

-- entity_versions: the face joins the lineage key.
ALTER TABLE entity_versions ADD COLUMN pointer TEXT COLLATE "C" NOT NULL DEFAULT '';
ALTER TABLE entity_versions DROP CONSTRAINT entity_versions_pkey;
ALTER TABLE entity_versions ADD PRIMARY KEY (entity_id, pointer, vseq);

-- relation_versions: the state-specific TAIL joins it, mirroring the
-- relations table (§2.3 — heads stay entity-level, so there is deliberately
-- no to_pointer here either).
--
-- Relations need this even though rel_record_id already fences lineages
-- per row: the rename STITCH matches a predecessor by the old triple
-- (prev_from, rel_type, prev_to), which cannot tell a state-tailed edge
-- from a default-tail one. Without the column, a state edge's stitch would
-- merge it with the default face's lineage — the exact interleaving the
-- entity skip existed to prevent, arriving through the relation door.
ALTER TABLE relation_versions ADD COLUMN from_pointer TEXT COLLATE "C" NOT NULL DEFAULT '';

-- The sweep's per-entity probes are index-only LIMIT 1 lookups (see
-- 0004_versions.sql). Once state rows are in the table an (entity_id, vseq)
-- index no longer serves them: the probe asks for the latest version of ONE
-- FACE, and would have to scan and discard every sibling face's rows.
-- Rebuilt rather than added alongside, because the old shape is a strict
-- prefix of the new one and keeping both would pay double write cost for a
-- lookup the new index already serves.
--
-- LOCK NOTE: a non-concurrent CREATE INDEX inside the migration's
-- transaction takes an ACCESS EXCLUSIVE lock on the table. Apply over a
-- large dataset in a maintenance window.
DROP INDEX entity_versions_latest_idx;
CREATE INDEX entity_versions_latest_idx
    ON entity_versions (entity_id, pointer, vseq DESC);

-- The lineage CTE resolves a rename boundary with
--   SELECT max(vseq) ... WHERE prev_id = $1 AND op = 'rename'
-- and that subselect must now be scoped to the face, or one face's rename
-- would set another face's fence. Widening the index keeps it a lookup
-- rather than a scan over every face that ever carried the id.
DROP INDEX entity_versions_prev_id_idx;
CREATE INDEX entity_versions_prev_id_idx
    ON entity_versions (prev_id, pointer) WHERE prev_id IS NOT NULL;

-- The relation rename stitch and the key->lineage resolution both match by
-- triple, and there is no index for that today (0005 indexed only
-- (rel_record_id, vseq DESC), which serves the per-lineage read but not the
-- by-triple lookup). Adding it now rather than later because the tail turns
-- those queries from "scan the triple's rows" into "scan every face's rows
-- for the triple" — a new index, not a rebuild.
CREATE INDEX relation_versions_triple_idx
    ON relation_versions (from_id, from_pointer, rel_type, to_id, vseq DESC);
