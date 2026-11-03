-- pgstore schema, version 33: channel session reminders (TKT-36Z6X3).
--
-- Each channel row carries its session reminder records
-- (channels.Session.Reminders) in session_reminders: per reminder name the
-- start of its span, the time of its last post and its count, as a JSON
-- object with times in microseconds ('' for none). A row written before
-- this migration gets the default: no reminder records.
--
-- No new index: reminders share session_next_due_at and
-- channels_session_due_idx with the interrupting timers of version 32.
--
-- NEW FILE rather than an amendment to 0032: Migrate tracks only the
-- version number, so a deployment already past 32 would never see an edit.
-- Migrate also skips any version at or below the current one, so the
-- numbering must stay contiguous when the stack is rebased onto develop.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_reminders TEXT NOT NULL DEFAULT '';
