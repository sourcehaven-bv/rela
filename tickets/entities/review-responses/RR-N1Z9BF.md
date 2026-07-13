---
id: RR-N1Z9BF
type: review-response
title: Widget must be non-destructive for datetime values it did not author (git churn)
finding: 'Widget always emits `...Z`. If a file stores `due: 2026-07-13T12:30:00+02:00`, opening the entity and saving an UNRELATED field round-trips the datetime through zone->UTC and rewrites it to `...Z` (same instant, different bytes) => spurious git diff on an untouched field. Fix: dirty-track per field; emit a new value ONLY when the user interacts with THIS field; pass the original stored string through verbatim otherwise. Confirm the data-entry PATCH path sends only changed fields (per-property auto-save from TKT-E6094 suggests it does). Canonicalization to Z, if wanted, must be an explicit migration, not an incidental side effect of viewing.'
severity: significant
resolution: 'Accepted into plan. Widget is non-destructive: (a) dirty-track per field, emit only on user interaction with THIS field, never on mount; (b) untouched fields pass their original stored string through verbatim. Data-entry uses per-property auto-save (TKT-E6094) which PATCHes only the changed field, so an untouched datetime is never re-sent - confirm in impl. No incidental Z-canonicalization. Widget always emits ...Z only for values the user actually edits.'
status: addressed
---
