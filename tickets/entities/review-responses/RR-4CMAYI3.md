---
id: RR-4CMAYI3
type: review-response
title: Id mode ignores the relation's endpoint types
finding: 'resolveRelationTargets switches a key to id mode for any served entity whose id equals the value. Slug ids exist (e.g. concept `search`), so an old title link filter[requires]=search silently matches only edges to that entity. The design''s ''ids have a type prefix'' premise is false. Fix: take id mode only when the served type is a valid endpoint of the relation in the control''s direction.'
severity: significant
resolution: 'relationReaches checks that the value''s type is a type the relation reaches in the control''s direction; otherwise the value is a title. Test: TestRelationFilter_IDOfUnreachableTypeIsATitle.'
status: addressed
---
