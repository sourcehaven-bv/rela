---
id: TKT-SJWC7H
type: ticket
title: Extract internal/expr from internal/predicate (general typed expression engine + ExpectType)
kind: refactor
priority: medium
effort: m
status: ready
---

## Problem

`internal/predicate` is no longer a predicate engine. It is a general typed
expression engine — own compiler, typed IR, `Value` sum type over
Bool/Number/String/Date/Record/List, `DeclareFunc` with typed signatures,
linter, fuzz tests — with a **single-line boolean restriction bolted on at the
door**:

```go
// internal/predicate/compile.go:109
if !root.resultType().equalsType(BoolType) {
    return nil, &CompileError{Reason: "top-level expression must be bool, got " + ...}
}
```

Everything below that line is already general: `Program.Eval` returns `(Value,
error)`, not `(bool, error)`. The name says "predicate" but the thing is an
expression engine whose Bool case happens to be the only one exposed.

Separately, **automation `when:` conditions cannot do date arithmetic.** The
automation engine evaluates conditions through `filter.Match`
(`internal/automation/engine.go:381`), a `key op value` string DSL with no
functions and no composition. So a rule cannot express "due within N days" or
"next occurrence of this RRULE" — the exact predicates a recurring-task
automation needs.

Meanwhile `internal/predicatefns` already ships `today()` (DateType, UTC-
truncated, `now` injected for purity — RR-YPYTP), `match`, `regex`, `fuzzy`, and
`contains`. **`Declare`/`Bind` have zero production callers.** The stdlib is
built, documented, tested, and unwired.

## Scope

**In scope**

1. **Move** the general engine to `internal/expr`: `value.go`, `ir.go`,
`env.go`, `bindings.go`, `compile.go`, `eval.go`, `walk.go`, `preprocess.go`,
`dateparse.go`, `errors.go`, `program.go` (~1,950 LOC non-test).
2. **Keep** `internal/predicate` as the Bool-typed facade: `Compile` delegating
to `expr.Compile(..., expr.ExpectType(expr.BoolType))`, `Eval` returning a plain
`bool`, plus `lint.go` (predicate-specific author-time check that
`conditionlint` builds on) and the Lua-equality-semantics `doc.go`. Type aliases
(`type Env = expr.Env`, etc.) keep the five importing packages compiling
unchanged.
3. **Add `ExpectType`** as a `CompileOption` (the mechanism already exists) so
the top-level type assertion is parameterized rather than hardcoded. Default
stays Bool — no behaviour change for existing callers.
4. **Add date/RRULE host functions** to the stdlib: `date_add(d, n, unit)`,
`days_between(a, b)`, `rrule_next(rule, after)`. `rrule_next` validates its
literal at compile time via the existing `metamodel.ValidateRrule`.
5. **Wire the stdlib into automation `when:`** so conditions can call
`today()`, the date functions, and the existing string matchers.

**Out of scope** (deliberately deferred)

- `{{...}}` as an expression context for `set:`/`value:`. Today it is pure
variable substitution; making it evaluate expressions touches every existing
automation and needs its own design decision (`{{ }}` vs a distinct `{{= }}`
syntax). Separate ticket.
- Any scheduler / time-triggered-automation work. That is the follow-up this
ticket unblocks.
- Renaming `internal/predicatefns`. It is the metamodel-to-engine adapter that
keeps the engine schema-free; the name can follow later if it grates.
- Binary arithmetic operators (`+`/`-` on dates). Functions are preferred:
`Date + Number` is ambiguous (days? months? — calendar arithmetic is a policy
decision), and operators would tempt eval-time coercion that breaks RR-A3EZR.

## Constraints carried forward

- **RR-A3EZR** — date parsing happens at compile time, never at Eval. Host
functions compute over already-parsed `Date` values; they must not parse.
- **RR-N176T** — ReDoS safety rests on RE2. Do not add a matcher using anything
but stdlib `regexp`.
- **RR-YPYTP** — `today()` is UTC-truncated to match how date literals parse.
New date functions must use the same convention or they will skew by a day.
- **arch-lint** — `internal/expr` inherits predicate's boundary: it must not
import `internal/metamodel`. `predicatefns` holds that import today and keeps
it. Needs an explicit `.go-arch-lint.yml` rule for the new package.
- `RES-6PK0S3` decided **filter = data-shaping, predicate = policy**, keeping
two evaluators on purpose. This ticket does not overturn that: it renames the
policy engine honestly and lends automations its function layer. Whether
automation `when:` should migrate off `filter.Match` entirely is a *consequence
to evaluate*, not a premise — see the open question.

