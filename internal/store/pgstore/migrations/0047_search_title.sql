-- pgstore schema, version 47: an indexed title for search ranking
-- (TKT-M58EQ1W).
--
-- Free-text search orders its hits by trigram distance between the needle and
-- the hit's title. Computed per row, that ranking read every match before the
-- LIMIT, so a common word cost a scan of most of the table. search_title holds
-- the lowercased title, or the lowercased id for a type without one or with an
-- empty one, and a GiST trigram index on it lets PostgreSQL read rows in
-- distance order and stop at the limit.
--
-- Which property is a type's title is configuration (the metamodel), so the
-- store records it in search_title_props at open (searchtitle.go) and the
-- trigger below computes the column from it on every write. Doing it in the
-- database covers every statement that writes a row, including rename,
-- convert, soft-delete restore and a binary older than this migration.
--
-- Existing rows get '' here and are filled in by the store's background pass
-- after open; until then they still match, and rank as if untitled. The pass
-- runs outside this migration on purpose: computing the column here would
-- rewrite every row while the migration holds its locks.

CREATE TABLE IF NOT EXISTS search_title_props (
    type TEXT COLLATE "C" PRIMARY KEY,
    prop TEXT NOT NULL
);

-- Types whose title property changed since their rows were last computed.
-- The background pass recomputes their rows and then deletes the entry if gen
-- is unchanged; a later change bumps gen, so its requeue is not lost.
CREATE TABLE IF NOT EXISTS search_title_pending (
    type TEXT COLLATE "C" PRIMARY KEY,
    gen BIGINT NOT NULL DEFAULT 0
);

ALTER TABLE entities ADD COLUMN IF NOT EXISTS search_title TEXT NOT NULL DEFAULT '';

-- The trigger reads the title map of its own table's schema (TG_TABLE_SCHEMA),
-- not whatever the writer's search_path names, so an admin session, another
-- tool, or a schema that is renamed or restored under another name keeps
-- working (as in 0021). searchtitle.go's heal pass computes the same value; a
-- test holds the two together.
CREATE OR REPLACE FUNCTION rela_search_title_set() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    title_prop TEXT;
BEGIN
    EXECUTE format('SELECT prop FROM %I.search_title_props WHERE type = $1', TG_TABLE_SCHEMA)
      INTO title_prop USING NEW.type;
    NEW.search_title := lower(coalesce(nullif(NEW.properties ->> title_prop, ''), NEW.id));
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS entities_search_title ON entities;
CREATE TRIGGER entities_search_title
    BEFORE INSERT OR UPDATE OF id, type, properties ON entities
    FOR EACH ROW EXECUTE FUNCTION rela_search_title_set();

-- Every row holds '' when this runs, so the build is quick, but it locks
-- writers out for its duration like the rest of the migration.
CREATE INDEX IF NOT EXISTS entities_search_title_trgm_idx ON entities USING GIST (search_title gist_trgm_ops);

-- The background pass's queue for rows from before this migration. A computed
-- value is never '' (an empty title falls back to the id), so this index is
-- empty once the pass has run.
CREATE INDEX IF NOT EXISTS entities_search_title_unset_idx ON entities (id, face) WHERE search_title = '';
