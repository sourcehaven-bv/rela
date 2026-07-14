---
id: TKT-R8QEU
type: ticket
title: 'Add native relation-cardinality support to validation rules (relations: block on ValidationRule)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Validation rules cannot express "entity must have min/max linked entities of
relation type R, optionally filtered by the target's own properties". The
`relations:` block used throughout `tickets/metamodel.yaml` is silently dropped
because `ValidationRule` (`internal/metamodel/types.go`) has no such field — the
key is discarded at YAML parse time and the gate becomes a no-op.

As a stopgap, TKT-IFHO2L ported the 14 affected workflow gates to a
`require-relation-count.lua` validator. This ticket replaces that stopgap with
first-class engine support so the rules stay declarative and a future typo fails
loudly instead of silently.

## Scope

1. Add a `Relations map[string]RelationConstraint` field to `ValidationRule`
with `min` / `max` / `where` (target-property filters reusing the existing
`--where` predicate parser).
2. Evaluate it in `internal/analysis` alongside `when` / `then`, emitting an
error/warning per the rule severity.
3. Make the metamodel loader **reject unknown keys** inside a validation rule so
a future typo fails loudly instead of silently (this is the real root cause of
the original bug).
4. Migrate the 14 Lua-ported gates back to the declarative `relations:` form and
delete `tickets/validations/require-relation-count.lua`.
5. Conformance tests: a `done` ticket without a completed `has-review` must
produce an error; a strict-loader test for the unknown-key rejection.

## Context

Follow-up to TKT-IFHO2L, which restored the gates via Lua with no engine risk.
This is the larger Go change (metamodel + analysis + loader strictness) with its
own test surface, deferred deliberately.

## Acceptance Criteria

- [ ] `relations:` block on a validation rule is parsed and evaluated (min/max/where).
- [ ] Metamodel loader rejects unknown keys within a validation rule.
- [ ] The 14 gates are migrated back to declarative form; the Lua stopgap deleted.
- [ ] `rela validate` behaviour is unchanged from the Lua stopgap (same violations).
- [ ] Conformance + strict-loader tests added; `just ci` green.
