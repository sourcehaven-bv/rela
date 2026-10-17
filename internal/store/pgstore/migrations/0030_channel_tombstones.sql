-- pgstore schema, version 30: channel retention (TKT-964SIS).
--
-- channel_tombstones holds what a tombstoned batch of channels recorded when
-- it was filed (channels.Filing): why it was filed, the earliest time it may
-- be purged, and whether the delete that filed it has committed yet. A batch
-- is the channel rows whose tomb column holds one stamp; this table adds one
-- row per batch. A batch filed before this migration has no row and reads
-- as unrecorded.
--
-- channels_type_state_idx serves ListByLifecycle, which the close and
-- archive sweeps call per channel type and state every minute.
--
-- NEW FILE rather than an amendment to 0026: Migrate tracks only the
-- version number, so a deployment already past 26 would never see an edit.
-- Renumber it, rather than leave a gap, if the develop stack lands another
-- migration first.

CREATE TABLE IF NOT EXISTS channel_tombstones (
    anchor_id   TEXT    COLLATE "C" NOT NULL,
    tomb        TEXT    COLLATE "C" NOT NULL,
    -- Empty for a batch of every face of the id.
    anchor_face TEXT    COLLATE "C" NOT NULL DEFAULT '',
    policy      TEXT    NOT NULL DEFAULT '',
    purge_after BIGINT  NOT NULL DEFAULT 0,
    -- 1 until the delete that filed the batch has committed.
    pending     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (anchor_id, tomb)
);

CREATE INDEX IF NOT EXISTS channels_type_state_idx ON channels (tomb, type, state);
