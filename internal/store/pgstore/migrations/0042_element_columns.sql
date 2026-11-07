-- pgstore schema, version 42: the reconciliation element of every live row
-- (TKT-1G2XR1R, step 2 of FEAT-CMVZQFI).
--
-- Rela-to-rela sync compares ranges of rows by an additive fingerprint, summed
-- in SQL (internal/reconcile). Each live row therefore stores, from the write
-- that last changed it:
--
--   element_hash   the canonical content hash of the row (canonical.SumEntity
--                  or SumRelation, hex), as the store reads the row back;
--   element_fp0..3 the four 32-bit limbs of the row's element fingerprint
--                  (reconcile.Fingerprint.Limbs, most significant first).
--                  Each is non-negative and below 2^32. SUM over bigint
--                  returns numeric; for up to 2^31 rows it casts back to
--                  bigint, and reconcile.FingerprintFromLimbSums folds the
--                  four sums into the fingerprint.
--
-- This is not content_hash. content_hash means "the latest version of this
-- lifecycle has this hash" and the version sweep selects rows where it is NULL
-- (0020, 0021); setting it at write time would stop the sweep from capturing
-- creates and updates.
--
-- The address is not stored. It is a function of the key columns, so the
-- range index below is an expression index over it, and an unhashed row keeps
-- its address: a range can count its unhashed rows (count(*) against
-- count(element_hash)) instead of silently leaving them out. pgstore's
-- entityAddressSQL and relationAddressSQL hold the same expressions; a query
-- must spell them exactly as here to use the index. COLLATE "C" makes the
-- order bytewise. That equals Go's string order only because rela databases
-- use the UTF8 server encoding; under another encoding the byte order of
-- non-ASCII text differs.
--
-- The store writes the columns in the same statement as the content (create,
-- update) or fills them in the same transaction (rename, convert, endpoint
-- swap, restore). The trigger below is the safety net: a statement that
-- changes a hashed column without changing element_hash (raw SQL, or a
-- process still running the previous binary) clears the element, so a missed
-- path leaves NULL, never a stale value. A write whose canonical hash did not
-- change is cleared too; the store refills it in the same transaction. The
-- version sweep fills NULL rows in batches, which also drains every existing
-- row after this migration. That drain updates every row once. The indexed
-- element columns change, so the update is not HOT and adds an entry to every
-- index of the table. On a large tenant expect a burst of WAL and up to one
-- dead row version per row, spread over the sweep ticks.
--
-- Changing the canonical hash or the fingerprint encoding invalidates every
-- stored value: such a change needs a migration that sets these columns to
-- NULL.
--
-- The stored values describe the whole row, hidden fields included. They may
-- stand in for a reader's view hash only on rows where that reader has no
-- hidden field (see internal/reconcile).
--
-- Every statement can run again: tests roll schema_version back and migrate
-- twice.
--
-- ADD COLUMN without a default is a catalog-only change. Each CREATE INDEX
-- scans its table once and blocks writes to it while it runs.

ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS element_hash TEXT,
    ADD COLUMN IF NOT EXISTS element_fp0  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp1  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp2  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp3  BIGINT;

ALTER TABLE relations
    ADD COLUMN IF NOT EXISTS element_hash TEXT,
    ADD COLUMN IF NOT EXISTS element_fp0  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp1  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp2  BIGINT,
    ADD COLUMN IF NOT EXISTS element_fp3  BIGINT;

CREATE OR REPLACE FUNCTION rela_clear_element() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.element_hash := NULL;
    NEW.element_fp0 := NULL;
    NEW.element_fp1 := NULL;
    NEW.element_fp2 := NULL;
    NEW.element_fp3 := NULL;
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS entities_clear_element ON entities;
CREATE TRIGGER entities_clear_element
    BEFORE UPDATE OF id, face, type, properties, content ON entities
    FOR EACH ROW
    WHEN ((OLD.id, OLD.face, OLD.type, OLD.properties, OLD.content)
          IS DISTINCT FROM (NEW.id, NEW.face, NEW.type, NEW.properties, NEW.content)
          AND NEW.element_hash IS NOT DISTINCT FROM OLD.element_hash)
    EXECUTE FUNCTION rela_clear_element();

DROP TRIGGER IF EXISTS relations_clear_element ON relations;
CREATE TRIGGER relations_clear_element
    BEFORE UPDATE OF from_id, from_face, rel_type, to_id, properties, content ON relations
    FOR EACH ROW
    WHEN ((OLD.from_id, OLD.from_face, OLD.rel_type, OLD.to_id, OLD.properties, OLD.content)
          IS DISTINCT FROM (NEW.from_id, NEW.from_face, NEW.rel_type, NEW.to_id, NEW.properties, NEW.content)
          AND NEW.element_hash IS NOT DISTINCT FROM OLD.element_hash)
    EXECUTE FUNCTION rela_clear_element();

-- Range sums over the address: an index-only scan reads the limbs and the
-- hash without visiting the heap.
CREATE INDEX IF NOT EXISTS entities_element_idx ON entities
    ((type || '/' || id || CASE WHEN face = '' THEN '' ELSE '@' || face END) COLLATE "C")
    INCLUDE (element_fp0, element_fp1, element_fp2, element_fp3, element_hash);

CREATE INDEX IF NOT EXISTS relations_element_idx ON relations
    ((from_id || CASE WHEN from_face = '' THEN '' ELSE '@' || from_face END
      || '--' || rel_type || '--' || to_id) COLLATE "C")
    INCLUDE (element_fp0, element_fp1, element_fp2, element_fp3, element_hash);

-- The sweep's heal: unhashed rows in key order.
CREATE INDEX IF NOT EXISTS entities_elementless_idx  ON entities  (id, face) WHERE element_hash IS NULL;
CREATE INDEX IF NOT EXISTS relations_elementless_idx ON relations (from_id, from_face, rel_type, to_id)
    WHERE element_hash IS NULL;
