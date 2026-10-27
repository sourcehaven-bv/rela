-- pgstore schema, version 35: read markers and unread counts (TKT-2B2AIA).
--
-- channel_reads holds one read marker per channel and reader: the reader's
-- user and organization (channels.Reader), the highest seq read, and when
-- the marker last moved, in microseconds. A marker goes with its channel:
-- the store deletes the rows before the channel, and the foreign key
-- cascades as a backstop.
--
-- channel_messages.counts is 1 for a message that counts toward a reader's
-- unread (channels.Counts) and 0 for a removed message or a session
-- notice. Rows written before this migration are filled from the doc. The
-- UPDATE prefilters with plain text, the prefixes encoding/json writes, so
-- the helper function and its jsonb cast run only on candidate rows. A doc
-- that is not valid JSON, or holds the escape \u0000 that jsonb refuses,
-- keeps 1: it counts, which shows one extra unread at worst and never hides
-- a message. The 'session:' literal is channels.SessionMarker
-- (TestChannelReadsMigrations_SessionMarker). Only rows still at 1 are
-- written, so a re-run changes nothing.
--
-- channels.last_at is the later of the channel's newest stored message
-- and its latest lifecycle time (channels.LastActivity); it orders the
-- read window (channels.Store.ListReads). Rows written before this
-- migration start at their latest lifecycle time; the next message or
-- lifecycle move raises it.
--
-- channel_messages_counts_idx makes an unread count a range of at most
-- channels.UnreadCap entries; channel_reads_reader_idx finds one reader's
-- markers. sqlitedb's channelReadsDDL and channelReadsIndexDDL have the
-- same shape.
--
-- The ALTER TABLE takes an ACCESS EXCLUSIVE lock on channel_messages and
-- holds it until the migration commits. The backfill reads every doc
-- under that lock, and the CREATE INDEX that follows also blocks writes,
-- so the lock time grows with the size of channel_messages. The strpos
-- prefilter only limits the jsonb casts, not the reads.
--
-- NEW FILE rather than an amendment to 0034: Migrate tracks only the
-- version number, so a deployment already past 34 would never see an edit.

CREATE TABLE IF NOT EXISTS channel_reads (
    channel_id  BIGINT NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    reader_user TEXT   COLLATE "C" NOT NULL,
    reader_org  TEXT   COLLATE "C" NOT NULL DEFAULT '',
    seq         BIGINT NOT NULL DEFAULT 0,
    at          BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (channel_id, reader_user, reader_org)
);

ALTER TABLE channel_messages ADD COLUMN IF NOT EXISTS counts INTEGER NOT NULL DEFAULT 1;

CREATE FUNCTION rela_message_counts(doc text) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    d jsonb;
BEGIN
    IF strpos(doc, '\u0000') > 0 THEN
        RETURN 1;
    END IF;
    BEGIN
        d := doc::jsonb;
    EXCEPTION WHEN others THEN
        RETURN 1;
    END;
    IF d ? 'removed' OR starts_with(COALESCE(d ->> 'triggered_by', ''), 'session:') THEN
        RETURN 0;
    END IF;
    RETURN 1;
END;
$$;

UPDATE channel_messages SET counts = 0
    WHERE counts = 1
    AND (strpos(doc, '"removed":') > 0 OR strpos(doc, '"triggered_by":"session:') > 0)
    AND rela_message_counts(doc) = 0;

DROP FUNCTION rela_message_counts(text);

ALTER TABLE channels ADD COLUMN IF NOT EXISTS last_at BIGINT NOT NULL DEFAULT 0;

UPDATE channels SET last_at = GREATEST(created_at, closed_at, archived_at, reopened_at) WHERE last_at = 0;

CREATE INDEX IF NOT EXISTS channel_messages_counts_idx ON channel_messages (channel_id, seq) WHERE counts = 1;

CREATE INDEX IF NOT EXISTS channel_reads_reader_idx ON channel_reads (reader_user, reader_org);
