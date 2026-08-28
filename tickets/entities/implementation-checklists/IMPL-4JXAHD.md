---
id: IMPL-4JXAHD
type: implementation-checklist
title: 'Implementation: Automation and validation conditions accept a predicate expression'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Integration coverage is at the real seam, not just unit level:
`TestEngine_Condition_DateArithmetic` and `TestEngine_Condition_AndsWithWhen`
drive the automation engine end to end; `TestDateFuncs_ThroughEvaluator`
exercises `Evaluator.Compile` + `EntityRecord` against a metamodel-bound entity
— the path a date property binding as `String` would fail on.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Fixtures use the existing `testutil.Entity(...).With(...)` builder and
`buildEntity`. `condAutomation` / `datePropMeta` / `pinClock` are local
factories so each case states only what it varies. `setVar` collapsed 19
repeated bind-and-check blocks (also removed the govet shadow warnings).

The clock is pinned via `NewEvaluatorWithClock` so `today()` cannot drift with
the calendar — without it every date fixture would rot.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Evidence |
|----|----------|
| 1 — condition fires on date arithmetic | `TestEngine_Condition_DateArithmetic`, 6 fixtures (due today / 3d / exactly 7d / 8d / far / overdue) |
| 2 — `condition:` ANDs with `when:` | `TestEngine_Condition_AndsWithWhen`, all four combinations |
| 3 — absent condition unchanged | `TestEngine_NoCondition_Unchanged` + the whole pre-existing automation suite green against the new signature |
| 4 — broken condition is a load error | `TestEngine_Condition_CompileErrorIsFatal`, 5 shapes (syntax, unknown fn, unknown property, non-boolean, wrong arg type); each error names the automation |
| 5 — condition without metamodel errors | `TestEngine_Condition_RequiresMetamodel`, `TestEngine_Condition_RequiresEntityType` |
| 6 — validation accepts conditions | `TestWhenCondition`, `TestThenCondition`, `TestConditionAbsent_Unchanged` |
| 7 — gates | `go test ./...` green; `golangci-lint` 0 issues; `just arch-lint` clean; `just coverage-check` PASS |

Verified beyond the ACs, by direct experiment rather than inspection:

- **Concurrency**: `-race` clean with 50 concurrent `Process` calls against a
condition-bearing automation.
- **Multi-type conditions**: an automation spanning `[taak, bug]` where `due`
exists only on `taak` fails at load naming the offending type; so does an
`entity:` naming an undeclared type.
- **Upgrade impact**: `filter.Parse` rejects exactly three shapes (empty,
missing operator, empty property). All 70 distinct `when:`/`then:` clauses in
this repo's own `schema.yaml` still parse. Pinned by
`TestWhenClauseCompatibility`.
- **Saturation**: `9999-01-01` minus `1000-01-01` now returns 3286817 days,
not the saturated 106751.
- **Documented guard**: `TestEngine_Condition_NilGuard` proves the
`entity.due ~= nil and …` form the docs recommend gives a clean false with no
warning, and does not break the normal path.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Patterns followed**: `condition:` mirrors the existing `when:` shape;
`LoadError` reuses the channel a broken `lua_file:` already uses;
`matchCondition` sits beside `matchFilters` with the same signature; RRULE
mechanics live beside `ValidateRrule` in `metamodel` rather than adding a vendor
edge to `predicatefns` (caught by `just arch-lint`).

**DRY**: `utcDay` is the single place the UTC convention is applied, so
`today()`, `days_between` and `date_add` cannot drift apart. `setVar` removed 19
duplicated blocks. Deliberately NOT extracted: the three host functions keep
their own arg-validation preambles — they differ in arity and types, and a
shared validator would obscure each signature.

**Security**: no new secrets, auth, crypto or file access. Expression source is
operator-authored config (root CLAUDE.md), the same trust boundary as the
existing `lua:` action, which is strictly more powerful. The sandbox is
inherited unchanged — no I/O at eval, allow-listed AST, step/depth budgets. The
one eval-time parse (`rrule_next`) runs a validated rule and returns a single
occurrence, never an unbounded expansion.

**No silent failures — this was the main defect found and fixed**, three times
over, each an instance of the exact bug this ticket exists to remove:

1. Automation eval errors returned a bare `false` with no logging anywhere in
the engine → now surfaced via `Result.Warnings` (`8fa70c14`).
2. Validation `when_condition:` compile failure read as "no entity selected", so
a rule silently stopped validating and reported clean → now a `LoadError` with
the rule abandoned (`719564b9`).
3. `days_between` saturated silently past ±292 years, and `date_add` truncated
fractions and wrapped on overflow → both now exact or an explicit error
(`8de85520`).

Two doc comments asserted behaviour the code did not have ("no-match plus a
warning"; "caught at load by Validate"). Both corrected rather than left to
mislead the next reader.

**No debug code**: all probe files removed; `git status` clean.
