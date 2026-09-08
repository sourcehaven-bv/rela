---
id: RR-YK5XFA
type: review-response
title: Frontend regression tests only asserted textContent
finding: Text survives the broken configurations too so the preserves tests could not distinguish preserved labels from flattened ones.
severity: significant
resolution: 'Tests assert structure (foreignObject.children>0 / .nodeLabel / p / br / style) and label-side sanitization separately. Mutation-verified: no-restore fails 7 tests; unsanitized-restore fails 5.'
status: addressed
---
