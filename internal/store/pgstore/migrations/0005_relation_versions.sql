-- pgstore schema, version 5: relation content versioning (TKT-92JL8P).
--
-- Extends the entity time-machine history (0004) to relations, which carry the
-- same rich content as entities (a JSONB property set + a markdown body). The
-- design mirrors entity_versions but must solve three things entities never
-- had to: relations have NO stable id (the composite (from_id, rel_type, to_id)
-- IS the key, and it MUTATES on endpoint rename), they are destroyed by a
-- store-level cascade below the entitymanager, and their key can collide on
-- rename. See internal/store/pgstore/relation_version.go and the ticket for the
-- full rationale.

-- relation_record_seq assigns the surrogate lineage id (rel_record_id). It is a
-- DEDICATED sequence, deliberately NOT rela_seq (which feeds the change-feed
-- watermark) and NOT version_seq (which orders version rows). A relation's
-- rel_record_id is stamped once at CreateRelation and carried verbatim through
-- endpoint-rename cascade UPDATEs, so it survives a rename; delete drops the row
-- and a re-created (from,type,to) gets a fresh id (no history merge).
CREATE SEQUENCE IF NOT EXISTS relation_record_seq;

-- rel_record_id is the stable lineage id, living ON the relations row. Putting
-- it here (rather than reconstructing it per sweep-tick from the composite key)
-- is what dissolves the sweep-vs-synchronous-capture allocation race and the
-- reused-id-merge / delete-recreate class of bugs: lineage is read straight off
-- the row. Backfilled for pre-existing rows via the DEFAULT.
ALTER TABLE relations
    ADD COLUMN rel_record_id BIGINT NOT NULL DEFAULT nextval('relation_record_seq');

-- relation_versions holds one full snapshot per captured relation version.
--
-- Keyed by (rel_record_id, vseq). vseq is a global monotonic ordinal from the
-- SHARED version_seq (relations and entities interleave in one ordering; the
-- change-feed watermark never touches version_seq). The human-facing "version
-- N" is a read-time row_number over vseq within a rel_record_id, not stored.
--
-- Lineage is TRIVIAL compared to entities: rel_record_id is a stable surrogate,
-- so a lineage is simply "all rows WHERE rel_record_id = $1" — no rename-lineage
-- CTE or vseq fencing is needed, because a fresh rel_record_id is minted on
-- delete+recreate and the SAME id is carried across a rename.
--
-- from_id/rel_type/to_id are the composite AS-OF this version (they change on a
-- rename row, whose prev_from/prev_to carry the pre-rename endpoints). NO
-- from_vseq/to_vseq columns: the endpoints' versions are resolved at READ time
-- (with the reader's ACL), never stored — storing them would leak a TO-side
-- oracle and be NULL for most rows (endpoint create is debounced). See ticket
-- RR-S4W5KI / RR-SDDYZO.
--
-- op is 'create' | 'update' | 'rename' | 'delete'. content_hash is
-- canonical.HashRelation of the snapshot (folds in the triple so two distinct
-- relations with identical content do not dedup against each other). Sweep-
-- captured create/update rows carry the system principal (tool='version-sweep');
-- synchronously-captured delete/rename rows carry the editing principal from ctx.
CREATE TABLE relation_versions (
    rel_record_id  BIGINT      NOT NULL,
    vseq           BIGINT      NOT NULL DEFAULT nextval('version_seq'),
    op             TEXT        NOT NULL,
    from_id        TEXT        COLLATE "C" NOT NULL,
    rel_type       TEXT        COLLATE "C" NOT NULL,
    to_id          TEXT        COLLATE "C" NOT NULL,
    prev_from      TEXT        COLLATE "C",
    prev_to        TEXT        COLLATE "C",
    content        TEXT        NOT NULL DEFAULT '',
    properties     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    content_hash   TEXT        NOT NULL,
    schema_hash    TEXT        COLLATE "C" NOT NULL REFERENCES schema_versions(hash),
    principal_user TEXT        NOT NULL DEFAULT '',
    principal_tool TEXT        NOT NULL DEFAULT '',
    triggered_by   TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (rel_record_id, vseq)
);

-- Latest-version-per-lineage probe: the sweep asks "does this relation's newest
-- version differ from its current content?" A descending (rel_record_id, vseq)
-- index serves as an index-only LIMIT 1 probe and orders a lineage read.
CREATE INDEX relation_versions_latest_idx ON relation_versions (rel_record_id, vseq DESC);

-- The sweep's settle filter is "relations WHERE updated_at < now() - $idle".
-- relations.updated_at was unindexed (0001 created only PK + from/to/type
-- indexes), so without this the sweep's relation candidate scan is a seqscan
-- every tick.
--
-- LOCK NOTE: like 0004's entities_updated_at_idx, this is a NON-CONCURRENT
-- CREATE INDEX inside pgstore.Migrate's single advisory-locked transaction; on
-- an already-large relations table it briefly blocks writes for the build.
CREATE INDEX relations_updated_at_idx ON relations (updated_at);
