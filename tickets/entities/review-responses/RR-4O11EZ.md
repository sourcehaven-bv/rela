---
id: RR-4O11EZ
type: review-response
title: B1 double-flags a type used as both a verb grant and an affordance key; slicesClone misnamed; stale roleGrants comment
finding: 'Three small ones: (a) B1''s seen map keys on verb+type, so a single typo''d type in both create: and fields: yields two findings for one mistake — dedupe on type alone for B1. (b) cli/acl.go slicesClone also sorts; rename to sortedClone so readers don''t assume order-preservation (shadows stdlib slices.Clone semantics). (c) aclaudit.go: the doc comment says ''roleGrants reports...'' above func roleDeclared — stale comment, fix the name.'
severity: nit
resolution: (a) B1 dedupe now keys on type alone (one typo'd type = one finding). (b) cli/acl.go slicesClone renamed to sortedClone. (c) roleDeclared doc comment fixed (was stale 'roleGrants').
status: addressed
---
