---
id: RR-67NJ
type: review-response
title: Lock icon emoji not announced to screen readers
finding: "PropertyDisplay.vue, EntityList.vue (desktop + mobile), EntityDetail.vue: bare \U0001F512 emoji inside a span. Screen-reader users get inconsistent output ('lock encrypted', 'lock', or nothing depending on UA). Only the EntityDetail banner has aria-hidden. Fix: wrap with `<span aria-label=\"encrypted\" role=\"img\">\U0001F512</span>` or move emoji into ::before with aria-hidden and put a plain-text 'encrypted' label in the DOM. Accessibility regression."
severity: minor
status: open
---
