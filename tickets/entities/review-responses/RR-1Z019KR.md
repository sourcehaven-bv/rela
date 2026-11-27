---
id: RR-1Z019KR
type: review-response
title: PatchWithRelations did not fire
finding: A data-entry save fired only when no other field changed.
severity: significant
resolution: PatchWithRelations fires removals then creations inside the patch transaction.
status: addressed
---
