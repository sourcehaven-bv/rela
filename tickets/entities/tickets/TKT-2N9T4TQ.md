---
id: TKT-2N9T4TQ
type: ticket
title: Derive postgres indexes from list filter controls
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Derive postgres indexes from list `filter_controls`. Part of the smart-lists
feature (RES-7SH71TW).

`queryplan.listIndexSpec` (`internal/queryplan/queryplan.go`) builds a
`DerivedListIndex` from a list's static `=` filters and `sort:` keys. It does
not read `filter_controls`, so a runtime control filter has no index.

## Scope

- One index per control: (type, face, control property, then the list's sort
keys). Postgres combines them with a BitmapAnd for multi-control filters.
- An opt-in composite on the list for a hot combination, e.g.
`index: [status, assignee]`.
- Same reconciler, dry-run and `unenforced` reporting as today
(`internal/store/pgstore/derivedschema.go`). Report the added index count in
`rela db` output so operators see the write cost.
- Check manual list order (TKT-AVNEX5T) against the derived sort keys.

## Acceptance criteria

- `EXPLAIN` of a list page filtered on a declared control uses its derived
index (pin with a graphquery explain test).
- Removing a control drops its index on the next reconcile.
