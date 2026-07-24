-- pgstore schema, version 6: write-authorship columns (TKT-ZIRMGM).
--
-- last_edited_by_user / last_edited_by_tool record who/what performed the most
-- recent create/update write on the live row, stamped by the store from the
-- boundary-populated store.Attribution on ctx. The version sweep reads them to
-- attribute its debounced create/update snapshots to the REAL editor instead of
-- the system principal; NULL (legacy rows, writes with no attribution) keeps
-- the pre-existing fallback (principal_tool = 'version-sweep').
--
-- Deliberately nullable with NO DEFAULT: NULL is the "no recorded editor"
-- encoding the sweep's fallback keys on — the boundary must not translate a
-- wholly-unknown principal into stored values (RR-U964M0). A PARTIALLY unknown
-- principal is stored verbatim, though: a CLI write with an unset $USER
-- legitimately lands as user='unknown', tool='cli' (the tool is real
-- information). No backfill: existing rows self-heal on their next attributed
-- write.
--
-- RenameEntity's bulk relation re-key (UPDATE relations SET from_id/to_id ...)
-- deliberately does not touch these columns: a rename does not edit relation
-- content, so the last CONTENT editor remains the attributed author
-- (RR-U1RGSE).
ALTER TABLE entities
    ADD COLUMN last_edited_by_user TEXT,
    ADD COLUMN last_edited_by_tool TEXT;

ALTER TABLE relations
    ADD COLUMN last_edited_by_user TEXT,
    ADD COLUMN last_edited_by_tool TEXT;
