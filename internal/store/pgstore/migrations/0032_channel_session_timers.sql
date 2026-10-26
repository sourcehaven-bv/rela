-- pgstore schema, version 32: channel session timers (TKT-INM3AZ).
--
-- Each channel row carries its session timer schedule (channels.Session):
-- session_next_due_at, the earliest due time of the current state's
-- timers in microseconds (0 for none), and session_restarts, the restart
-- anchor of each timer that came due while the channel was closed or
-- tombstoned, as a JSON object of microseconds ('' for none). A row written
-- before this migration gets the defaults: no due time and no restart
-- anchors.
--
-- channels_session_due_idx serves channels.Store.ListSessionsDue: one range
-- per channel type, with the lifecycle state in the key so closed channels
-- are skipped by the scan. It is partial, so it holds only sessions with a
-- due time; every row has 0 when this migration runs, so it starts empty.
-- The sqlite channelTimersDDL has the same shape.
--
-- NEW FILE rather than an amendment to 0031: Migrate tracks only the
-- version number, so a deployment already past 31 would never see an edit.
-- Migrate also skips any version at or below the current one, so the
-- numbering must stay contiguous when the stack is rebased onto develop.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_next_due_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_restarts TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS channels_session_due_idx ON channels (type, state, session_next_due_at)
	WHERE session_next_due_at > 0 AND tomb = '';
