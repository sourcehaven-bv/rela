---
id: RR-E3Y5N0R
type: review-response
title: A single PATCH looks runs up twice and falls back per row on failure
finding: A PATCH looks the subject's runs up once for the verdicts and once on the write check; on a lookup failure a section falls back to a lookup per row.
severity: minor
reason: 'Both are bounded: two indexed lookups per single-entity write, and the per-row fallback only runs after the page lookup already failed. Batching across the write check needs a request-scoped cache in the manager; not worth the coupling for this ticket.'
status: deferred
---
