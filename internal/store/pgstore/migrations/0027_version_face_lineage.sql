-- pgstore schema, version 27: allow conversion history to cross face names.
-- A conversion can rekey an entity family and map each old face to a target
-- face. The rename version records both coordinates so history can walk back
-- through that mapping without merging timelines that share an entity ID.
ALTER TABLE entity_versions ADD COLUMN prev_face TEXT COLLATE "C";
UPDATE entity_versions SET prev_face = face
    WHERE op = 'rename' AND prev_id IS NOT NULL;

DROP INDEX entity_versions_prev_id_idx;
CREATE INDEX entity_versions_prev_id_idx
    ON entity_versions (prev_id, (COALESCE(prev_face, face)))
    WHERE prev_id IS NOT NULL;
