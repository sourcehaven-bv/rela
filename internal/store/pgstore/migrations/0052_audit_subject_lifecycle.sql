-- pgstore schema, version 52: the face and lifecycle of each audit subject
-- (TKT-X1ZFO04).
--
-- face is the face of the entity the row names: the entity's own face, or a
-- relation's tail face. It is '' for a relation's head, an unfaced type and
-- rows written before this migration, and then the row belongs to every face.
-- starts is true when the record begins a lifecycle of that id and face: a
-- create, or a rename or conversion into it. A per-entity read starts at the
-- latest such row, so a reused id does not show its predecessor's records.

ALTER TABLE audit_subjects
	ADD COLUMN IF NOT EXISTS face TEXT COLLATE "C" NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS starts BOOLEAN NOT NULL DEFAULT false;

UPDATE audit_subjects s SET starts = true
FROM audit_log l
WHERE l.seq = s.seq AND NOT s.starts AND (
	(l.op = 'create-entity' AND l.doc::jsonb -> 'subject' ->> 'id' = s.entity_id)
	OR (l.op IN ('rename-entity', 'convert-entity') AND l.doc::jsonb -> 'after' ->> 'id' = s.entity_id)
);

CREATE INDEX IF NOT EXISTS audit_subjects_starts_idx ON audit_subjects (entity_id, face, seq) WHERE starts;
