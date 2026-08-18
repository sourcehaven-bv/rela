---
id: TKT-8GD41J
type: ticket
title: Automation and validation conditions accept a predicate expression
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

Automation and validation conditions are *evaluated* by the predicate engine
(TKT-J4IR1G phase 2b), but they are still *authored* in filter syntax. The
source string never reaches the engine intact: `convertFromMetamodel`
(`automation/engine.go:61`) runs every `when:` clause through `filter.Parse`,
and only the resulting `*filter.Filter` is kept. The predicate path is reached
by transpiling that back via `FromFilter`.

So the date functions added in TKT-HQONQE are unreachable from a `when:`.

**It fails silently, which is the worst part.** `filter.Parse` does not reject
an expression — it mangles it:

```text
days_between(entity.due, today()) <= 7
  -> prop="days_between(entity.due, today())" op=<= val="7"
entity.status == 'todo'
  -> prop="entity.status" op== val="= 'todo'"
```

Both "parse". The property does not exist, so `matchTyped` returns
`handled=false`, the string fallback runs, and the condition simply never fires.
No error at load, no warning at eval.

## Approach

Keep `when:` byte-identical (filter syntax, transpiled through `FromFilter` —
that is the compatibility path and it works). Add a sibling key carrying an
expression that reaches `Evaluator.Compile` **unparsed**:

```yaml
on:
  entity: taak
  when: ["status=todo"]                                 # filter — unchanged
  condition: "days_between(entity.due, today()) <= 7"   # expression
```

The two AND together, mirroring how multiple `when:` clauses already combine.

**Why a second key rather than auto-detection.** Filter and expression syntax
overlap without erroring (see the probe above), so any sniffing heuristic would
guess, and guess silently. Two keys, two parsers, no ambiguity.

`Evaluator.Compile(entityType, source)` already accepts raw expression source
and caches compiled programs per (entityType, source), so the evaluation half
needs no new machinery.

## Fail loud

`convertFromMetamodel` currently **swallows** an unparseable `when:` clause
(`engine.go:61`, "safer than having no constraints at all"). It is not safer: a
dropped clause makes the automation fire on **more** entities than the operator
wrote. The cause is structural — `NewEngineFromMetamodel` returns no error, so
there is nowhere to report one.

This ticket gives it an `error` return and threads it through its two production
callers (`appbuild.go:508` inside `buildAutomation`, which already returns
`error`; and `appbuildtest/fixture.go:321`). A `condition:` that fails to
compile is then a **load-time error**, not a silent skip.

`NewEngine(automations)` (no metamodel) cannot compile a `condition:` at all —
there is no schema to build an env from. Declaring one on that path is a
construction error rather than a silent no-op.

## Scope

IN:

1. `Condition string` on `metamodel.AutomationTrigger` (`condition:`).
2. The same on the validation-rule type.
3. `automation.Trigger` carries the raw source + its compiled program.
4. Compile at load (once), AND into `matchesWhenConditions`.
5. `NewEngineFromMetamodel` returns `error`; both callers updated.
6. Docs: the key, the available functions, and when to use `when:` vs
`condition:`.

OUT:

- Migrating existing `when:` strings to expression syntax. `internal/filter` is
the query-filter DSL across 8 surfaces; `when:` stays as-is.
- `{{...}}` as an expression context for `set:`/`value:`. Separate ticket.
- Time-triggered automations — the scheduler follow-up this unblocks.
- `internal/expr` extraction (TKT-SJWC7H).

## Acceptance criteria

1. An automation `condition:` evaluating
`days_between(entity.due, today()) <= 7` fires for a due entity and not for a
distant one. — *Test: engine-level, due/not-due/overdue fixtures.*
2. `condition:` and `when:` AND together; each side failing independently
blocks the automation. — *Test: table over the four combinations.*
3. An absent `condition:` leaves behaviour byte-identical to today. — *Test:
the existing automation suite, unmodified.*
4. A `condition:` that fails to compile is a **load-time error** surfaced
through `NewEngineFromMetamodel`, never a silent skip. — *Test: bad expression →
error; and specifically NOT a fire-more-often no-op.*
5. A `condition:` on an engine built without a metamodel is a construction
error. — *Test: `NewEngine` + condition → error.*
6. Validation rules accept `condition:` with the same semantics. — *Test:
validation-level fixtures.*
7. `just test`, `just lint`, `just arch-lint`, `just coverage-check` pass.

## Notes

Compile once at load, not per event: `*predicate.Program` is immutable and safe
for concurrent `Eval`; `Evaluator` already caches. An eval error means no-match
plus a warning, matching `matchTyped`'s existing posture (`engine.go:382-387`) —
a load error is fatal, an eval error is not.

---

## Self-review findings (2026-08-18)

Three issues found and fixed while reviewing my own work, before the formal code
review returned. All three were the SAME bug class the ticket exists to
eliminate — a condition that silently does nothing — reintroduced in the
implementation.

**1. Automation eval errors were swallowed** (`8fa70c14`). `matchesCondition`
returned a bare `false` on eval failure, and the automation engine has no
logging at all, so a condition failing at eval (missing property, exhausted step
budget) meant the automation silently never fired. The doc comment claimed
"no-match plus a warning" — there was no warning. Now routed to
`Result.Warnings` via the existing channel. `*Result` is threaded through
`matches`/`matchesWhenConditions` rather than stashed on the Engine, which would
race under concurrent `Process`.

**2. Validation `when_condition:` errors were swallowed** (`719564b9`). Compiled
per entity, so a malformed expression surfaced as "no match" — indistinguishable
from "no entity qualified". The rule would check nothing and report clean: a
validation rule that silently stopped validating. Conditions now compile once
per rule before any entity is touched; a failure is a `LoadError` (the channel a
broken `lua_file:` already uses) and the rule is abandoned. A doc comment
claiming these were "caught at load by Validate" was also false — nothing
validated them.

**3. Upgrade blast radius was asserted, not measured** (`188bd089`). Making an
unparseable `when:` clause fatal is a behaviour change on a load path. Measured
rather than assumed: `filter.Parse` rejects exactly three things — empty string,
missing operator, empty property name. Odd-looking input (`a=b=c`, `spaces in
prop=x`, `status = ready`) parses fine. So the change can only newly fail a
project whose clause was ALREADY broken and silently matching nothing. Verified
against all 70 distinct `when:`/`then:` clauses in this repo's own
`schema.yaml`; every one still parses. Pinned by `TestWhenClauseCompatibility`.

Also verified directly: `-race` clean with 50 concurrent `Process` calls against
a condition-bearing automation; a condition naming an unknown entity type, or a
property absent from one of several listed types, fails at load with an error
naming the automation, the condition and the offending type.
