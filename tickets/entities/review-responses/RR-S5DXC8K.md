---
id: RR-S5DXC8K
type: review-response
title: Guard copies V per subtree and hits MaxCondRelations
finding: Many guarded subtrees can exceed store.Cond limits; untested.
severity: minor
reason: The refusal is exact and now tested (TestScriptReader_ListEntityPage_HiddenFieldTooLarge). The single-hidden-property expansion is an optimization for large filters and can follow when a caller needs it.
status: deferred
---
