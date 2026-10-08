---
id: RR-0ACCUZ
type: review-response
title: Move fails between tied or collapsed values
finding: MidpointOrder returns false on ties/collapse; renumber only runs after a write.
severity: significant
resolution: 'Plan updated: Same as densify path: on midpoint failure renumber in the Tx and recompute. Test equal-value siblings.'
status: addressed
---
