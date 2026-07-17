-- pgstore schema, version 4: content versioning (TKT-9INY0Y).
--
-- Adds time-machine history for entity content: entity_versions holds a full
-- snapshot per captured version, schema_versions holds the content-addressed
-- render-schema projection each snapshot was taken under (so a historical
-- version renders faithfully against the schema it was created under, not
-- today's). Version rows are NOT cascade-linked to entities: an entity's
-- history survives its deletion (a compliance/audit requirement).
--
-- Capture is HYBRID (see internal/store/pgstore/sweep.go and the entitymanager
-- version hook): rename and delete are captured synchronously at the write
-- choke-point (they carry information a later snapshot can't reconstruct — the
-- old->new id, and the pre-delete state); create/update are captured by a
-- periodic reconciliation sweep that debounces bursts.

-- version_seq is a DEDICATED monotonic sequence for ordering version rows. It is
-- deliberately NOT the shared rela_seq: rela_seq feeds the change-feed watermark
-- (listener.go primeWatermark/catchUp scans entities/relations/deletions), and
-- burning rela_seq values that never land in those tables would erode the
-- watermark's overlap budget and could drop real change-feed events. version_seq
-- is independent and the watermark never touches it.
CREATE SEQUENCE IF NOT EXISTS version_seq;

-- schema_versions is the content-addressed store of render-schema projections.
-- hash is the SHA-256 of the metamodel's render-relevant projection
-- (metamodel.RenderProjection().Hash()); projection is that projection as JSON.
-- Deduplicated by hash: an unchanged (render-relevant) metamodel across many
-- writes stores exactly one row. Never pruned — it is tiny after dedup and every
-- entity_versions row's schema_hash must stay resolvable.
CREATE TABLE schema_versions (
    hash        TEXT        COLLATE "C" PRIMARY KEY,
    projection  JSONB       NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- entity_versions holds one full snapshot per captured version.
--
-- Keyed by (entity_id, vseq). vseq is a global monotonic ordinal from
-- version_seq; the human-facing "version N" is a read-time row_number over vseq
-- within a lineage, not stored (so no app-side max+1 race). Lineage across a
-- rename is walked via op='rename' rows: such a row carries prev_id (the old id)
-- so "history of B including its life as A" is reconstructable at read time.
--
-- op is 'create' | 'update' | 'rename' | 'delete'. content/properties are the
-- full snapshot (properties as JSONB, matching the entities table). content_hash
-- is canonical.HashEntity of the snapshot, used to dedup no-op captures. The
-- principal_* / triggered_by columns carry attribution for synchronously
-- captured ops (rename/delete); sweep-captured create/update rows carry the
-- system principal (tool='version-sweep') — the editing principal for those is
-- recoverable from the audit log.
CREATE TABLE entity_versions (
    entity_id      TEXT        COLLATE "C" NOT NULL,
    vseq           BIGINT      NOT NULL DEFAULT nextval('version_seq'),
    op             TEXT        NOT NULL,
    prev_id        TEXT        COLLATE "C",
    type           TEXT        NOT NULL,
    content        TEXT        NOT NULL DEFAULT '',
    properties     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    content_hash   TEXT        NOT NULL,
    schema_hash    TEXT        COLLATE "C" NOT NULL REFERENCES schema_versions(hash),
    principal_user TEXT        NOT NULL DEFAULT '',
    principal_tool TEXT        NOT NULL DEFAULT '',
    triggered_by   TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_id, vseq)
);

-- Latest-version-per-entity probe: the sweep asks "does this entity's newest
-- version differ from its current content?" and reads answer 'list newest
-- first'. A descending (entity_id, vseq) index serves both as an index-only
-- LIMIT 1 probe.
CREATE INDEX entity_versions_latest_idx ON entity_versions (entity_id, vseq DESC);

-- Lineage walk: op='rename' rows are looked up by prev_id to stitch a renamed
-- entity's history back to its former id. Partial index — only rename rows carry
-- a prev_id, and they are rare.
CREATE INDEX entity_versions_prev_id_idx ON entity_versions (prev_id) WHERE prev_id IS NOT NULL;

-- The sweep's settle filter is "entities WHERE updated_at < now() - $idle".
-- Without this index that is a seqscan every tick. entities.updated_at was
-- unindexed until now.
--
-- LOCK NOTE: like 0003_sync.sql's seq indexes, this is a NON-CONCURRENT CREATE
-- INDEX run inside pgstore.Migrate's single advisory-locked transaction, so on
-- an already-large entities table it takes a SHARE lock that briefly blocks
-- writes for the duration of the build. Apply over a large dataset in a
-- maintenance window.
CREATE INDEX entities_updated_at_idx ON entities (updated_at);
