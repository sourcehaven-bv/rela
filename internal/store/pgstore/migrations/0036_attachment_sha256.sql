-- pgstore schema, version 36: the SHA-256 of each attachment (TKT-1TS55R).
--
-- sha256 is a stored generated column, so PostgreSQL computes it from the
-- bytes on every insert and update, including the upsert in
-- AttachFamilyFile and any writer added later. No writer can forget it, and
-- the rows already in the table get it when the column is added.
--
-- Adding a stored generated column rewrites the table once, while this
-- migration holds its lock. On a large attachments table that pause happens
-- at the first start after the upgrade. Generated columns need
-- PostgreSQL 12 or later.
--
-- sqlitedb's addAttachmentSHA256 gives SQLite the same column.

ALTER TABLE attachments
    ADD COLUMN IF NOT EXISTS sha256 TEXT GENERATED ALWAYS AS (encode(sha256(bytes), 'hex')) STORED;
