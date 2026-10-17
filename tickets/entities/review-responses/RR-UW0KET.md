---
id: RR-UW0KET
type: review-response
title: Anchor-mode record misses hot reload and is too strict
finding: Types added by reload are never recorded; types with no channels are refused; RecordAnchorModes writes on every Assemble. Record lazily on first write.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
