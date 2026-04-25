---
id: RR-M1TIC
type: review-response
title: Manual ID input has no format hint, violating principle of least surprise
finding: 'The existing InlineCreateModal placeholder is ''Unique ID...'' with no guidance. For manual-ID types, the metamodel may define id_pattern validation (via custom types). The plan should: (a) surface the expected pattern/description as placeholder or help text, OR (b) explicitly punt on that as out-of-scope. Without guidance, users will hit server-side 422 errors they can''t easily diagnose. At minimum, mention this in the plan so the reviewer knows it was considered.'
severity: minor
reason: Surfacing the metamodel-defined ID pattern as a form hint is a separate UX concern; not blocking for this ticket. Users still get a server-side 422 with the pattern error. Call out in scope as deferred.
status: deferred
---
