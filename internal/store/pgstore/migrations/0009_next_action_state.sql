-- pgstore schema, version 9: next-action per-user state (TKT-CXD0A4).
--
-- Snoozes, muted sources and last-shown timestamps for the advisory
-- next-action layer. Three tables rather than one JSON document (the shape the
-- filesystem backend uses) because this backend exists specifically for the
-- MULTI-PROCESS deployment: a document would make every write a
-- read-modify-write of the whole thing, so two servers would silently clobber
-- each other's snoozes. Row-level upserts make concurrent writers safe without
-- coordination.
--
-- NOT graph content, deliberately. A snooze is a fact about one person's
-- relationship to a suggestion at a moment, not about the entity — storing it
-- as an entity would make it visible to everyone, audited forever in the
-- append-only log, and (worse) fed through the version-capture sweep on every
-- render. These tables are therefore outside the entity/relation model, carry
-- no versioning, and are never touched by the sweep.
--
-- The state is DISPOSABLE: losing it costs a user a repeated suggestion, not
-- data. That is why there is no foreign key to entities and no cascade —
-- deleting an entity leaves an orphaned snooze row that simply never matches
-- again, and Prune reclaims it. An FK would make an ordinary entity delete
-- fail on a stale suggestion record, which is a far worse trade.

-- One row per (user, suggestion). The suggestion key is
-- (source, entity_id, variant); entity_id is '' for a count-based source with
-- no entity, and variant is '' when the source declares no key_props.
--
-- Empty string rather than NULL for the optional parts so the primary key
-- stays simple: NULLs are not comparable, so a NULL entity_id would let the
-- same logical key be inserted repeatedly.
CREATE TABLE next_action_snoozes (
    user_id    TEXT        NOT NULL,
    source     TEXT        NOT NULL,
    entity_id  TEXT        NOT NULL DEFAULT '',
    variant    TEXT        NOT NULL DEFAULT '',
    until      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, source, entity_id, variant)
);

-- Prune deletes by deadline across all users, so the index is on `until`
-- alone; per-user reads are already served by the primary key.
CREATE INDEX next_action_snoozes_until_idx ON next_action_snoozes (until);

-- One row per (user, suggestion) recording when it was last surfaced. Drives
-- cooldown. Separate from snoozes because the lifecycles differ: a snooze is
-- an explicit user choice with a deadline, a shown-record is telemetry that
-- Prune reclaims on age.
CREATE TABLE next_action_shown (
    user_id   TEXT        NOT NULL,
    source    TEXT        NOT NULL,
    entity_id TEXT        NOT NULL DEFAULT '',
    variant   TEXT        NOT NULL DEFAULT '',
    shown_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, source, entity_id, variant)
);

CREATE INDEX next_action_shown_at_idx ON next_action_shown (shown_at);

-- Muted sources, per user. Keyed on the SOURCE, never an entity: a per-entity
-- mute is invisible state nobody can find later, whereas the handful of
-- configured sources make "what have I turned off?" a short, reversible list.
--
-- No expiry column: a mute is a standing choice, and Prune deliberately never
-- touches this table. A housekeeping pass that silently un-muted a source
-- would surface as suggestions the user switched off coming back.
CREATE TABLE next_action_mutes (
    user_id TEXT NOT NULL,
    source  TEXT NOT NULL,
    PRIMARY KEY (user_id, source)
);
