---
id: RR-IDBJ5Q
type: review-response
title: srHint is dropped when RlSegmentedControl is iconOnly
finding: 'frontend/packages/rela-components/src/components/data/RlSegmentedControl.vue:76,81: in iconOnly mode aria-label=option.label overrides the button content, so the new srHint span is never read. The composer does not use iconOnly today, but SegmentOption.srHint is public API. Fix: in iconOnly mode, set aria-label to `${label}${srHint ?? ''''}`, or document that srHint is ignored there.'
severity: nit
resolution: With iconOnly the aria-label is label plus srHint; two segmented-control stories cover both modes.
status: addressed
---
