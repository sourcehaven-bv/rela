-- pgstore schema, version 45: the last reply time and the main-stream flag
-- of each channel message (TKT-ETL233O).
--
-- channel_messages.last_reply_at is the time of the newest reply to the
-- message, in microseconds, set in the step that raises reply_count. Rows
-- written before this migration start at 0; the next reply sets it.
--
-- channel_messages.in_stream is 1 for a message of the main stream
-- (channels.InStream): a top-level message, or a reply that answered a
-- choice or moved the session. Every reply written before this migration
-- leaves the stream except one that moved its session. A reply that
-- answered a choice cannot be told from its row and joins its thread.
-- The text test is a prefilter: a property may also be named
-- moved_session, so rela_moved_session reads the top-level member of the
-- doc. A doc that jsonb refuses counts as not moved. The step runs
-- once, before any reply is written with in_stream set by the store.
--
-- channel_messages_thread_idx reads the replies of one parent in seq
-- order; channel_messages_stream_idx reads the main stream. sqlitedb's
-- channelThreadsIndexDDL has the same shape.
--
-- The ALTER TABLE takes an ACCESS EXCLUSIVE lock on channel_messages until
-- the migration commits; the backfill reads the doc of every reply under it.

ALTER TABLE channel_messages ADD COLUMN IF NOT EXISTS last_reply_at BIGINT NOT NULL DEFAULT 0;

ALTER TABLE channel_messages ADD COLUMN IF NOT EXISTS in_stream INTEGER NOT NULL DEFAULT 1;

CREATE FUNCTION rela_moved_session(doc text) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE
    d jsonb;
BEGIN
    IF strpos(doc, '\u0000') > 0 THEN
        RETURN false;
    END IF;
    BEGIN
        d := doc::jsonb;
    EXCEPTION WHEN others THEN
        RETURN false;
    END;
    RETURN COALESCE(d -> 'moved_session' = 'true'::jsonb, false);
END;
$$;

UPDATE channel_messages SET in_stream = 0
    WHERE in_stream = 1 AND reply_to <> 0
    AND (strpos(doc, '"moved_session":true') = 0 OR NOT rela_moved_session(doc));

DROP FUNCTION rela_moved_session(text);

CREATE INDEX IF NOT EXISTS channel_messages_thread_idx ON channel_messages (channel_id, reply_to, seq)
    WHERE reply_to > 0;

CREATE INDEX IF NOT EXISTS channel_messages_stream_idx ON channel_messages (channel_id, seq) WHERE in_stream = 1;
