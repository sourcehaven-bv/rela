---
id: RR-ACJ0ZY
type: review-response
title: RenameEntity re-keyed relations with no tombstone for old triples — ghost edges
finding: Relations are keyed (from_id, rel_type, to_id). Rename runs UPDATE relations SET from_id/to_id = newID (entity.go:406-417), changing the PRIMARY KEY of every incident relation. To an id-keyed client the old triple (oldID, type, X) is removed and (newID, type, X) is new. The manifest surfaces the new triples but never reports the old triples gone -> permanent ghost edges in every sync client for every relation touching a renamed entity.
severity: critical
resolution: RenameEntity now captures the incident relation triples (SELECT ... WHERE from_id=$1 OR to_id=$1) BEFORE re-keying, and writes a relation tombstone for each old triple in the same tx. Regression TestRenameTombstonesOldIdentities asserts the old (DEC-1,addresses,REQ-1) triple is tombstoned and the re-keyed (DEC-2,...) appears live.
status: addressed
---
