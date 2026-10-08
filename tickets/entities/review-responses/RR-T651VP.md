---
id: RR-T651VP
type: review-response
title: Heading context empty for selections starting in a heading
finding: extractHeadingContext sees the partial line '## ' and yields empty, so the plan's claim that heading context disambiguates repeated sections is false for the main use case.
severity: significant
resolution: 'Plan corrected: recorded as known limitation (changing extraction would rescore existing stored anchors); repeated sections are disambiguated by prefix/suffix, pinned by tests; documented in docs/comments.md.'
status: addressed
---
