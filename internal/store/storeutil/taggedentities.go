package storeutil

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// taggedEntitiesSQL answers [store.VersionTagIndex.TaggedEntities] in one
// statement (TKT-7LRUPB). {{name}} is the tag name and {{type}} the type
// filter (the empty string for every type); [TaggedEntitiesSQL] substitutes
// each backend's bind syntax. Shared by pgstore and sqlitestore so the lineage rules are written
// once.
//
//   - tagged: the version rows that carry the tag.
//   - fwd: from each tagged row, the next rename that moved its (id, face)
//     away, hop by hop. A delete of the (id, face) before that rename ends the
//     lifecycle, so the walk stops there, as [CurrentLifecycle] ends a
//     lifecycle at a delete.
//   - head: the last hop, the face the lineage lives on now (or ended on),
//     picked with row_number so the step stays linear in the hops.
//   - bwd and ord: the backward lineage walk the backends' ListVersions uses,
//     seeded at the head, so the count of its rows up to the tagged row is
//     the ordinal ListVersions gives that row. The head's lineage includes the
//     rows of an earlier occupant of a reused id, as ListVersions does.
//     count(DISTINCT) dedups a rename diamond. The seed's upper fence is the
//     largest bigint, standing for the head's unbounded segment in both
//     dialects. When the tagged row is not in the head's lineage (a deleted
//     id that a later occupant renamed away), the ordinal is 0.
//
// A head is deleted when a delete row follows the last hop, or when it has no
// live row (a soft delete moves the row out of entities without a version).
// A deleted row reports the type of the tagged version, which is the type the
// caller baselined; a live row reports its live type, which the read gate
// checks. The final tvseq key orders two tags of one name on one face (a tag
// on an ended lifecycle and one on the current lifecycle) deterministically.
const taggedEntitiesSQL = `
	WITH RECURSIVE
	tagged AS (
	    SELECT t.vseq AS tvseq, ev.entity_id AS id, ev.face AS face, ev.type AS type
	    FROM version_tags t
	    JOIN entity_versions ev ON ev.vseq = t.vseq
	    WHERE t.name = {{name}}
	),
	fwd(tvseq, id, face, after) AS (
	    SELECT tvseq, id, face, tvseq FROM tagged
	    UNION ALL
	    SELECT f.tvseq, r.entity_id, r.face, r.vseq
	    FROM fwd f
	    JOIN entity_versions r ON r.vseq = (
	        SELECT min(x.vseq) FROM entity_versions x
	        WHERE x.op = 'rename' AND x.prev_id = f.id
	          AND COALESCE(x.prev_face, x.face) = f.face AND x.vseq > f.after)
	    WHERE NOT EXISTS (
	        SELECT 1 FROM entity_versions d
	        WHERE d.entity_id = f.id AND d.face = f.face AND d.op = 'delete'
	          AND d.vseq > f.after AND d.vseq < r.vseq)
	),
	head AS (
	    SELECT w.tvseq, w.id, w.face, w.after FROM (
	        SELECT f.tvseq, f.id, f.face, f.after,
	               row_number() OVER (PARTITION BY f.tvseq ORDER BY f.after DESC) AS rn
	        FROM fwd f
	    ) w
	    WHERE w.rn = 1
	),
	bwd(tvseq, id, face, lo, hi) AS (
	    SELECT h.tvseq, h.id, h.face,
	           COALESCE((SELECT max(x.vseq) FROM entity_versions x
	                     WHERE x.prev_id = h.id AND x.op = 'rename'
	                       AND COALESCE(x.prev_face, x.face) = h.face), 0),
	           9223372036854775807
	    FROM head h
	    UNION
	    SELECT b.tvseq, r.prev_id, COALESCE(r.prev_face, r.face),
	           COALESCE((SELECT max(x.vseq) FROM entity_versions x
	                     WHERE x.prev_id = r.prev_id AND x.op = 'rename' AND x.vseq < r.vseq
	                       AND COALESCE(x.prev_face, x.face) = COALESCE(r.prev_face, r.face)), 0),
	           r.vseq
	    FROM bwd b
	    JOIN entity_versions r
	      ON r.entity_id = b.id AND r.face = b.face AND r.op = 'rename'
	     AND r.prev_id IS NOT NULL AND r.vseq < b.hi
	),
	ord AS (
	    SELECT b.tvseq, count(DISTINCT ev.vseq) AS n,
	           max(CASE WHEN ev.vseq = b.tvseq THEN 1 ELSE 0 END) AS has
	    FROM bwd b
	    JOIN entity_versions ev
	      ON ev.entity_id = b.id AND ev.face = b.face AND ev.vseq > b.lo AND ev.vseq < b.hi
	     AND ev.vseq <= b.tvseq
	    GROUP BY b.tvseq
	),
	found AS (
	    SELECT h.tvseq, h.id, h.face, tg.type AS tagged_type, e.type AS live_type,
	           (e.id IS NULL OR EXISTS (
	               SELECT 1 FROM entity_versions d
	               WHERE d.entity_id = h.id AND d.face = h.face AND d.op = 'delete'
	                 AND d.vseq > h.after)) AS deleted,
	           CASE WHEN o.has = 1 THEN o.n ELSE 0 END AS version
	    FROM head h
	    JOIN tagged tg ON tg.tvseq = h.tvseq
	    LEFT JOIN entities e ON e.id = h.id AND e.face = h.face
	    LEFT JOIN ord o ON o.tvseq = h.tvseq
	)
	SELECT q.id, q.face, q.type, q.version, q.deleted FROM (
	    SELECT tvseq, id, face, deleted, version,
	           CASE WHEN deleted THEN tagged_type ELSE live_type END AS type
	    FROM found
	) q
	WHERE {{type}} = '' OR q.type = {{type}}
	ORDER BY q.id, q.face, q.version, q.tvseq`

// TaggedEntitiesSQL returns the tagged-entities query with nameParam and
// typeParam as the bind expressions of the tag name and the type filter.
// Each backend passes its own syntax, for example "$1" and
// "CAST($2 AS text)". The parameters are fixed by the backend, never taken
// from input.
func TaggedEntitiesSQL(nameParam, typeParam string) string {
	return strings.NewReplacer("{{name}}", nameParam, "{{type}}", typeParam).Replace(taggedEntitiesSQL)
}

// TaggedEntityRow builds a [store.TaggedEntity] from the query's columns.
func TaggedEntityRow(id, face, typ string, version int, deleted bool) store.TaggedEntity {
	return store.TaggedEntity{
		Ref:     entity.Ref{ID: id, Face: entity.Face(face)},
		Type:    typ,
		Version: version,
		Deleted: deleted,
	}
}
