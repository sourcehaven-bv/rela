-- pgstore schema, version 50: the last change of a mutable record row
-- (TKT-ERO2X5G). updated_at is BIGINT microseconds since the epoch, 0 for a
-- row never changed; revision counts the changes, so a change can be made
-- conditional on the revision its caller read.

ALTER TABLE records ADD COLUMN IF NOT EXISTS updated_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE records ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT '';
ALTER TABLE records ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 0;
