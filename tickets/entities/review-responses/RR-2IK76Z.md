---
id: RR-2IK76Z
type: review-response
title: Relation tails gated by Family leak relations on hidden faces
finding: Content-scoped relation tails attach to (From FromFace) but the design gates both endpoints with Family mode so a reader of any face sees a hidden face's relations
severity: significant
resolution: 'Design section 8.2 (PR 3 and PR 5): head uses Family(To); tail uses Ref{From FromFace} when FromFace is set and Family otherwise; FamilyMany becomes EndpointsReadable with per-type batches. Test: tail on hidden face B is 404 and absent from lists.'
status: addressed
---

**Where:** design section 3 (`relation_read_handler.go:111-112`,
`relation_history_handler.go:166,167,317`, `write_handler.go:1367`) and risk 3
(`FilterRelations` via `FamilyMany`).

These map both relation endpoints to Family mode ("any readable face of this
id"). A content-scoped relation's tail is face level: it attaches to `(From,
FromFace)`. Gating the tail by Family lets a principal who may read face A of an
entity read a relation whose tail is on face B, which they may not read.
Relation content and existence are secret, so this is a disclosure the current
per-face intent does not allow.
