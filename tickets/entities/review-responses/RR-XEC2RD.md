---
id: RR-XEC2RD
type: review-response
title: Schema/data mismatch on a scalar enum warns once per row per render
finding: A property declared scalar enum whose stored value is an array (legacy data, hand-edited markdown, an un-backfilled metamodel change) routes to SelectWidget, whose safeStringValue console.warns per instance and renders only the first element - so 'b' is silently dropped where the old string path joined both. Routing is decided per column from the SCHEMA, but this failure is per row from the DATA, so the hint cannot prevent it.
severity: significant
reason: 'Not introduced by this change and not fixable in the routing layer. The pre-existing detail-view path has the identical exposure: PropertyDisplay resolves the same SelectWidget from the same schema, so any surface rendering a scalar-declared enum holding an array already warns and truncates. Fixing it means either making SelectWidget''s warn non-per-render or having it render all elements, both of which change a widget shared with forms and detail views - out of scope for a display-path migration. Filed as a follow-up rather than silently widened into this PR.'
status: deferred
---
