---
id: TKT-270KRY
type: ticket
title: "Gated analyze: run validation through the requester's visible reader, annotate rules that need the whole graph"
kind: refactor
priority: high
effort: l
status: ready
description: >-
  Parent tracking ticket. `_analyze` currently runs validation over the WHOLE
  graph reading RAW entities (deliberately — `visibility.Unrestricted`, see the
  comment at internal/dataentry/app.go:560), then filters the output
  per-requester. A wire-boundary sentinel test caught that a validation issue's
  MESSAGE can quote a value the requester cannot see (e.g. custom-Lua rules that
  read other entities via rela.get_entity), leaking through the output filter.
  New model (supersedes the earlier "provenance" plan): run analyze GATED
  through the requester's visible reader, so a rule literally cannot read hidden
  data — the leak becomes impossible by construction, not filtered. A gated run
  over a partial view can produce FALSE positives (a whole-graph rule fires
  because a required neighbor is merely hidden); annotate a rule with the roles
  it is meaningful for so it is SKIPPED for principals outside them. Annotations
  are additive (default = run for everyone); gating alone closes the leak.
---

## Why gated, not provenance

The abandoned "provenance" plan (record which (entity,field) a violation read,
gate the output on it) tracked leaks post-hoc. Gating prevents them up front and
is the same "hidden = nonexistent" principle used everywhere else (DEC-ZBI39P).
The provenance machinery (recording marshaller, PR #1240 / branch
refactor/lua-entity-marshaller-factory) is NOT needed by the gated model and is
being closed unmerged; the branch stays as reference.

## The arc (each step is its own PR + child ticket)

- **1 — gate the analyze reader (THE SECURITY FIX).** Swap the data-entry
  validator's `VisibleReader` from `visibility.Unrestricted` to a ctx-gating
  reader (the `ctxRowGate` / `PolicyReader` seam already used by the view fix),
  and route the ~6 raw `svc.store` reads in `analyze.go` through a ctx-gated
  reader. `runAnalysis`/`CheckRuleFull` already take ctx, so no validator-API
  change. Remove the now-redundant `visibleAnalysisIssues` output filter and the
  earlier title/message patches. **Leak closes here, by construction.** Built-in
  whole-graph checks (Cardinality, Orphans) may false-positive for narrow
  principals — documented as known, addressed by step 2 / later.

- **2 — `roles:` annotation on validation rules (THE NOISE FIX, additive).** Add
  `roles: []string` to `metamodel.ValidationRule` (internal/metamodel/types.go)
  + parse + skip a rule whose roles the principal does not hold. Absent → run for
  all (no behavior change). Lets authors quiet the false positives on custom
  rules. Does NOT affect leak-safety (gating already owns that).

Related but separate (not children of this arc): the sync read/push field-ACL
fix, and landing the wire-boundary sentinel test (which now PROVES the gated
model — analyze reads gated, so the sentinel finds nothing).

## Semantic note (deliberate redefinition)

`_analyze` changes from "the graph's TRUE integrity state" to "the integrity
state of YOUR visible slice." Defensible: you can only act on issues about
entities you can see. This reverses the documented `Unrestricted` decision at
app.go:560 — intentionally, now that we have the role annotation to handle the
false positives that decision was avoiding.
