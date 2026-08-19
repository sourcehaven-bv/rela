---
id: REV-LFLBR8
type: review-checklist
title: 'Review: Automation and validation conditions accept a predicate expression'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` green (full suite, including every pre-existing automation and
validation test against the new `NewEngineFromMetamodel` signature).
`golangci-lint` 0 issues across all four changed packages. `just arch-lint`
clean — it caught a real boundary violation during implementation
(`predicatefns` may not import `rrule-go`), which is why the RRULE mechanics
live in `metamodel.NextRrule`. Coverage PASS: automation 86.8%, validation
90.3%, predicatefns 79.9%, metamodel 83.7%.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5KYLPZ, RR-SEJ9UU, RR-K6IIIW, RR-O2AV40, RR-Y0N50P,
RR-2I7JHJ, RR-FTEW47 (design review, on the parent plan) and RR-INR4Q5,
RR-BGXMWV, RR-Y151OZ, RR-7Q4DZQ, RR-F6092Z (code review). All `addressed`.

The code review found one **wrong-answer bug** I had missed: `days_between`
computed its span via `time.Duration`, an int64 nanosecond count that saturates
at ~292 years. `9999-01-01` minus `1000-01-01` returned 106751 days instead of
3286817 — a birthdate, or a zero-valued year-1 date, silently produced a
plausible-looking wrong number (RR-INR4Q5).

Three further defects were found in self-review before the agent returned, all
instances of the same class this ticket exists to remove — a condition that
silently does nothing. See the ticket body.

Two claims from the review were checked and **not** adopted as-is:

- The suggested DST test fix still did not discriminate. I searched for
endpoints where UTC and local truncation actually disagree (5 vs 4) and used
those (RR-Y151OZ).
- The `SetMetamodel` data race is real and reproduced, but pre-existing and
currently unreachable (no production caller). Filed as **BUG-K3T3SR** rather
than fixed here — it is not this ticket's change.

**Unrelated changes**: none. The diff is confined to the four packages plus
their docs. One incidental fix (`AM-date-property-write-roundtrip.md` YAML
frontmatter) was made earlier because it blocked all entity creation; it is in
its own commit and was superseded upstream.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all seven PASS. Per-criterion evidence table is in
IMPL-4JXAHD; summary:

| AC | Status | Evidence |
|----|--------|----------|
| 1 date arithmetic in a condition | PASS | `TestEngine_Condition_DateArithmetic` (6 fixtures) |
| 2 ANDs with `when:` | PASS | `TestEngine_Condition_AndsWithWhen` (4 combinations) |
| 3 absent condition unchanged | PASS | `TestEngine_NoCondition_Unchanged` + full pre-existing suite |
| 4 broken condition is a load error | PASS | `TestEngine_Condition_CompileErrorIsFatal` (5 shapes) |
| 5 no metamodel → construction error | PASS | `TestEngine_Condition_RequiresMetamodel`, `..._RequiresEntityType` |
| 6 validation accepts conditions | PASS | `TestWhenCondition`, `TestThenCondition`, `TestConditionAbsent_Unchanged` |
| 7 gates | PASS | see Automated Checks |

Beyond the ACs: `-race` clean under 50 concurrent `Process` calls; the
motivating case is pinned by `TestAtlasRecurrenceConditionIsExpressible`, which
expresses the guard from the top of `atlas/scripts/recurrence.lua` — ~30 lines
of hand-rolled Lua — as one compiling, evaluating condition.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-O1DQ98

Includes the upgrade note the review asked for (RR — S3): the repo has no
CHANGELOG (release notes come from commits via GoReleaser), so it went into
`docs/metamodel.md` beside the fail-loud behaviour, covering what changed, why
it can only affect an already-broken clause, and how to read the error.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Each commit states the defect and the reasoning, not just the edit — e.g. why
`Int` rather than `Number`, why Unix seconds rather than `time.Duration`, why
`*Result` is threaded rather than stored on the Engine.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1380

Single PR covering both stacked tickets (TKT-HQONQE then TKT-8GD41J): the
condition work builds directly on the date functions, and the second is
untestable without the first.
