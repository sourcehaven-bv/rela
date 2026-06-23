-- Multi-attachment per property (TKT-WLLRO7).
--
-- A `file` property can now hold several attachments, each identified by its
-- (normalized) file name. The primary key moves from (entity_id, property) to
-- (entity_id, property, file_name), so a property is no longer limited to one
-- attachment. The file_name DEFAULT '' is dropped — an empty file name can no
-- longer be a valid key (the store layer rejects it via ValidateFileName).
--
-- Forward-only. The DDL itself is not re-runnable (DROP CONSTRAINT has no IF
-- EXISTS); it is applied exactly once by the migration runner, which tracks
-- schema_version and skips already-applied files. Existing single-attachment
-- rows already carry a real file_name and simply gain it as a key component.
-- (Any legacy row with an empty file_name would need a backfill, but the store
-- never wrote '' as a file name in practice — AttachFile has always received a
-- base name.)

ALTER TABLE attachments DROP CONSTRAINT attachments_pkey;
ALTER TABLE attachments ALTER COLUMN file_name DROP DEFAULT;
ALTER TABLE attachments ADD PRIMARY KEY (entity_id, property, file_name);
