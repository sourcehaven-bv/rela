---
id: RR-UYB99D
type: review-response
title: FieldCeilingFor re-derived the baseline the Request already resolved
finding: FieldCeilingFor called policy.baselineFor(principal.PrincipalType()) on every invocation, even though compiledCeiling.name already held the baseline matched at Request construction. Two lookups keyed off the same source, derived at different times — they agree only because the principal is immutable after binding, which undercuts the stated reason for computing the ceiling once ('there is nothing to invalidate'). Also a map walk on every field verdict in a list response.
severity: minor
resolution: compiledCeiling now stores the matched ClientBaseline at construction; FieldCeilingFor reads it. One derivation, one source of truth, no per-call lookup.
status: addressed
---
