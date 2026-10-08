---
id: RR-348WSJ
type: review-response
title: '[security] No negative case showed the relation still limits the face grant'
finding: '[security] Every 200 was for alice with the owned-by edge; a shortcut that granted ticket@draft from the face allowlist alone (skipping the relation query) would pass every case.'
severity: significant
resolution: Added TKT-002@draft (no edge) and bob on TKT-001@draft (no relation), both expecting 404 on both routes. A mutation of readableFacesMany that skips the relation query when a face allowlist is set fails the unrelated-entity cases on both routes (verified).
status: addressed
---
