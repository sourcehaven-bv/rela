-- pgstore schema, version 18: soft-delete side tables (the data-entry Undo).
--
-- A soft-deleted entity family and its incident relations are MOVED here from
-- entities/relations rather than flagged in place. Every read of the live
-- tables then stays correct without a new predicate, and restore moves the
-- rows back unchanged.
--
-- Each row keeps the whole live row as jsonb (to_jsonb of the entities or
-- relations row), restored with jsonb_populate_record. A column added to a
-- live table later is therefore carried across without touching this table.
-- Only the identity columns are kept alongside, for keys and lookups.

CREATE TABLE marked_entities (
    id         TEXT        COLLATE "C" NOT NULL,
    face       TEXT        COLLATE "C" NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_by TEXT        NOT NULL DEFAULT '',
    row        JSONB       NOT NULL,
    PRIMARY KEY (id, face)
);
-- The id stays held while marked, case-folded like entities_id_lower_key.
CREATE INDEX marked_entities_id_lower_idx ON marked_entities (lower(id));

CREATE TABLE marked_relations (
    -- owner_id is the marked entity this edge comes back with. An edge between
    -- two marked entities belongs to one of them at a time.
    owner_id  TEXT COLLATE "C" NOT NULL,
    from_id   TEXT COLLATE "C" NOT NULL,
    from_face TEXT COLLATE "C" NOT NULL DEFAULT '',
    rel_type  TEXT COLLATE "C" NOT NULL,
    to_id     TEXT COLLATE "C" NOT NULL,
    row       JSONB NOT NULL,
    PRIMARY KEY (from_id, from_face, rel_type, to_id)
);
CREATE INDEX marked_relations_owner_idx ON marked_relations (owner_id);
CREATE INDEX marked_relations_to_idx    ON marked_relations (to_id);
