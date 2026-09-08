-- pgstore schema, version 13: write provenance (origin) for history.
--
-- History could say WHO wrote a row (0006's last_edited_by_*) and WHAT
-- happened to it (entity_versions.op), but not HOW the bytes got there. A
-- copy — the Step-4 copy kernel writing a face from another face — was
-- therefore indistinguishable in history from someone typing the same
-- content in by hand, and named no source.
--
-- These columns record the mechanism and its source. They are NOT a new
-- entity_versions.op value: a copy genuinely IS a create-or-update of the
-- target row, `op` answers that, and a sixth op would reclassify the write
-- for every client already switching on the five. A reader wants both facts
-- at once — "v3 · update · copied from POL-1@draft".
--
-- NEW FILE rather than an amendment to the unreleased 0011/0012: those two
-- are one ticket's content-states change, and this spans a different pair of
-- tables for a different reason. Amending them would make either file's
-- header a lie about what it does.
--
-- WHY COLUMNS ON `entities` AND NOT ONLY ON `entity_versions`:
-- create/update versions are captured by the DEBOUNCED SWEEP, minutes after
-- the write, from the live row — the sweep can reconstruct nothing the row
-- does not carry. This is precisely the problem 0006 already solved for
-- authorship, and the solution is deliberately the same shape: the write
-- boundary stamps the live row, the sweep copies the columns onto the
-- version it captures. Any other route (a side table keyed by id, a
-- ctx value the sweep cannot see, a queue) would be a second mechanism for
-- a solved problem.
--
-- NULLABLE WITH NO DEFAULT, like last_edited_by_*: NULL means "no recorded
-- provenance", which IS the encoding for a direct edit. There is
-- deliberately no literal 'manual' value — a default label would make NULL
-- ambiguous between "hand edit" and "written before this migration", and a
-- hand edit is already fully described by absent origin + present
-- last_edited_by_*. No backfill for the same reason.
--
-- THE COLUMNS DESCRIBE THE MOST RECENT WRITE, not the row's distant past:
-- every create/update stamps them, so an ordinary edit of a
-- previously-copied row writes NULL back and the row stops claiming to be a
-- copy. That is what makes a captured version's origin describe THAT
-- version.
-- origin_source_type exists so a READ-OUT path can gate origin_source. An
-- entity id is row-level secret (whether an entity exists is a genuine
-- secret), the ACL read probe is keyed by (type, id), and a cross-entity copy
-- names a source the reader may have no grant for. Without the type the wire
-- could not gate it and history would become an existence oracle. It is a
-- gating input, not display data.
ALTER TABLE entities
    ADD COLUMN origin_kind        TEXT,
    ADD COLUMN origin_source      TEXT COLLATE "C",
    ADD COLUMN origin_source_face TEXT COLLATE "C",
    ADD COLUMN origin_source_type TEXT,
    ADD COLUMN origin_definition  TEXT;

-- entity_versions carries the captured provenance. Written by BOTH capture
-- paths: the sweep copies the live row's columns (create/update), and a
-- synchronous capture carries it in store.VersionInput (the same division
-- of labor as attribution).
--
-- origin_source COLLATE "C" matches entities.id / entity_versions.entity_id:
-- it holds an entity id, and a future join or lineage walk against it must
-- not hit a 42P21 collation mismatch (the trap 0012 documents for face).
ALTER TABLE entity_versions
    ADD COLUMN origin_kind        TEXT,
    ADD COLUMN origin_source      TEXT COLLATE "C",
    ADD COLUMN origin_source_face TEXT COLLATE "C",
    ADD COLUMN origin_source_type TEXT,
    ADD COLUMN origin_definition  TEXT;
