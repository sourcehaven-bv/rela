---
id: RR-L6XI6S
type: review-response
title: 'Badge text-transform: capitalize overrides author label casing'
finding: 'Badge.vue has `text-transform: capitalize`. Once an author-supplied label flows in (e.g. `high priority` -> `High Priority`, or an intentionally-cased label), capitalize silently overrides the author''s chosen display form. Plan must drop `capitalize` when a real label is present and only apply it to the fallback raw value.'
severity: significant
status: open
---
