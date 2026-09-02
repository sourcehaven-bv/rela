---
id: TKT-5LUGYP
type: ticket
title: 'Gantt perf: header projection + SQL-scoped subtree drill'
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Follow-up to TKT-MW28U5, driven by the postgres scaling report
(`.ignored/gantt-perf/REPORT.md`): the gantt endpoint is linear in STORE size
(~7–8µs/entity, 387ms at 50k), dominated by two full-table streams —
`ListEntities` decoding every source-type row including markdown bodies the
gantt never reads (~55% of handler cost), and `ListRelations` (~30%). A drill
(`?root=`) costs the same as the full tree (RR-FJWAZS, confirmed at 327 vs
387ms).

## Scope

**A. Header projection.** Load the forest through `store.ListEntityHeaders`
(ID/Type/Properties/Redacted — no body) instead of full entities, honouring the
read-gate verdict exactly as `scopedSortedEntities` does. Field redaction via
`visibility.RedactHeader`. Cuts row payload, decode and GC.

**B. SQL-scoped subtree drill.** `?root=` resolves the subtree via the existing
`pgstore.GraphQuery` pushdown (relation-type + depth predicates) instead of
building and discarding the global forest. Closes RR-FJWAZS.

Both preserve the gate→redact-once→fold→cap pipeline invariants (CLAUDE.md);
under an ACL Query verdict where a scoped header/subtree path is not
expressible, fall back to the existing full build (correct, slower) rather than
widening.

## Out of scope

Per-principal forest caching (option C — deferred until measured need),
where:-pushdown (post-redact evaluation is a pinned security property).
