-- pgstore schema, version 21: the sweep reads only rows with no stored hash
-- (TASK-Y73Y9 in Atlas; follow-up to BUG-1DWMYO).
--
-- 0020 let the version sweep select rows whose stored content_hash is NULL or
-- differs from their latest version's. The second half needs the latest
-- version of every row, so each tick probed the version index once per live
-- row: about 185 ms per tick at 50,000 entities with nothing to do.
--
-- This migration makes a stronger invariant hold instead: a non-NULL
-- content_hash means the latest version of the row's current lifecycle has
-- that hash. The sweep then selects only rows whose hash is NULL, which the
-- partial indexes below serve, so a tick costs in proportion to the rows that
-- changed.
--
-- Three things could break the invariant, and a trigger covers each:
--
--   * A version is inserted with a different hash, or is a delete, which starts
--     a new lifecycle. The rename, delete and purge paths write versions
--     outside the sweep.
--   * A version is deleted (purge), which can change which version is latest.
--   * A row is inserted carrying a hash. Soft-delete restore copies the whole
--     row back, while its lifecycle may have ended in a delete version.
--
-- The triggers only ever clear the hash. The worst a spurious clear does is
-- make the sweep look at the row once more.
--
-- The version triggers update the live table in their own schema
-- (TG_TABLE_SCHEMA), not whatever the session's search_path names, so a tenant
-- schema that is renamed or restored under another name keeps working.

CREATE TRIGGER entities_insert_clear_content_hash
    BEFORE INSERT ON entities
    FOR EACH ROW WHEN (NEW.content_hash IS NOT NULL)
    EXECUTE FUNCTION rela_clear_content_hash();

CREATE TRIGGER relations_insert_clear_content_hash
    BEFORE INSERT ON relations
    FOR EACH ROW WHEN (NEW.content_hash IS NOT NULL)
    EXECUTE FUNCTION rela_clear_content_hash();

CREATE FUNCTION rela_entity_version_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        EXECUTE format('UPDATE %I.entities SET content_hash = NULL
                         WHERE id = $1 AND face = $2 AND content_hash IS NOT NULL', TG_TABLE_SCHEMA)
          USING OLD.entity_id, OLD.face;
        RETURN OLD;
    END IF;
    EXECUTE format('UPDATE %I.entities SET content_hash = NULL
                     WHERE id = $1 AND face = $2 AND content_hash IS NOT NULL
                       AND ($3 = ''delete'' OR content_hash IS DISTINCT FROM $4)', TG_TABLE_SCHEMA)
      USING NEW.entity_id, NEW.face, NEW.op, NEW.content_hash;
    RETURN NEW;
END
$$;

CREATE TRIGGER entity_versions_clear_content_hash
    AFTER INSERT OR DELETE ON entity_versions
    FOR EACH ROW EXECUTE FUNCTION rela_entity_version_changed();

CREATE FUNCTION rela_relation_version_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        EXECUTE format('UPDATE %I.relations SET content_hash = NULL
                         WHERE rel_record_id = $1 AND content_hash IS NOT NULL', TG_TABLE_SCHEMA)
          USING OLD.rel_record_id;
        RETURN OLD;
    END IF;
    EXECUTE format('UPDATE %I.relations SET content_hash = NULL
                     WHERE rel_record_id = $1 AND content_hash IS NOT NULL
                       AND ($2 = ''delete'' OR content_hash IS DISTINCT FROM $3)', TG_TABLE_SCHEMA)
      USING NEW.rel_record_id, NEW.op, NEW.content_hash;
    RETURN NEW;
END
$$;

CREATE TRIGGER relation_versions_clear_content_hash
    AFTER INSERT OR DELETE ON relation_versions
    FOR EACH ROW EXECUTE FUNCTION rela_relation_version_changed();

-- The relation trigger finds the live row by its lineage id.
CREATE INDEX relations_rel_record_id_idx ON relations (rel_record_id);

-- Hashes stored under 0020 were held only to the weaker rule, so start over.
-- The sweep rehashes every row once. This runs before the indexes below
-- exist, so it does not maintain them row by row.
UPDATE entities  SET content_hash = NULL WHERE content_hash IS NOT NULL;
UPDATE relations SET content_hash = NULL WHERE content_hash IS NOT NULL;

-- The sweep's candidate scan: unhashed rows in settle order.
CREATE INDEX entities_unhashed_idx  ON entities  (updated_at) WHERE content_hash IS NULL;
CREATE INDEX relations_unhashed_idx ON relations (updated_at) WHERE content_hash IS NULL;

