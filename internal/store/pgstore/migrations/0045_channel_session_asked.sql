-- pgstore schema, version 45: asked questions of a channel's session span
-- (TKT-GR1MXL5).
--
-- Each channel row carries in session_asked the questions of the session's
-- current span (channels.Channel.Asked) as a JSON object keyed by seq, or
-- '' for none. A row written before this migration gets the default '' and
-- reads as no questions; there is no backfill.
--
-- NEW FILE rather than an amendment to an earlier version: Migrate tracks
-- only the version number, so a deployment already past it would never see
-- an edit.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_asked TEXT NOT NULL DEFAULT '';
