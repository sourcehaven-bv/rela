---
id: RR-IPK7W2
type: review-response
title: Redirect carries the child list scope to the owner page
finding: The owned-entity redirect kept the whole query, so prev/next used the child's list scope.
severity: nit
resolution: Only ?world= is kept. Covered in EntityDetail.owner.test.ts.
status: addressed
---
