---
id: RR-BPFGG6
type: review-response
title: 'Cross-type label scan: inaccurate styles analogy + untested collision'
finding: 'labelsForProperty falls back to scanning entity+relation types and returning the first-inserted match when entityType is omitted (first-wins on collision). This is defensible (search-mode filters have no single type; mirrors the documented AdHocFilterMenu union), but: (a) the doc comment inaccurately claims it mirrors how `styles` collapses — `styles` is a flat server-authored config map, not a per-type-def scan; (b) no test pins the ambiguous no-entityType first-wins behavior. Fix the comment and add a collision test.'
severity: significant
resolution: Rewrote labelsForProperty's comment to document the deliberate first-wins tie-break and to correct the styles analogy (it is a per-type-def scan, not the flat server-authored styles map). Added schema.test.ts 'falls back to first-inserted type on collision when no entity type given' pinning the behavior.
status: addressed
---
