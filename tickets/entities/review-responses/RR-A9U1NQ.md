---
id: RR-A9U1NQ
type: review-response
title: List export resolves relations + ACL gate per-cell, violating the batching contract
finding: 'listTableRenderer calls columnCell -> visibleNeighborTitles per (row x relation column): each does a ListRelations store query + visibleRelationIDs (PermitsReadMany). The relation_visibility.go/CLAUDE.md contract says collect ALL neighbor IDs for the page and gate once, NOT per-row in a loop. For a 5000-row export with 2 relation columns that is up to 10k store queries + 10k ACL calls, each a DB roundtrip on postgres, inside one synchronous request. Output is correct; this is a scalability/DoS regression on the highest-cardinality path.'
severity: significant
resolution: 'List export now resolves relation columns in ONE batched pass (resolveListRelations/gatherListPeers/memoNeighborTitle in export_list.go): each row''s edges load once, ALL neighbor IDs across all rows feed a SINGLE visibleRelationIDs gate, and each distinct visible neighbor''s title loads once (memoized). Replaces the per-cell store-query + ACL-gate fan-out. Test: TestExport_List_SharedNeighborResolvedOnce (shared neighbor rendered in both rows via the memo) + existing whole-set/hidden-neighbor tests.'
status: addressed
---
