---
id: RR-VOCBBC
type: review-response
title: Cascade and copy writes and RestoreEntity bypass the bounds
finding: Cascade WriteRelation, copy and promote merge, and RestoreEntity wrote edges without checking max_outgoing or max_incoming.
severity: significant
resolution: Fixed in 21e383051 and 71d1088cc. Cascade and copy now check the bound (TestCascadeWriteRelation_RefusedOverMaxOutgoing, TestCopyReplace_RefusedOverMaxIncoming). Restore of a soft-deleted entity stays unbounded by design because its edges are grandfathered. This is commented in the code. Bulk loaders stay unbounded and GUIDE-concepts documents this.
status: addressed
---

Review finding R1-3.
