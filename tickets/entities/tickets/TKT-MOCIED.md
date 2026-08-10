---
id: TKT-MOCIED
type: ticket
title: Unify metamodel->predicate type adapter + filter->predicate transpiler (TKT-7EJK4 phase 2a)
kind: refactor
priority: medium
status: done
---

## Background

Phase 1 (TKT-7EJK4, merged #1151) extended `internal/predicate` into a typed
superset of `internal/filter`: Int/Date value types, compile-time literal
coercion, and `internal/predicatefns` (host funcs + the `EntityRecordType`
adapter). Phase 2 converges the condition evaluators on predicate. It is
delivered in two PRs:

- **This ticket (phase 2a):** the two foundational, non-user-facing slices —
unify the type adapter, and build the filter->predicate transpiler.
- **Follow-up TKT-J4IR1G (phase 2b):** the user-facing slices — `--filter` CLI
flag + migrate automation/validation onto predicate.

## Scope (this ticket — phase 2a)

### Slice 1 — Unify the metamodel->predicate type adapter (DONE)

Before Phase 2, predicate had two divergent metamodel->type adapters:
`predicatefns.EntityRecordType` (integer->Int, date->Date; production-unused)
and `affordances.scalarPredicateType` (integer->Number, date->String; live on
the ACL affordance `when:` path). This slice makes the scalar type CHOICE a
single source of truth in `predicatefns.ScalarType`/`ScalarTypeForProp`, and
migrates the live `affordances` path onto it (binder now emits
`NewInt`/`NewDate` handling both the `time.Time` and string YAML shapes). Closes
RR-TBG91.

### Slice 2 — filter->predicate transpiler (DONE)

`predicatefns.FromFilter(meta, def, *filter.Filter)` transpiles the legacy
`--where` / metamodel `When:`/`Then:` filter syntax to a predicate source
expression. Lives in predicatefns (not filter — cycle) with an injection-safe
Lua-string escaper; unsupported cases (fuzzy-with-wildcard, list ordered ops)
error rather than silently diverge. Cross-engine parity verified against
`filter.Match`.

## Scope (OUT — moved to phase 2b, TKT-J4IR1G)

- `--filter` CLI flag (predicate-backed) + Program cache (RR-2Y851X, RR-9V6PF).
- Migrate automation `when:`/`validate:` + validation `When:`/`Then:` onto
predicate.
- Negative-literal grammar fix (RR-G3Y70).
- Naming the filter consumers that stay on `filter.Match` (RR-02P03I).

## Verification

Full build + arch-lint clean; race tests green across predicate, predicatefns,
affordances, dataentry (ACL read path), statemachine, conditionlint, filter;
lint 0 issues. (docscapture browser tests fail locally on a missing headless
Chrome — unrelated, passes in CI.)

## Related

- `has-research` -> RES-6PK0S3.
- Phase 1: TKT-7EJK4 / PR #1151 (merged dd60f475).
- Phase 2b follow-up: TKT-J4IR1G.
