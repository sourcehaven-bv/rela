---
id: TKT-6I2OPG
type: ticket
title: 'Roll-up properties: read-time aggregates over related entities'
kind: enhancement
priority: high
effort: xl
started: "2026-10-27"
status: in-progress
---

## Problem

Computed properties (TKT-NLM6EH) are entity-local. A GRC product needs values
derived from related entities, shown in lists and on entity pages:

- `maatregel.bewijs_aantal`: number of linked evidence items with status `geldig`.
- `maatregel.laatste_testresultaat`: `resultaat` of the linked `controletest`
with the latest `datum`.
- `risico.open_behandelplannen`: number of linked treatment plans not `afgerond`.

Existence filters already work through `related(...)` and are out of scope.

## Decision (approved brief G1)

Roll-ups are computed when read, per viewer, and never stored.

```yaml
bewijs_aantal:
  type: integer
  rollup: { via: evidencedBy, count: true, where: { status: geldig } }
laatste_testresultaat:
  type: testresultaat
  rollup: { via: testedBy, pick: resultaat, latest_by: datum }
```

- Functions: `count`, `exists`, `min`, `max`, `latest_by` (with `pick`); `sum`
and `avg` if cheap.
- `via`: one relation name, canonical or inverse, as in `related()`.
- `where`: property equality only. No time functions.
- No chains: reject at schema load.
- Gating: the viewer's gate decides which related entities count, as in
`internal/relresolve`. Hidden `where`/`pick`/`latest_by` fields must not leak.
- Batching: one grouped query per roll-up column per page; budget test.
Gate before fold.
- Surfaces: entity read (API + detail page), list columns (opt-in
`rollups=1`), view sections, MCP `show_entity`. MCP `list_entities` summaries
carry no properties; it refuses a filter on a roll-up.
- No write-time reads (user decision, option C): automation `condition:` /
`when:` / `validate:` and transition guards do not see roll-ups. Any predicate
expression (automation, validation, ACL `when:`, transitions, actions) that
names a roll-up property is a load error: roll-ups are not part of the entity
record type predicates compile against.
- Backends: fs, memory, SQLite, PostgreSQL.

## Out of scope (backlog tickets)

- `count_related(...)` filter predicate lowered into store queries.
- Filtering on `latest_by` values.
- Sorting lists by a roll-up.
- Roll-up chains.
- Saved (materialized) roll-ups.
- Write-time reads of roll-ups (automation conditions, validations, transition
guards). Guards cannot read entity properties at all today
(`internal/statemachine/predicate.go`).
- `avg` (rela has no decimal type) and multi-hop `via` paths.

## Acceptance criteria

1. A schema with the three example roll-ups loads; a chain, an unknown
relation, an unknown property, a type mismatch (`pick` vs declared type) and a
write to a roll-up property are rejected with clear errors.
2. Entity read, list column and view section show the correct value on every
backend.
3. Two principals with different read rights on the related type see different
values; a hidden related entity is never counted.
4. A list page with N rows and K roll-up columns issues at most K extra store
queries (budget test).
5. Docs: `docs/metamodel.md` (via `docs-project/` source) describes roll-ups and
how they differ from computed properties.

## Decisions taken in planning

- **Hidden fields: exclude.** A related row the reader cannot read never
counts. A related row on which the reader cannot see a property the roll-up
reads (`where`, the `min`/`max`/`sum` property, `latest_by`) is left out. When
the reader cannot see `pick` on the latest row, the value is empty; the roll-up
does not fall back to an older row. The result is always what the reader could
compute from the rows and fields they can read, so it discloses no hidden value.
- `sum` is included; `avg` is not. `via` is a single hop.
- A computed property may not read a roll-up (this also rules out chains).
