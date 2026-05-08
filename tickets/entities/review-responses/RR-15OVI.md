---
id: RR-15OVI
type: review-response
title: Top-level tool description omits the empty-string no-op rule
finding: tools.go:88-92. Top-level description says 'set a property to null in `properties` to remove it' but does NOT mention that empty string is a silent no-op. The per-arg description has both. If a client renders only the top-level description, the model may treat "" as delete — wrong. Make both descriptions carry the same contract.
severity: significant
resolution: 'Extended the top-level tool description in tools.go to carry the same contract as the per-arg description: null deletes, empty string is silently ignored, required properties cannot be deleted. Both descriptions now match.'
status: addressed
---
