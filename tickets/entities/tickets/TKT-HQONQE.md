---
id: TKT-HQONQE
type: ticket
title: 'Date arithmetic for condition expressions: days_between, date_add, rrule_next'
kind: enhancement
priority: medium
effort: s
status: review
---

## Problem

Automation and validation conditions now evaluate through the predicate engine
(TKT-J4IR1G phase 2b, PR #1315), and the `predicatefns` stdlib ships
`match`/`regex`/`fuzzy`/`contains`/`len`/`today`. But there is **no date
arithmetic**, so a condition cannot express:

- "due within N days" — `days_between(entity.due, today()) <= 7`
- "the next occurrence of this RRULE" — `rrule_next(entity.herhaling, entity.volgende_datum)`

`today()` returns a `DateType` and date properties bind as `Date`
(`predicatefns.EntityRecord`), so the two ends exist — only the arithmetic
between them is missing.

## Why it matters

`atlas/scripts/recurrence.lua` is ~152 lines of hand-written Lua interpreting
data that is *already declarative*: `terugkerend` declares `herhaling` as a
validated `rrule`, `modus`/`status` as enums, and `volgende_datum` /
`laatste_aangemaakt` as typed dates. The script exists largely because a
condition cannot compare dates. Closing this gap is the prerequisite for
expressing recurrence declaratively.

## Scope

IN — three host functions in `internal/predicatefns`:

| Function | Signature |
|---|---|
| `days_between(a, b)` | `(Date, Date) -> Number` |
| `date_add(d, n, unit)` | `(Date, Number, String) -> Date` |
| `rrule_next(rule, after)` | `(String, Date) -> Date` |

OUT:

- `internal/expr` extraction and `ExpectType` — stays on TKT-SJWC7H.
- Binary `+`/`-` operators on dates. `Date + Number` is ambiguous (days?
months? calendar arithmetic is a policy decision) and operators tempt eval-time
coercion that would break RR-A3EZR. Functions state the unit.
- `{{...}}` as an expression context for `set:`/`value:`. Separate ticket.
- Time-triggered automations. The scheduler follow-up this unblocks.

## Design constraints

- **RR-YPYTP (timezone).** `today()` is truncated to **UTC** midnight, not
local, to match how date literals parse (`parseDateLiteral` yields UTC when the
layout carries no zone). `days_between` and `date_add` MUST use the same
convention or results skew by up to a day.
- **RR-A3EZR (no parse at eval).** Date parsing happens at compile time. These
functions compute over already-parsed `Date` values. `rrule_next` is the single
exception — it parses an RRULE *string* at eval, because the rule may come from
an entity property and could never be compile-validated. It must return a single
occurrence, never an unbounded expansion.
- **`date_add` unit is restricted to `day` and `week` for v1.** Go's `AddDate`
normalizes Jan 31 + 1 month to Mar 2/3, which is exactly the kind of silent
policy decision the function-over-operator choice exists to avoid. `month` and
`year` are rejected with a clear error; they can be added later with explicit
clamp-to-end-of-month semantics.
- **RRULE validation is eval-time**, via the existing `metamodel.ValidateRrule`
(`internal/metamodel/rrule.go:17`, already used by the Lua binding). There is no
per-argument compile-time hook on `FuncSig` — it type-checks types, not values.
- Eval errors mean **no match** plus a logged warning, matching the automation
engine's existing posture; they never fail a write.

## Acceptance criteria

1. `Declare` and `Bind` register all three functions with typed signatures.
— *Test: signature assertions; Declare/Bind agree.*
2. `days_between(a, b)` returns whole days, correctly signed (negative when `a`
is before `b`), UTC-truncated on both sides. — *Test: table incl. 0, ±1, a large
span, and a local-vs-UTC boundary case.*
3. `date_add(d, n, 'day')` and `'week'` work for positive and negative `n`;
`'month'`/`'year'`/garbage are rejected with a clear error. — *Test: table incl.
month-end and a leap day.*
4. `rrule_next(rule, after)` returns the next occurrence strictly after
`after`; a malformed rule is an eval error naming the rule; an exhausted rule
(e.g. `COUNT` reached) returns a documented value. — *Test: daily/weekly/monthly
rules, malformed rule, exhausted rule.*
5. An automation `when:` can evaluate `days_between(entity.due, today()) <= 7`
end-to-end through the engine. — *Test: due / not-due fixtures via the
automation engine.*
6. `just test`, `just lint`, `just arch-lint`, `just coverage-check` pass.

## Notes

`internal/predicatefns` may already import `metamodel` and `filter`
(`.go-arch-lint.yml`), so `ValidateRrule` needs no new arch edge. `rrule-go` is
already vendored and used by `metamodel/rrule.go` and `lua/date.go`.

---

## Implementation note (2026-08-18)

Delivered on branch `tkt-hqonqe-date-arithmetic`, commit `c1f54fa9`.

**Where the RRULE logic landed.** `internal/predicatefns` may not import
`rrule-go` (`.go-arch-lint.yml` — caught by `just arch-lint`), so occurrence
stepping went into **`metamodel.NextRrule`**, beside the existing
`ValidateRrule`. That keeps RRULE mechanics in one package with identical prefix
handling and error text, and leaves `predicatefns` depending only on
`metamodel`. `predicatefns.ErrRruleExhausted` re-exports the sentinel.

**Exhausted rules are an error, not a nil or zero date.** The engine enforces
declared return types (`predicate/eval.go:156`), so a `Date`-returning host
function cannot return `Nil` — the first design attempt failed its own test. A
zero date would be worse: a real comparable value that no caller could
distinguish from a far-future one. Malformed and exhausted rules carry distinct
messages so an operator can tell a typo from a finished schedule.

**`predicate.EvalError` flattens host errors into a message string**
(`predicate/errors.go:41`), so `errors.Is` does not survive `Eval`. The sentinel
is therefore asserted through the exported `predicatefns.RruleNext` helper, and
the through-`Eval` path is pinned separately on message content.

## AC5 could not be met as written — deferred

The stated criterion was an automation `when:` evaluating
`days_between(entity.due, today()) <= 7` end-to-end. **That is not reachable
today.** TKT-J4IR1G phase 2b migrated the automation/validation *evaluation
engine* onto predicate, but the *authoring syntax* is still filter strings:
`convertFromMetamodel` (`automation/engine.go:61`) runs every `when:` clause
through `filter.Parse`, and the predicate path is reached only by transpiling
those clauses via `FromFilter`. A raw expression in `when:` parses as a property
filter and never matches.

Substituted with an integration test at the real seam —
`TestDateFuncs_ThroughEvaluator` compiles each expression through
`predicatefns.Evaluator.Compile` and evaluates it against an entity whose
date/rrule properties were bound by `EntityRecord` from the metamodel. That is
the same path automation and validation conditions take, and it is the check
that matters: a date property binding as a `String` would compile fine and fail
only at eval, in production.

Making `when:` accept raw expressions needs a `condition:` key on
`AutomationTrigger` — separate work, and the natural follow-up.

## Verification

- `go test ./...` — full suite green
- `golangci-lint run` on both changed packages — clean
- `just arch-lint` — clean (after moving the rrule dependency)
- `just coverage-check` — PASS (metamodel 83.7%, predicatefns 78.9%)
- `./scripts/generate-docs.sh` — `docs/cli-reference.md` regenerated from the
`docs-project/` source, as CI requires

Edge cases pinned by tests: negative and zero spans, month/year/leap-day
boundaries, a DST-crossing span, a non-UTC input date, time-component
truncation, malformed/exhausted rules, wrong argument types and arity, and a
missing date property.
