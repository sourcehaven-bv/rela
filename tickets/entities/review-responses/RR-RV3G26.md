---
id: RR-RV3G26
type: review-response
title: SearchVisibleFields lost its memory bound
finding: With hidden fields and text, the fix read the whole filtered result into memory before judging rows. SearchVisible without a limit did the same.
severity: significant
resolution: Both now read keyset pages over (rank, id) via visiblePages; the page size is min(q.Limit, page size) when no Go-side filter can drop rows. Pinned by TestBuildVisibleSearchSQL_Keyset and the visible-search conformance suites at page size 2.
status: addressed
---
