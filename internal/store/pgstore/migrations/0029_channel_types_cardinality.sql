-- pgstore schema, version 28: the cardinality of each recorded channel type
-- (TKT-ZU7P13).
--
-- channel_types recorded only the anchor mode (0026). A channel type's
-- cardinality (one or many) also decides how its channels are keyed, so the
-- boot check needs it too.
--
-- NEW FILE rather than an amendment to 0026: 0026 shipped in a release, and
-- Migrate tracks only the version number, so a deployment already at 26 would
-- never see an edit to it.
--
-- Existing rows take '', which internal/channels reads as "recorded before
-- cardinality was kept": it matches any configured cardinality, and the next
-- RecordTypes for the type fills it in.

ALTER TABLE channel_types ADD COLUMN IF NOT EXISTS cardinality TEXT NOT NULL DEFAULT '';
