---
id: RR-3QEVFT
type: review-response
title: Next-action count-zero hint applies default scopes
finding: nextaction.go applies the type's default query scope, so a default scope using related() runs traversals on a next-action path the plan listed as out of scope. Must be covered and tested.
severity: significant
resolution: 'countIsZero goes through scopedSortedEntities and applyScope, so a default scope''s traversal runs under the caller''s gate there too. Test: TestQueryScopeTraversal_CountZeroHintAppliesTheDefault (alice and bob false, carol true).'
status: addressed
---
