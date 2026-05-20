---
id: RR-67NJ
type: review-response
title: Lock icon emoji not announced to screen readers
finding: "PropertyDisplay.vue, EntityList.vue (desktop + mobile), EntityDetail.vue: bare \U0001F512 emoji inside a span. Screen-reader users get inconsistent output ('lock encrypted', 'lock', or nothing depending on UA). Only the EntityDetail banner has aria-hidden. Fix: wrap with `<span aria-label=\"encrypted\" role=\"img\">\U0001F512</span>` or move emoji into ::before with aria-hidden and put a plain-text 'encrypted' label in the DOM. Accessibility regression."
severity: minor
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
