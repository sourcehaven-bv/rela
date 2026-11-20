---
id: RR-C2LIL6B
type: review-response
title: Non-id users join the title batch
finding: Every non-reserved user, including emails or JWT subjects that are not entity ids, is added to the ResolveIDsErr batch.
severity: minor
reason: Bounded by the page size and part of the one existing batch query; an unknown id returns nothing. Guessing id syntax would miss custom prefixes.
status: wont-fix
---
