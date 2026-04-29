---
id: RR-9ZQLP
type: review-response
title: Separate datetime formatter for time-bearing values
finding: 'If a datetime property type is added later, this helper silently truncates. A formatDateTime companion using dateStyle: ''medium'', timeStyle: ''short'' would match. Out of scope here.'
severity: nit
reason: No datetime property type exists in the metamodel today. When/if added, formatDateTime can be authored alongside it; speculatively adding the helper now violates 'don't design for hypothetical future requirements' (CLAUDE.md rule).
status: deferred
---
