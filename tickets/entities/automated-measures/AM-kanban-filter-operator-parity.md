---
id: AM-kanban-filter-operator-parity
type: automated-measure
title: 'Test pin: every filter operator a surface''s validator accepts is one that surface can evaluate'
description: 'Cross-layer parity pin. AM-filter-operator-set-pin holds the validator to the DOCS set; it does not hold each consuming surface to the set its own validator accepts. That gap is what let BUG-MYN56J ship: the kanban validator accepts nine operators while the board evaluates three, silently ignoring the other six. This measure asserts, per surface (list, kanban), that the accepted operator set and the evaluable operator set are equal - so a fifth divergent operator set cannot open silently.'
kind: test
location: internal/dataentryconfig/validate_test.go + frontend/src/views/__tests__/KanbanView.spec.ts
status: proposed
---

# Filter operator parity pin

## What it protects against

`BUG-F1LTV0`'s why4 named the failure class:

> Four independent hand-maintained operator sets (docs table, validator map,
> SPA OPERATOR_MAP, API switch) existed with no cross-layer contract test, so
> each layer's silent-fallback behavior masked the others' gaps.

`AM-filter-operator-set-pin` closed one axis: validator set == documented set.
It does **not** close the other: *documented and accepted* is not the same as
*evaluable on the surface where it was configured*.

`BUG-MYN56J` fell straight through that gap. The kanban validator
(`internal/dataentryconfig/validate.go:1732-1746`) accepts all nine operators
from `validFilterOperators`; the board evaluates three
(`frontend/src/views/KanbanView.vue:230-241`) and its `default: return true` arm
silently ignores `~`, `in`, `<`, `<=`, `>`, `>=`.

## The assertion

For each surface that carries `filters:` — currently lists and kanbans — the set
of operators its validator accepts must **equal** the set that surface can
evaluate. Not a subset in either direction:

- an accepted-but-unevaluable operator is a silent no-op or a silent empty
result (the two BUG-F1LTV0/BUG-MYN56J failure modes)
- an evaluable-but-rejected operator is a capability the author cannot reach
(the `~`/`in`/`==` half of BUG-F1LTV0)

## Why it needs two halves

The evaluable set does not live in one language. Until a kanban has a Go read
path (see `TKT-LPLZ1V`), the board's matcher is TypeScript, so the parity check
needs a fixture both sides agree on — the operator list exported from one place
and asserted against on each side, rather than two hand-maintained literals,
which is the exact pattern that produced the bug.

If `TKT-LPLZ1V` gives kanbans a server read path and one Go matcher serves both
surfaces, this collapses to a single Go test and the TypeScript half can be
retired. Record that outcome here when it happens.

## Status

`proposed` — to be implemented with whichever of `BUG-MYN56J`'s two fix options
is taken. It is the part that must land either way: narrowing the validator
without the pin leaves the next divergence undetected.
