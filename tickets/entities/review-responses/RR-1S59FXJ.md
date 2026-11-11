---
id: RR-1S59FXJ
type: review-response
title: Transforms need post-conditions
finding: A cmd transform can produce any type or size.
severity: minor
resolution: Final bytes are re-sniffed against accept and re-capped; stored MIME and name come from the final output.
status: addressed
---
