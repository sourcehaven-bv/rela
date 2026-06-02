-- pgstore schema, version 1.
--
-- All rows carry created_at / updated_at and a global monotonic `seq`
-- (from the rela_seq sequence) so a future cross-process change feed
-- (LISTEN/NOTIFY with catch-up from a watermark) can be added without a
-- schema migration. Nothing reads `seq` yet in the single-writer build.
--
-- pg_trgm powers substring/fuzzy/wildcard search; tsvector + GIN powers
-- ranked full-text. Both indexes are maintained on entities only — the
-- search Backend contract indexes entities, not relations.

-- pg_trgm is installed into the public schema (it can only be installed once
-- per database). The application keeps public on its search_path so the
-- similarity() function and gin_trgm_ops operator class resolve regardless of
-- the active schema. WITH SCHEMA public is idempotent under IF NOT EXISTS.
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

-- rela_seq is the global monotonic change marker. INSERTs consume it via the
-- `seq` column DEFAULT below; UPDATE/rename paths bump it explicitly with
-- nextval('rela_seq') so every mutation advances the watermark.
CREATE SEQUENCE IF NOT EXISTS rela_seq;

CREATE TABLE entities (
    id         TEXT PRIMARY KEY,
    type       TEXT        NOT NULL,
    properties JSONB       NOT NULL DEFAULT '{}'::jsonb,
    content    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    seq        BIGINT      NOT NULL DEFAULT nextval('rela_seq')
);

CREATE INDEX entities_type_idx ON entities (type);

-- search_text is the concatenation the search Backend matches against:
-- id + title + stringified properties + content. Maintained by the
-- application on write (kept in a column so both tsvector and trgm
-- indexes can be expression-free and the matched text is explicit).
ALTER TABLE entities ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

CREATE INDEX entities_search_tsv_idx
    ON entities USING GIN (to_tsvector('simple', search_text));

CREATE INDEX entities_search_trgm_idx
    ON entities USING GIN (search_text gin_trgm_ops);

CREATE TABLE relations (
    from_id    TEXT        NOT NULL,
    rel_type   TEXT        NOT NULL,
    to_id      TEXT        NOT NULL,
    properties JSONB       NOT NULL DEFAULT '{}'::jsonb,
    content    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    seq        BIGINT      NOT NULL DEFAULT nextval('rela_seq'),
    PRIMARY KEY (from_id, rel_type, to_id)
);

CREATE INDEX relations_from_idx ON relations (from_id);
CREATE INDEX relations_to_idx   ON relations (to_id);
CREATE INDEX relations_type_idx ON relations (rel_type);

CREATE TABLE attachments (
    entity_id    TEXT        NOT NULL,
    property     TEXT        NOT NULL,
    file_name    TEXT        NOT NULL DEFAULT '',
    content_type TEXT        NOT NULL DEFAULT '',
    bytes        BYTEA       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    seq          BIGINT      NOT NULL DEFAULT nextval('rela_seq'),
    PRIMARY KEY (entity_id, property)
);
