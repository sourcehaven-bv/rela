---
id: RR-IJNT
type: review-response
title: query_test.go duplicates two-step init pattern 3 times
finding: 'The pattern `ws := &Workspace{graph: g}; ws.meta.Store(meta)` is duplicated in three test functions in query_test.go. Extract to a helper or use NewForTest.'
severity: nit
resolution: Extracted `newQueryTestWorkspace(g, meta)` helper at the top of query_test.go. All three test functions now use it. The helper is explicit about not initializing a search index (those tests don't need one), which is a small pedagogical improvement over reaching for the production constructor.
status: addressed
---
