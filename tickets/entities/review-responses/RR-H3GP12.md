---
id: RR-H3GP12
type: review-response
title: List q= free text is the same hidden-value oracle
finding: 'freeTextIDsForType searches with the raw Searcher, not the field-provenance VisibleSearcher path /_search uses, so a list row survives ?q=<guess> when the guess matches only a hidden property. Verified on the golden fixture: as the viewer, /api/v1/tickets/?q=secret returns TKT-2, whose only match is its hidden secret; /_search?q=secret drops it. Route the list free-text pass through searchVisibleHits.'
severity: critical
resolution: freeTextIDs now searches through searchVisibleHits with the read gate's search scope and the field-provenance filter that /_search uses; a searcher that cannot redact yields errACLListQuery. TestListFieldGate_FreeTextSkipsHiddenMatch pins it (viewer q=bravo returns nothing; hr gets TKT-001).
status: addressed
---
