-- pgstore schema, version 26: realtime channels (TKT-DD8B6I).
--
-- Backs internal/channels/pgchannels through internal/channels/sqlchannels,
-- whose package doc describes the four tables. Numbered 26 because 0024
-- (mention targets) and 0025 (leases) land before it on the develop stack,
-- which keeps the sequence gap-free there. Renumber it, rather than leave a
-- gap, if that order changes.
--
-- Rows live in the tenant's schema like every other table here.
--
-- # Not graph content
--
-- No foreign key to entities. The channel service owns its rename and delete
-- lifecycle through the entitymanager alias hook.
--
-- Times are BIGINT microseconds and JSON is TEXT, the same encoding the
-- SQLite tables use, so one implementation serves both. Ids are matched
-- byte-exactly, hence COLLATE "C" as on entities.id.

CREATE TABLE channels (
    id          BIGSERIAL PRIMARY KEY,
    anchor_id   TEXT   COLLATE "C" NOT NULL,
    anchor_face TEXT   COLLATE "C" NOT NULL DEFAULT '',
    type        TEXT   COLLATE "C" NOT NULL,
    name        TEXT   COLLATE "C" NOT NULL DEFAULT '',
    -- Empty for a live channel; the tombstone stamp of a detached one.
    tomb        TEXT   COLLATE "C" NOT NULL DEFAULT '',
    -- The anchor lifetime the channel was created in; see
    -- channels.Channel.Incarnation.
    incarnation TEXT   NOT NULL DEFAULT '',
    state       TEXT   NOT NULL,
    title       TEXT   NOT NULL DEFAULT '',
    created_by  TEXT   NOT NULL,
    created_at  BIGINT NOT NULL,
    closed_at   BIGINT NOT NULL DEFAULT 0,
    archived_at BIGINT NOT NULL DEFAULT 0,
    last_seq    BIGINT NOT NULL DEFAULT 0,
    rev         BIGINT NOT NULL DEFAULT 0,
    UNIQUE (anchor_id, anchor_face, type, name, tomb)
);

-- Serves the lifecycle sweeps.
CREATE INDEX channels_state_idx ON channels (tomb, state);

CREATE TABLE channel_messages (
    channel_id  BIGINT NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    seq         BIGINT NOT NULL,
    rev         BIGINT NOT NULL,
    reply_to    BIGINT NOT NULL DEFAULT 0,
    reply_count INTEGER NOT NULL DEFAULT 0,
    -- When the message's action went pending; 0 when none is pending.
    pending_at  BIGINT NOT NULL DEFAULT 0,
    doc         TEXT   NOT NULL,
    PRIMARY KEY (channel_id, seq)
);

-- Serves paging by rev, the cursor that sees edits and removals.
CREATE INDEX channel_messages_rev_idx ON channel_messages (channel_id, rev);

-- Serves the sweep of stale pending actions.
CREATE INDEX channel_messages_pending_idx ON channel_messages (pending_at) WHERE pending_at > 0;

CREATE TABLE channel_audit (
    id        BIGSERIAL PRIMARY KEY,
    anchor_id TEXT COLLATE "C" NOT NULL,
    tomb      TEXT COLLATE "C" NOT NULL DEFAULT '',
    record    TEXT NOT NULL
);

CREATE INDEX channel_audit_anchor_idx ON channel_audit (anchor_id, tomb, id);

-- The anchor mode (family or face) and cardinality (one or many) each
-- channel type was used with, so a restart can refuse a realtime.yaml that
-- changed either.
CREATE TABLE channel_types (
    type        TEXT COLLATE "C" PRIMARY KEY,
    anchor_mode TEXT NOT NULL,
    cardinality TEXT NOT NULL
);
