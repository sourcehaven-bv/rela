---
id: BUG-58BL9I
type: bug
title: 'Deleting a face with a content-scoped edge is refused: relation check reads the zero face'
description: The delete path resolves an edge's source type with a zero-face read, so a faced source has no type and the relation delete check fails closed with 403.
priority: medium
effort: s
why1: The cascade relation check in authorizeCascadeRelations resolved each edge's source type with tx.GetEntity(rel.From), a zero-face read; a faced source such as POL-1 has no zero-face row, so fromType was empty and the check failed closed.
why2: 'The lookup predates faces and was not changed when content-scoped edges gained a FromFace: the subject key started carrying rel.FromFace (BUG-64MU2Q) but the type lookup kept reading the bare id.'
why3: The BUG-HC6I2T sweep replaced zero-face endpoint reads with anyFaceOf on the relation write paths but not in the delete cascade check, because that call ran inside the Tx and was grouped with the delete code.
why4: Cascade-delete ACL tests used faceless fixtures (decision and requirement), so an empty source type never arose; no test deleted a faced entity or face with a content-scoped edge under a real policy.
why5: Faces were added route by route to APIs built for one record per id (DEC-NPZICR); the zero-face read allowlist tolerated the call instead of forcing each remaining site to state which face it reads.
prevention: edgeSourceType reads the edge's tail face and falls back to any face of the family; TestDeleteEntityFace_ContentEdge* and TestDelete_CascadeResolvesEdgeSourceType run on every concBackend; the archguard zero-face allowlist for manager.go shrinks to 1; the e2e face-delete spec is live.
status: done
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
