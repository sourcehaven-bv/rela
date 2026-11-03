-- pgstore schema, version 37: principal mentions and replies (TKT-6RXAC0).
--
-- channel_mentions holds one addressed row per (channel, seq, target,
-- kind): a principal a message mentions (kind 'mention'), or the author of
-- the message a reply answers (kind 'reply'). The target is the principal
-- key: a person entity id, a job identity or a login. The message's author
-- and type are copied into the row, so channels.Store.ListAddressed reads
-- no message document. A row goes with its message and its channel: the
-- store deletes the rows before the message or channel, and the foreign
-- key cascades as a backstop.
--
-- channel_mentions_target_idx serves the newest-first scan of one target's
-- rows; channel_mentions_kind_idx serves that scan for one kind;
-- channel_mentions_cap_idx serves the per-target cap in one channel.
-- sqlitedb's channelMentionsDDL has the same shape.
--
-- No backfill: messages stored before this migration were written under no
-- mention rule, so they never notify.
--
-- The REFERENCES channels (id) constraint takes a SHARE ROW EXCLUSIVE lock
-- on channels for the length of the CREATE TABLE. It is brief, because the
-- new table is empty.
--
-- NEW FILE rather than an amendment to 0035: Migrate tracks only the
-- version number, so a deployment already past 35 would never see an edit.

CREATE TABLE IF NOT EXISTS channel_mentions (
    channel_id  BIGINT NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    seq         BIGINT NOT NULL,
    target      TEXT   COLLATE "C" NOT NULL,
    kind        TEXT   NOT NULL CHECK (kind IN ('mention', 'reply')),
    at          BIGINT NOT NULL,
    author_user TEXT   COLLATE "C" NOT NULL,
    author_tool TEXT   COLLATE "C" NOT NULL DEFAULT '',
    author_org  TEXT   COLLATE "C" NOT NULL DEFAULT '',
    author_type TEXT   COLLATE "C" NOT NULL DEFAULT '',
    msg_type    TEXT   NOT NULL,
    reply_to    BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (channel_id, seq, target, kind)
);

CREATE INDEX IF NOT EXISTS channel_mentions_target_idx
    ON channel_mentions (target, at DESC, channel_id DESC, seq DESC);

CREATE INDEX IF NOT EXISTS channel_mentions_kind_idx
    ON channel_mentions (target, kind, at DESC, channel_id DESC, seq DESC);

CREATE INDEX IF NOT EXISTS channel_mentions_cap_idx ON channel_mentions (channel_id, target, seq);
