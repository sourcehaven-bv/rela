-- pgstore schema, version 31: channel sessions (TKT-Y6NJ6U).
--
-- Each channel row carries its session snapshot (channels.Session): the
-- state the session is in, its generation (how often it moved) and when it
-- entered the state, in microseconds. A row written before this migration
-- gets the defaults: an empty state and generation 0, which read as an
-- unset session in the configured initial state.
--
-- No index: nothing queries channels by session state yet.
--
-- NEW FILE rather than an amendment to 0030: Migrate tracks only the
-- version number, so a deployment already past 30 would never see an edit.
-- Migrate also skips any version at or below the current one, so the
-- numbering must stay contiguous when the stack is rebased onto develop.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_state TEXT NOT NULL DEFAULT '';
ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_gen BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_entered_at BIGINT NOT NULL DEFAULT 0;
