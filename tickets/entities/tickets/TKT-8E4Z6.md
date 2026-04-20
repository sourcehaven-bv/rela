---
id: TKT-8E4Z6
type: ticket
title: Make `rela schema --graphviz` readable for large/polymorphic metamodels
kind: enhancement
priority: medium
effort: s
status: review
---

Large or polymorphic metamodels render poorly with `rela schema --graphviz |
dot`: a relation whose source has many targets produces a fan of edges that
dominates the layout and obscures the rest.

## Scope

Two independent enhancements to `rela schema --graphviz`:

1. **`--exclude <type>` (repeatable)** — drop an entity type and its edges from the diagram.
2. **Auto-rendering of many-target relations**, applied per `(source-type, relation)` pair based on the size of the effective `to` set and whether those targets are otherwise connected:

   | Target count | Targets otherwise connected? | Rendering |
   |:--:|:--:|:--|
   | ≤ 2 | — | plain edges (unchanged) |
   | 3 or 4 | at least one target otherwise isolated | hub-bundle: `source → • → targets` |
   | 3 or 4 | all targets otherwise connected | legend entry (no edges) |
   | ≥ 5 | — | legend entry (no edges) |

"Otherwise connected" = the target has ≥ 1 edge in the final diagram other than
the edge under consideration.

## Legend format

Single `__legend` node (shape=plaintext, white fill), HTML-like TABLE:
- Header row: `Universal relations`.
- Per collapsed `(source, relation)`, two rows forming one visual block:
  1. `SIDES="LTR"`: bold source + italic relation label.
  2. `SIDES="LBR"`: italic target list, left-aligned via `<BR ALIGN="LEFT"/>`.
- Target list, auto-selected:
  - Exactly all entity types: `any entity`
  - At least `total - 2`: `any entity except X, Y`
  - Otherwise: sorted, comma-separated labels, ~2 per line.

## Acceptance criteria

1. `--exclude` drops the entity and any edges touching it; relations that still have non-excluded targets keep only those.
2. A (source, relation) pair with ≥ 5 targets produces a `__legend` row, no edges.
3. A (source, relation) pair with 3-4 targets where at least one is otherwise-isolated renders as `source → __hub_N → targets`.
4. A (source, relation) pair with 3-4 targets where all are otherwise-connected produces a `__legend` row, no edges.
5. A (source, relation) pair with ≤ 2 targets renders as plain edges.
6. Entity types whose only connections are legend-collapsed AND have no remaining edges are omitted from the DOT.
7. `--no-legend` / `--no-bundle` flags turn off the respective features; output reverts to today's.
8. A generic demo script under `scripts/` constructs a synthetic metamodel exercising all four classification buckets, runs through `| dot -Tpng`, and asserts the PNG is non-empty.
9. Existing `TestSchemaGraphviz*` tests pass unchanged.

## Out of scope

- `-o` / `-f` rendering parity with `rela graph` (separate ticket).
- Alternative layout engines, cluster attributes, `concentrate=true`.
- Changes to `rela graph`.
