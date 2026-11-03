---
id: RR-Y5QGKK
type: review-response
title: Colour table duplicates Badge.vue and folds purple to blue
finding: useRelationColumns had its own copy of the style colour table from Badge.vue. Purple was mapped to blue.
severity: nit
resolution: Fixed in 82f0f9a0d. Both use the shared table in frontend/src/utils/styleColors.ts. Purple is added to StatusColor. Covered by the styleStatusColor tests.
status: addressed
---

Review finding R2-11.