## Acceptance criteria

1. `internal/expr` exists and holds the general engine; `internal/predicate`
holds only the Bool facade + lint + doc.
2. The five importing packages (`affordances`, `conditionlint`, `metamodel`,
`predicatefns`, `statemachine`) compile with no call-site changes.
3. `expr.Compile(env, src)` with no option still rejects a non-Bool top-level
expression; `ExpectType(DateType)` accepts a Date-returning expression and
rejects a Bool one, both at **compile** time.
4. `predicatefns.Declare`/`Bind` gain `date_add`, `days_between`, `rrule_next`
with typed signatures; a bad RRULE literal fails at compile.
5. An automation `when:` can evaluate `days_between(entity.due, today()) <= 7`
and produce the correct result for due/not-due fixtures.
6. `just arch-lint`, `just test`, `just lint`, `just coverage-check` pass.
7. No behaviour change for any existing predicate caller — pinned by the
existing suites in `affordances`, `conditionlint`, `statemachine`.

## Open question for planning

Wiring functions into `when:` means automation conditions are evaluated by two
engines depending on syntax, or `filter.Match` conditions get migrated to the
expression engine. The first is simpler but means two condition dialects; the
second is cleaner but is a user-facing change to existing `when:` strings and
re-opens the `RES-6PK0S3` boundary. **Decide this in planning before writing
code** — it is the main design risk in the ticket.

## Why now

The recurring-task case (`atlas/scripts/recurrence.lua`, 152 lines) is a
hand-written interpreter for data that is *already declarative*: `terugkerend`
declares `herhaling` as a validated `rrule`, `modus`/`status` as enums, and
typed dates. The script exists largely because `when:` cannot compare dates.
Closing that gap is the prerequisite for expressing recurrence declaratively —
which in turn makes a scheduled job's ACL access surface derivable from config
instead of buried in Lua.

---

## RE-SCOPED 2026-08-18 (after rebasing onto origin/develop @ 3a735757)

`origin/develop` overtook roughly half this ticket. **TKT-J4IR1G phase 2b**
(commit `6db1f2c0`, PR #1315) migrated automation and validation onto the
predicate engine and shipped the glue this ticket was going to build:

| Original scope item | Status |
|---|---|
| Wire the stdlib into automation `when:` | **DONE upstream** — automation + validation evaluate through predicate |
| A binder congruent with `EntityRecordType` (finding RR-2I7JHJ / S2) | **DONE upstream** — `predicatefns.EntityRecord` (`bind.go`) + `Evaluator` with a compile-once Program cache |
| `today()` wired into conditions | **DONE upstream** — per-evaluation, via `NewEvaluatorWithClock` |
| Legacy `when:`/`then:` compatibility | **DONE upstream** — `predicatefns.FromFilter` transpiles filter strings on load |
| `NewEngineFromMetamodel` error return (RR-K6IIIW / C3) | **MOOT** — superseded by the upstream wiring |

The new root `CLAUDE.md` section "Condition engine: `internal/predicate` +
`internal/predicatefns`" now records the architecture: predicate is the
condition/policy engine (affordances, statemachine, conditionlint, automation,
validation, CLI `--filter`); `internal/filter` remains the query-filtering DSL.
That is the boundary this plan argued for — settled upstream.

**Remaining scope for THIS ticket** (unchanged and still valid):

1. Extract `internal/expr` from `internal/predicate` — `internal/expr` does not
exist; `compile.go:109` still hardcodes the Bool restriction.
2. `ExpectType` compile option so expressions can return non-Bool values.

Findings **RR-5KYLPZ (C1)**, **RR-SEJ9UU (C2)** and **RR-O2AV40 (C4)** survive
intact and still govern the work: `Program` stays a type alias, `Eval` keeps
returning `(Value, error)`, Bool ergonomics arrive via an additive `EvalBool`,
and `ExpectType` is restricted to scalar types with its argument validated at
`Compile` entry.

**Split out:** date arithmetic (`days_between`, `date_add`, `rrule_next`) moved
to **TKT-HQONQE** — it is now small and additive against the upstream wiring,
and it is the piece that actually unblocks declarative recurrence.

**Superseded finding:** RR-Y0N50P (S1, `new`/`old` vs `entity`) is no longer a
free choice — upstream shipped an env declaring `entity`. Revisiting it would be
a breaking change to released config, so it is deferred rather than addressed.
