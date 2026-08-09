---
id: TKT-MOCIED
type: ticket
title: 'Converge condition evaluators on predicate: --filter CLI, unify type adapter, migrate automation/validation (TKT-7EJK4 phase 2)'
kind: refactor
priority: medium
status: in-progress
---

## Background

Phase 1 (TKT-7EJK4, merged #1151) extended `internal/predicate` into a typed
superset of `internal/filter`: added Int/Date value types, compile-time literal
coercion, and `internal/predicatefns` (host funcs
match/regex/fuzzy/contains/today + the `EntityRecordType`
metamodel→predicate.Env adapter). Phase 2 is the convergence — make predicate
the one condition engine.

## Current landscape (re-surveyed on this branch, post-merge)

predicate now has **four** consumers using **two divergent
metamodel→predicate.Type adapters**:

- `internal/predicatefns.EntityRecordType` / `scalarPredicateType` (env.go:38,54) — Phase 1's adapter: integer→**IntType**, date→**DateTypeWithLayout**, boolean→BoolType. Binds NewInt/NewDate. **Production-unused today** (only Phase 1 tests).
- `internal/affordances.scalarPredicateType` (env.go:104-123) — pre-Phase-1: integer→**NumberType**, date→**StringType**. Binds Number/String. **Live** (ACL affordance `when:` path).
- `internal/statemachine` — transition `When:` guards already evaluate through predicate (predicate.go:19-83); hand-rolls its own Env.
- `internal/conditionlint` — author-time lint reusing predicate's parser/type-checker for wizard `visible_when`/`required_when` (compile-time only; runtime is the SPA TS engine). Hand-rolls its own Env.

Still pure-`filter` (the migration targets):
- `internal/automation/engine.go` — `on.when:` via `filter.Parse` (engine.go:59-68); `validate:`/`check` via `filter.Parse`+`matchProperty`→`filter.Match`/`filter.MatchValue` (engine.go:323-406).
- `internal/validation/validation.go` — metamodel `When:`/`Then:` via `filter.ParseAll`+`filter.MatchAll` (validation.go:163-286).
- `internal/cli/list.go` — `--where` flag → `filter.ParseAll`/`filter.MatchAll` (list.go:17,91,102). **No `--filter` flag and no CLI predicate usage exist yet** (greenfield).

`conditionlint`/`statemachine`/`affordances` each build their **own**
`predicate.Env`; none use `predicatefns.Declare`/`Bind`/`EntityRecordType`.

## Scope (IN)

1. **Unify the type adapter (prerequisite).** Make `predicatefns.EntityRecordType` the single metamodel→predicate.Type adapter. Migrate `affordances` off its own `scalarPredicateType` (integer→Number, date→String) onto it (integer→Int, date→Date), switching its binder to NewInt/NewDate. Route statemachine's Env through the shared adapter too where it declares entity fields. This closes RR-TBG91 and makes `predicatefns` actually used in production. **Highest-risk item — do first** (RR-4189H: binding Number to an IntType field fails the runtime type check, so adapter + binder must change together).
2. **New `--filter <expr>` CLI flag** on `list` (and any command with `--where`), predicate-backed, entity-only Env + predicatefns host funcs. `--where` stays.
3. **Freeze `--where`/legacy metamodel filter-strings as transpiled aliases.** A `filter`→predicate transpiler; `--where` and ValidationRule `When:`/`Then:` accept the old strings, transpiled on parse, with a one-time deprecation warning. Values map to typed literals via declared type (NOT always-string — preserve the typed comparison automations already have). The empty/missing-value transpiler target is pre-verified in `predicatefns/parity_missing_test.go`.
4. **Migrate automation `when:`/`validate:` and validation `When:`/`Then:`** to evaluate through predicate (compile once, cache Program, evaluate per event/entity), each with a minimal **entity-only** Env, preserving current typed behavior (measure `automation-typed-comparison-test`).

## Scope (OUT)

- New host functions / extra Env context on automation/validation (`current_user`, `has_role`, `old`/`new`) — deferred (unchanged from Phase 1 scope).
- Migrating `conditionlint` beyond keeping its Env congruent with the unified adapter (its runtime is the SPA TS engine; low value).
- SQL pushdown of filters (confirmed non-issue in Phase 1).

## Deferred Phase-1 review items to fold in

- **RR-9V6PF** (host-fn regex/glob compiled per-Eval, no cache; compile-time pattern validation) — the CLI `--filter` compile-once path is the natural home for a Program-level pattern cache.
- **RR-G3Y70** (negative numeric literals rejected by the grammar — `entity.balance > -100`) — a walker/grammar change; now user-visible via `--filter`.
- **RR-XJBGB** (enum RHS validation gap — plain-string enums don't validate membership) — revisit whether to carry allowed-values into the type.

## Related

- `has-research` → RES-6PK0S3 (the "keep two vs converge" decision this completes).
- Phase 1: TKT-7EJK4 / PR #1151 (merged dd60f475).
- Consolidation flagged in-code at predicatefns/env.go:28-37 (RR-TBG91).
