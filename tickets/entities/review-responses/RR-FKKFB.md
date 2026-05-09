---
id: RR-FKKFB
type: review-response
title: isCancelledFetch location is awkward (in usePageData.ts)
finding: 'We import isCancelledFetch from @/composables/usePageData. The helper is exported and stable, but its location has nothing to do with usePageData''s lifecycle role. Fix: move to src/api/cancellation.ts (or src/utils/cancellation.ts) so future callers don''t pull in a usePageData import they don''t need. Defer; not blocking this ticket.'
severity: nit
reason: Nit-level. isCancelledFetch is already exported and stable; moving it is mechanical and unrelated to the palette feature. Track as cleanup for whoever adds the next AbortController-using consumer.
status: deferred
---
