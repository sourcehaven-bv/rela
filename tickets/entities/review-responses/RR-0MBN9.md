---
id: RR-0MBN9
type: review-response
title: nearestAnchorId scans full subtree per click
finding: 'frontend/src/composables/useDocumentClicks.ts:81 calls querySelectorAll(''[id]'') on .document-body every click, then iterates with compareDocumentPosition. O(n) in the number of ids per click. Also, the preceding-id-walk branch doesn''t re-check isGlobalChrome per candidate (unlike the ancestor branch) — nested .document-body containers could leak ids across. Fix: walk previousElementSibling + parent chain instead of querySelectorAll. Or accept current implementation for typical doc sizes but fix the nested-container hole.'
severity: minor
resolution: 'Fixed the nested-.document-body leak: nearestAnchorId now rejects candidates whose nearest .document-body ancestor differs from the start element''s container. The O(n)-per-click concern is acknowledged and accepted — typical doc sizes are under a few hundred ids, and the click-path is user-initiated (not hot). revisit if profiling shows a real cost.'
status: addressed
---
