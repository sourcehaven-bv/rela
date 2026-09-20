---
id: TKT-HH7PKJ
type: ticket
title: 'reverse_relation migration step: rewrite stored edges when a relation type''s direction is swapped'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Reversing a relation type — deciding that `blocks` should point from feature to
ticket rather than ticket to feature — is expressible in `schema.yaml` by
swapping `from:` and `to:`, but nothing can migrate the stored edges to match.

`CompareShapes` compares the two endpoint lists independently
(`internal/metamodel/shapecompare.go:252-253`), so a swap raises **two**
unrelated `relation_endpoint_narrowed` deltas — one per side, each reporting
that an allowed type disappeared. There is no delta kind meaning "reversed", and
the classifier cannot distinguish a swap from two coincidental narrowings.

Downstream every path is a dead end. `resolvingSteps` lists the kind with an
empty value (`internal/datamigration/file.go:159`), so nothing is enforced;
`migrate gen` emits only the comment "no declarative step can fix this; write a
lua step or adjust the data by hand" (`generate.go:181-183`); and the lua step
cannot do it either, because `applyLuaPatch` accepts only `properties`, `unset`
and `content` on an **entity** (`luastep.go:180-246`) and no step iterates
relations for transformation.

The result is a gate that blocks with no way to discharge it. Nothing reports
which edges are now backwards either: `countRelationsFor`
(`internal/analysis/analysis.go:504-518`) will report cardinality violations as
a side effect of every edge being counted on the wrong side, naming a
cardinality fault rather than a direction one.

## Proposed solution: push the rewrite into the store

The rewrite is an optional STORE capability (`BulkMigrator`), type-asserted at
the call site like `HeaderReader`/`Formatter`/`HistoryReader`, with a generic
in-process fallback. pgstore and sqlitestore do it in one statement per table —
the same bulk in-place shape `RenameEntity` already uses
(`pgstore/entity.go:875`); fsstore and memstore take the fallback, because
fsstore encodes the endpoints in the FILENAME and there is no set-based rewrite
to be had.

This is not only faster. Three of the hazards below are properties of
create-then-delete and simply do not arise from an in-place `UPDATE`: a
self-edge updates to itself, a both-directions collision becomes a PRIMARY KEY
violation inside a transaction (clean refusal + rollback rather than a
half-applied loop), and — the significant one — `rel_record_id` is a column ON
the row, so the re-key PRESERVES version lineage instead of forking it. That is
exactly why #1127 made rename atomic.

The interface is named and shaped for the general job so `RenameRelationType`,
`RenameProperty` and `MapValues` can join it later; only the swap is implemented
now. Its vocabulary stays store-level — arch-lint forbids a store depending on
an application package, so the `scope:`/overlap decisions stay in
`datamigration` above the seam.

## Original proposal (superseded: in-process loop)

A `reverse_relation` step that rewrites stored edges of one relation type:

```yaml
steps:
  - reverse_relation: {type: blocks}
```

The operator hand-edits `schema.yaml` (swapping `from:`/`to:` and the
corresponding cardinality bounds); the step moves the data. Both halves stay
independently selectable — swapping the schema without the step is a detected,
blocked state rather than a silent one, and the step is valid on its own for a
type whose declaration was already corrected.

`CompareShapes` should additionally **recognise** the swap and raise it as its
own delta kind, so `migrate gen` drafts the step instead of the current "no
declarative step can fix this" comment. That is the part that turns a blocked
gate into a workflow.

## Why this is bigger than swapping two fields

**Identity.** A relation's endpoints are its address. `UpdateRelation` takes
`(from, relType, to, data)` and only `data` is mutable
(`internal/store/store.go:505-537`); there is no `RenameRelation` counterpart to
`RenameEntity`'s atomic re-key. So reversal is delete+create per edge, and on
the database backends each edge **starts a fresh version lineage** — the same
documented cost `rename_relation_type` carries (`steps.go:571-575`).

**Content-scoped edges cannot be reversed at all.** `entity.Relation` has
`FromFace` and deliberately no `ToFace` (`entity.go:260-266`): heads are
entity-level by construction, which is what makes cross-world dangling
references inexpressible. Reversing `(A@draft) --type--> (B)` would need to put
`draft` on the new head, where there is no slot for it. Worse, the tail face is
part of the key (`entity.go:302-308`), so two edges differing only in tail face
are two relations that a reversal collapses onto one key — a silent merge, not a
move. **The step must refuse a `scope: content` relation type** rather than
guess.

**Cardinality must swap with it.** `min_outgoing`/`max_outgoing` and
`min_incoming`/`max_incoming` are four independent bounds
(`internal/metamodel/types.go:1260-1263`). If the operator swaps the endpoints
but not the bounds, the reversed data violates the schema it was migrated to
satisfy. Whether the step verifies this or the classifier warns is a planning
question.

**Order properties must swap with it.** An `orderable:` relation type stores
`_order_out` / `_order_in` (`types.go:1207-1208`). Reversing the edge without
exchanging those two values leaves user-authored ordering attached to the wrong
side.

**Symmetric types are a no-op.** `symmetric: true` makes direction irrelevant,
so reversal has nothing to do; the step should say so rather than churn every
edge.

## Open questions for planning

- Should the step also rename? A reversed relation usually needs relabelling —
`blocks` reversed reads as `blocked-by` — and doing both in one step is one
delete+create rather than two. Deferred: it can be composed from
`reverse_relation` plus `rename_relation_type` in v1, at the cost of two lineage
breaks.
- Is `inverse:` relevant? It is display-only and excluded from `ShapeProjection`
entirely (`shapeprojection.go:82-92`), so editing it raises no delta. A reversal
likely wants the `inverse:` label swapped too, but that is the operator's schema
edit, not the step's business.
- How is the swap recognised? Comparing `from`/`to` for set equality against the
opposite side is the obvious test, but a partial swap (endpoints that overlap)
is ambiguous and probably should stay two narrowings.
- Self-referential types (`from` and `to` naming the same entity type) produce
no endpoint delta at all, so a swap there is invisible to the classifier and the
step must be requestable by hand.
