-- pgstore schema, version 24: the backlink index (TKT-Q6AKUN).
--
-- mention_targets holds the entity ids a row's body mentions, as extracted by
-- internal/mentions in Go and written in the same statement as the body, so
-- every path that moves a row (rename, soft delete, restore, purge) carries it
-- with no extra code. NULL means "not indexed yet": rows from before this
-- migration, and rows written by an older binary during a rolling deploy.
-- The store's reindex pass fills them in (mentions.go); until then such a row
-- is simply not listed as a backlink.
--
-- mentions_hash is the hex sha256 of content as of the last indexing. It lets the trigger
-- below tell a writer that indexed the body from one that did not: a body
-- change that leaves the hash untouched came from a binary that predates this
-- migration, so the stale targets are cleared for the reindex pass.

ALTER TABLE entities ADD COLUMN mention_targets TEXT[] COLLATE "C";
ALTER TABLE entities ADD COLUMN mentions_hash TEXT;

CREATE INDEX entities_mention_targets_idx ON entities USING GIN (mention_targets);
-- The reindex pass's work queue. Small once caught up, so the pass is cheap to
-- run at every open and on a timer.
CREATE INDEX entities_mentions_pending_idx ON entities (id, face) WHERE mention_targets IS NULL;

CREATE FUNCTION rela_mentions_invalidate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.content IS DISTINCT FROM OLD.content
       AND NEW.mentions_hash IS NOT DISTINCT FROM OLD.mentions_hash THEN
        NEW.mention_targets := NULL;
        NEW.mentions_hash := NULL;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER entities_mentions_invalidate
    BEFORE UPDATE OF content ON entities
    FOR EACH ROW EXECUTE FUNCTION rela_mentions_invalidate();
