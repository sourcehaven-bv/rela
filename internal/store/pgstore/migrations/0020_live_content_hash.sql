-- pgstore schema, version 20: the content hash on live rows (BUG-1DWMYO).
--
-- The version sweep must select only rows whose content differs from their
-- latest version. Its dedup compares canonical content hashes, which SQL
-- cannot compute, so the sweep used to select every settled row and skip the
-- unchanged ones in Go. Once more than one batch of settled, unchanged rows
-- existed, the same rows filled every tick and newer edits were never
-- captured.
--
-- content_hash holds the canonical hash of the row as the sweep last computed
-- it. NULL means "not known": every existing row, every new row, and every row
-- whose hashed columns changed since. The sweep selects rows whose hash is
-- NULL or differs from their latest version's, computes the hash, captures if
-- needed, and writes the hash back. The candidate query then compares exactly
-- what the dedup compares.
--
-- Writers never set the column. A trigger clears it whenever a hashed column
-- changes, so a write path that forgets about it, or a process still running
-- the previous binary, cannot leave a stale hash behind. The trigger fires only
-- on a real change; the sweep's write-back sets content_hash alone, so it does
-- not fire on that either. properties is compared as jsonb, so a change such
-- as 1 to 1.0 keeps the hash; the canonical hash folds those alike.
--
-- ADD COLUMN without a default is a catalog-only change; no rewrite.

ALTER TABLE entities  ADD COLUMN content_hash TEXT;
ALTER TABLE relations ADD COLUMN content_hash TEXT;

CREATE FUNCTION rela_clear_content_hash() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.content_hash := NULL;
    RETURN NEW;
END
$$;

CREATE TRIGGER entities_clear_content_hash
    BEFORE UPDATE OF id, face, type, properties, content ON entities
    FOR EACH ROW
    WHEN ((OLD.id, OLD.face, OLD.type, OLD.properties, OLD.content)
          IS DISTINCT FROM (NEW.id, NEW.face, NEW.type, NEW.properties, NEW.content))
    EXECUTE FUNCTION rela_clear_content_hash();

CREATE TRIGGER relations_clear_content_hash
    BEFORE UPDATE OF from_id, from_face, rel_type, to_id, properties, content ON relations
    FOR EACH ROW
    WHEN ((OLD.from_id, OLD.from_face, OLD.rel_type, OLD.to_id, OLD.properties, OLD.content)
          IS DISTINCT FROM (NEW.from_id, NEW.from_face, NEW.rel_type, NEW.to_id, NEW.properties, NEW.content))
    EXECUTE FUNCTION rela_clear_content_hash();
