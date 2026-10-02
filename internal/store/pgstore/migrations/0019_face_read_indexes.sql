-- pgstore schema, version 18: read indexes that serve every face (TKT-KQXVF7).
--
-- 0014 built both of its read indexes as partials on the implicit face
-- (WHERE face = ''). That fit a store with one row per id. Since content
-- faces (TKT-DOFYR1) a family can hold several rows, and neither partial
-- serves a faced type.
--
-- entities_type_id_face_idx replaces entities_type_id_idx. A type page is
-- ordered by (id, face) in every shape: the implicit-face page, the
-- all-faces page, an explicit face set, and a world, whose DISTINCT ON (id)
-- needs id order. (type, id, face) serves each of them as an index range
-- scan. The implicit-face page filters face inside the index, and a faced
-- type has only a few faces, so that costs at most a few extra index
-- entries per row. The partial served only the implicit-face page, so a
-- faced type's page sorted the whole type.
--
-- entities_type_idx (type), from 0001, is dropped: (type, id, face) and
-- 0010's (type, seq) both lead with type and serve every lookup it did. Kept,
-- it cost a write per row, and the planner preferred it for a world page
-- over the index that also gives id order.
--
-- entities_id_prefix_idx is dropped with no replacement. HighestID dropped
-- its face = '' filter in BUG-HC6I2T (a faced type must be counted), so the
-- partial no longer matched and every create scanned the table. HighestID
-- now sends a range, id >= $1 AND id < $2, and entities.id is COLLATE "C"
-- (0001), so the primary key (id, face) is already in byte order and serves
-- the range, in a generic plan as well. A text_pattern_ops index would add a
-- write per row for nothing; measured, it also took family and single-row
-- reads away from the primary key.
--
-- Runs inside pgstore.Migrate's single transaction under its advisory lock,
-- so the drops and the create commit together and no window exists without
-- an index. CREATE INDEX CONCURRENTLY cannot run in a transaction, so the
-- build holds a SHARE lock on entities, which blocks writers but not
-- readers: seconds at 20k rows, as 0014 did. The create runs first because
-- each DROP INDEX takes ACCESS EXCLUSIVE, which blocks readers too, until
-- commit; dropping last keeps that window to the end of the transaction.
-- IF [NOT] EXISTS keeps a re-run safe.

CREATE INDEX IF NOT EXISTS entities_type_id_face_idx ON entities (type, id, face);

DROP INDEX IF EXISTS entities_type_id_idx;
DROP INDEX IF EXISTS entities_type_idx;
DROP INDEX IF EXISTS entities_id_prefix_idx;
