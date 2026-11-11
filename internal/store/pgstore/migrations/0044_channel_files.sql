-- pgstore schema, version 43: the bytes of channel message files
-- (TKT-BYKOM3V).
--
-- One row per (channel, seq, file id). The entries live in the message's
-- doc; this table holds only the bytes, so a page of messages reads none of
-- it. Keyed by channel_id, so rekey and tombstone move the files with their
-- channel and the erasure of a channel takes them with it.

CREATE TABLE IF NOT EXISTS channel_files (
	channel_id BIGINT NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
	seq        BIGINT NOT NULL,
	file_id    TEXT COLLATE "C" NOT NULL,
	data       BYTEA  NOT NULL,
	PRIMARY KEY (channel_id, seq, file_id)
);
