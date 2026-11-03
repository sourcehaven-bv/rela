-- pgstore schema, version 40: channel session previous state (TKT-56TPOTE).
--
-- Each channel row carries in session_previous the state its session was
-- in before it entered the current one (channels.Session.Previous). A
-- re-entry keeps it. A row written before this migration gets the default
-- '' and keeps it until its next move to another state; there is no
-- backfill.
--
-- NEW FILE rather than an amendment to an earlier version: Migrate tracks
-- only the version number, so a deployment already past it would never see
-- an edit.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_previous TEXT NOT NULL DEFAULT '';
