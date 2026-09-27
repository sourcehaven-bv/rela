---
id: RR-DUVVZ1
type: review-response
title: Non-DenyAll gate refusals must not read as no match
finding: ErrTraversalDenied for faces, Any and slot collision applies to partly readable types. Treating it as no match makes not related(...) match every candidate, widening the filter.
severity: minor
resolution: 'Only DenyAll returns ErrTraversalDenied (read as no match). Faces, Any, field refusals and slot collisions return ErrTraversalUnsupported, which the scope returns as an error. Test: TestQueryScopeFilter_FailsClosed uses the negated scope.'
status: addressed
---
