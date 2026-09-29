---
id: BUG-58BL9I
type: bug
title: 'Deleting a face with a content-scoped edge is refused: relation check reads the zero face'
description: The delete path resolves an edge's source type with a zero-face read, so a faced source has no type and the relation delete check fails closed with 403.
priority: medium
effort: s
status: backlog
---

## Problem

Observed in the TKT-WCMW47 e2e fixture (PR #1711): `DELETE
/api/v1/policies/POL-1@draft` returns 403 `no role grants delete on relations
from type ""` when the face has a content-scoped edge. A face without edges
deletes with 204.

Verified in code: the relation authorization in the delete path resolves the
edge's source type with `tx.GetEntity(ctx, rel.From)`
(`internal/entitymanager/manager.go` ~1310). That is a zero-face read, so a
faced source resolves to nothing, `fromType` is empty, and the check fails
closed.

## Expected

The source type is resolved from the edge's own tail (`rel.From`,
`rel.FromFace`), or from any face of the family, since the type is the same at
every face. Deleting a face with content-scoped edges succeeds for a principal
allowed to delete the face and its edges.
