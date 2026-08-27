---
id: TKT-03HCWT
type: ticket
title: Derive PostgreSQL indexes from static pushed-down query predicates
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Extend the existing static property-filter pushdown so PostgreSQL derived-schema
reconciliation creates indexes matching the pushed GraphQuery predicate shapes.
Scope is limited to statically configured queries already compiled by the
data-entry/next-action path. Runtime/ad-hoc query observation, automatic
computed-property synthesis, and general cost-based indexing are out of scope.

The implementation must derive deterministic index specifications from validated
configuration, reconcile only Rela-owned indexes, preserve fs/mem behavior, and
prove with EXPLAIN-backed PostgreSQL tests that representative
type-plus-property queries use the generated index.
