---
id: RR-K2PXAT
type: review-response
title: Row href recomputed per cell reintroduced the per-cell work RR-UD2A removed
finding: |-
    `entityRowHref(entity)` appeared four times in the cell wrapper's bindings and
    is a plain function, not a computed, so Vue re-invoked it on every read. Each
    call runs `entityRowLocation`, which walks `Object.entries(route.query)`, does a
    `listConfig.columns.find()` and two `sortSpecs.map().join()`, then allocates a
    URLSearchParams. On the documented 200-row dense-list target with 6 columns that
    is thousands of allocations per render. The file's own `columnWidgets` comment
    explains at length why per-cell work was hoisted to per-column for exactly this
    reason (RR-UD2A) — this reintroduced the pattern that comment exists to
    prevent.
severity: significant
resolution: |-
    A row's href is identical for all its columns, so it is now computed ONCE per
    row in a `rowLinks` computed keyed by entity id, mirroring `columnWidgets`.
    `rowCellWrapper` is a Map lookup plus a `colIndex !== 0` short-circuit, and the
    non-first columns share a single frozen `inertCell` object rather than
    allocating one each.

    Not a behaviour change, so no new test — the existing row-link tests cover the
    values, and the full 2066-test suite stays green.
status: addressed
---

## Resolution

A row's href is identical for all its columns, so it is now computed ONCE per
row in a `rowLinks` computed keyed by entity id, mirroring `columnWidgets`.
`rowCellWrapper` is a Map lookup plus a `colIndex !== 0` short-circuit, and the
non-first columns share a single frozen `inertCell` object rather than
allocating one each.

Not a behaviour change, so no new test — the existing row-link tests cover the
values, and the full 2066-test suite stays green.
