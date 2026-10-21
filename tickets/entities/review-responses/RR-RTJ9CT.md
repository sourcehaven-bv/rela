---
id: RR-RTJ9CT
type: review-response
title: Kept entity leads to a duplicate on retry
finding: The 500 conversion_incomplete names the id only in detail; the SPA shows an error and a second click creates a second entity.
severity: minor
resolution: The 500 conversion_incomplete now carries entity_id. The SPA links the item to that entity and names it in the error toast, so a second press does not make a duplicate. Pinned by a handler test and a component test.
status: addressed
---
