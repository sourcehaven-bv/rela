---
id: RR-S5OBY0F
type: review-response
title: Declined /_search shapes re-read the whole match set per page
finding: executeQueryCut reads offset+size+1 rows for every page of a shape the plan declines, so paging a large declined result costs a full read per page.
severity: significant
reason: Exact totals over a Go-filtered shape need the full read. Lowering those shapes into the store is TKT-PSBGQLY and TKT-C5H506O; the allowlist reason for scopedHeaders now names this cost.
status: deferred
---
