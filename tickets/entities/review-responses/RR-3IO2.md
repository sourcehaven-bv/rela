---
id: RR-3IO2
type: review-response
title: TKT-VMD8 AC9 search-after-ACL test must assert call ordering, not just output
finding: 'Plan''s data-flow claim is correct against today''s scopedSortedEntities (line 371-395): load → search-intersect → filter → sort. Add to AC9: ''test must assert the search backend is invoked AFTER the ACL GraphQuery, e.g. via a mock searcher that records its call ordering relative to the store mock.'' Without that, ''filter the search results'' can be implemented by post-filtering search hits — same observable output, wrong contract for the future /_search ticket.'
severity: minor
reason: 'Belongs to TKT-VMD8 (list/sidebar PR 2/2). AC9 names a list-handler test (search-backend invoked after ACL GraphQuery in scopedSortedEntities) — that handler lives in TKT-VMD8''s scope and the call-ordering test must be written against that code, not this PR''s per-entity gate. Will be addressed in TKT-VMD8 implementation, with a mock searcher that records its invocation timestamp relative to the store mock so post-filter implementations fail the test.'
status: deferred
---
