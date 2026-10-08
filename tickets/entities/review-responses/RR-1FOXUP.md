---
id: RR-1FOXUP
type: review-response
title: Engine and read paths break ties differently
finding: SortRelations, sortRelationGroup and the planned list sort disagree on ties/missing values.
severity: significant
resolution: 'Plan updated: One comparator (finite first, value, other endpoint id, tail) next to FiniteOrder, used by engine, list, section, sortRelationGroup; mirrored in the client.'
status: addressed
---
