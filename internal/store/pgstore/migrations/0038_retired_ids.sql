-- pgstore schema, version 38: the ids renames retired (TKT-J6GISC3).
--
-- retired_ids implements store.RetiredIDs. A retired id stays reserved, so no
-- new entity takes it, and points at the id that replaced it, so a mention
-- written under the old id still reaches the entity. successor is always the
-- entity's current id: a rename re-points the rows that named its old id,
-- including on a case-only rename. A successor id stays reserved too, even
-- after its entity is deleted, so no unrelated entity inherits the mentions.
-- retired_ids_id_lower_key folds case like entities_id_lower_key;
-- retired_ids_successor_idx serves the re-point and MentionSources;
-- retired_ids_successor_lower_idx serves the create and rename guards.
-- sqlitedb's retiredIDsDDL has the same shape.
--
-- No backfill: renames before this migration left no record of the old id.

CREATE TABLE IF NOT EXISTS retired_ids (
    id         TEXT COLLATE "C" PRIMARY KEY,
    successor  TEXT COLLATE "C" NOT NULL,
    retired_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS retired_ids_id_lower_key ON retired_ids (lower(id));

CREATE INDEX IF NOT EXISTS retired_ids_successor_idx ON retired_ids (successor);

CREATE INDEX IF NOT EXISTS retired_ids_successor_lower_idx ON retired_ids (lower(successor));
