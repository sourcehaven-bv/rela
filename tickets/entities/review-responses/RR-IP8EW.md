---
id: RR-IP8EW
type: review-response
title: visibleEntities has dead assignments and an O(P^2 T) rescan
finding: Inner loop reassigns `visible[name] = true` for entities already true from the outer loop. Then a third pass re-scans every pair for every entity to check 'only-legend participation'. Can be a single pass maintaining seenAny and seenNonLegend sets. Code quality, not correctness.
severity: minor
resolution: 'Replaced with a single-pass implementation using two sets (seenAny, seenDrawn) and a final derivation: `visible = !seenAny || seenDrawn`. Drops the dead assignments and the O(P^2 T) rescan.'
status: addressed
---
