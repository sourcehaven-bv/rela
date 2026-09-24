---
id: RR-3MMGN9
type: review-response
title: Index inference must not switch to LowerScope
finding: listScopeIndexProperties derives columns for every scope's equalities because the Go path pushes them as ScopeProps even for inexact scopes (queryplan.go:421-459). Using LowerScope there drops columns for inexact scopes.
severity: significant
resolution: 'Plan: index columns stay on ConditionIndexProperties; LowerScope decides only pushdown eligibility. A test asserts LowerScope props are a subset of the index columns.'
status: addressed
---
