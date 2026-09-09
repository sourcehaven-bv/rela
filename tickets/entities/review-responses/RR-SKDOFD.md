---
id: RR-SKDOFD
type: review-response
title: worldQueryFor duplicates setWorld's normalisation and drops the page reset
finding: EntityDetail.vue worldQueryFor re-implements useWorld.setWorld's world-query spelling (norm helper copied verbatim) but keeps `page`, which setWorld deletes on purpose. Two copies of one rule, one already diverged. Export the spelling from useWorld and use it from both.
severity: significant
resolution: 'useWorld exports worldQuery(next, base, defaultWorld): the one place the spelling and the page reset live. setWorld and landAfterCopy both call it; worldQueryFor is gone. The existing setWorld tests cover the rule; the new EntityDetail landing test asserts the page reset on the copy path.'
status: addressed
---
