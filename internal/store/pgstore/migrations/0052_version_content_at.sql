-- pgstore schema, version 52: the edit time of each version (TKT-6KLUT3U).
--
-- created_at is when a version was CAPTURED. The sweep captures a create or
-- update once the row has settled, so created_at trails the edit by the idle
-- window, and by much longer when no sweep ran (a CLI edit while no server is
-- up). Point-in-time reads need the time the content became current, so a
-- version records it here: the live row's updated_at for a swept or
-- captured-now version, the write time for a synchronous rename or delete.
--
-- Nullable with no backfill: a row from before this migration reads NULL, and
-- readers fall back to created_at, which is the best that row can say.
ALTER TABLE entity_versions ADD COLUMN IF NOT EXISTS content_at TIMESTAMPTZ;
ALTER TABLE relation_versions ADD COLUMN IF NOT EXISTS content_at TIMESTAMPTZ;
