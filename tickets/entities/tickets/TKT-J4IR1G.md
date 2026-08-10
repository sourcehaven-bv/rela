---
id: TKT-J4IR1G
type: ticket
title: --filter CLI flag + migrate automation/validation onto predicate (TKT-7EJK4 phase 2b)
kind: refactor
priority: medium
status: in-progress
---

## Background

Phase 2b — the user-facing half of the condition-engine convergence. Builds on
phase 2a (TKT-MOCIED: the unified type adapter + `predicatefns.FromFilter`
transpiler, both merged) and Phase 1 (TKT-7EJK4: the typed predicate superset).

## Scope (IN)

1. **`--filter <expr>` CLI flag** on `list` (and any command with `--where`),
predicate-backed: entity-only Env via `predicatefns.EntityRecordType` +
`Declare`/`Bind`, compile once, evaluate per candidate. `--where` stays, routed
through `predicatefns.FromFilter` (one-time deprecation notice at the flag-parse
site, NOT per-row).
   - Fold **RR-9V6PF**: compile-time pattern validation + a Program cache so
`--filter` doesn't recompile per entity.
   - Fold **RR-G3Y70**: allow negative numeric literals in the predicate walker
(`entity.balance > -100`) — literals only, not general unary minus.
   - **RR-2Y851X**: the Program cache must be scoped to the metamodel/resolver
instance (or key on a metamodel fingerprint), NOT a process-global
`(source,type)` — `EntityRecordType` bakes the field date layout + field set
into the type.

2. **Migrate automation `when:`/`validate:` + validation `When:`/`Then:`** onto
predicate via `FromFilter` on load (transpile legacy filter-strings, compile
once, cache Program, evaluate per event/entity), with a minimal **entity-only**
Env. Preserve current typed behavior (measure
`automation-typed-comparison-test`). One-time deprecation warning per rule at
load.

## Scope (OUT)

- New host functions / extra Env context on automation/validation
(`current_user`, `has_role`, `old`/`new`) — deferred.
- **RR-02P03I**: the following filter consumers stay on `filter.Match`,
UNCHANGED, and are explicitly NOT migrated in this phase:
`internal/dataentry/{views,feed_provider,helpers}.go` (SPA view/feed `where:`),
`internal/lua/runtime.go` (script queries), `internal/search/searchparser`
(search property clauses), `internal/cli/analyze.go`. Docs must say "predicate
is the condition engine; filter still backs query-filtering in these subsystems"
— NOT "filter is fully frozen."

## Test plan

- Golden replay of this repo's real `metamodel.yaml` automations/validations
through old (filter) vs new (predicate) path — identical verdicts (the
`FromFilter` parity harness from phase 2a extends here).
- `--filter` end-to-end incl. an `or` the old `--where` can't express.

## Docs

- docs/cli-reference.md (`--filter`, `--where` deprecation), docs/metamodel.md
(`when:`/`validate:`/`When:`/`Then:` accept predicate expressions + legacy
compat), CLAUDE.md (predicate = condition engine; filter = query-filtering).

## Related

- `has-research` -> RES-6PK0S3.
- Depends on phase 2a: TKT-MOCIED (adapter + transpiler).
- Phase 1: TKT-7EJK4 / PR #1151.
