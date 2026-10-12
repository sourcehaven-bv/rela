---
id: RR-GP31IH
type: review-response
title: Delta re-point refused with 422 after properties were saved
finding: DynamicForm sends add and remove deltas. The replace path ignored deltas and wrote upserts before removes. The create of the new target was refused while the old edge still counted. The PATCH returned 422 after the properties were already saved.
severity: critical
resolution: Fixed in 21e383051. ReplaceRelations sends every bounded relation through one transaction, whether the PATCH has data or deltas. A create is checked with the removed edges not counted. Covered by TestPatchRelations_DeltaRepointsSingleValued.
status: addressed
---

Review finding R1-1.
