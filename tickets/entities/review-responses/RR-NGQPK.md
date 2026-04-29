---
id: RR-NGQPK
type: review-response
title: Two overlapping formatters; rrule normalization duplicated
finding: 'Architectural smell: formatValue and formatCellValue have overlapping responsibilities (different null sentinel, different way of resolving the type). The `RRULE:` prefix normalization is also duplicated between formatValue and RruleBuilder.vue. A single formatter with an `emptySentinel` option, plus a shared `normalizeRruleString` helper, would kill the drift class entirely.'
severity: minor
reason: Architectural refactor (unify formatValue/formatCellValue, extract normalizeRruleString shared with RruleBuilder.vue) is out of scope for this xs ticket. The current change reduces drift to a single delegation call. Filing as a follow-up refactor ticket would be appropriate; not blocking this fix.
status: deferred
---
