-- pgstore schema, version 38: channel session cause (TKT-V05706X).
--
-- Each channel row carries in session_cause the seq of the message whose
-- post moved its session into the current state (channels.Session.Cause):
-- the waiting message the inbox shows. A timer move stores 0. A row written
-- before this migration gets the default 0 and keeps it until its next
-- move; there is no backfill.
--
-- NEW FILE rather than an amendment to an earlier version: Migrate tracks
-- only the version number, so a deployment already past it would never see
-- an edit.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_cause BIGINT NOT NULL DEFAULT 0;
