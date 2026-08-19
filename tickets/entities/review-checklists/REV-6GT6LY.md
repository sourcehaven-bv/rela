---
id: REV-6GT6LY
type: review-checklist
title: 'Review: Date arithmetic for condition expressions: days_between, date_add, rrule_next'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` green; `golangci-lint` 0 issues; `just arch-lint` clean;
coverage PASS (predicatefns 79.9%, metamodel 83.7%).

`arch-lint` earned its keep here: it rejected a direct `rrule-go` import in
`predicatefns`, which is why occurrence-stepping lives in `metamodel.NextRrule`
beside the existing `ValidateRrule` — one implementation, identical prefix
handling and error text, no second package depending on the rrule library.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-INR4Q5 (critical), RR-BGXMWV (significant), RR-Y151OZ
(minor), RR-7Q4DZQ (minor) — all `addressed`. Reviewed together with TKT-8GD41J
in one pass, since the two branches stack.

The critical finding was mine to own: `days_between` computed its span via
`time.Duration`, an int64 nanosecond count that **saturates** at ~292 years.
`9999-01-01` minus `1000-01-01` returned 106751 days instead of 3286817. A
birthdate — or a zero-valued year-1 date from a parse mishap — silently produced
a plausible-looking wrong number. Now computed on Unix seconds.

`date_add` had the neighbouring defect: `int(1e20)` saturated to maxint and made
`AddDate` wrap the date *backwards*, and a fractional count truncated silently.
Both now error, matching this package's stated preference for refusing ambiguity
over normalizing it quietly.

Two smaller items: my DST test was vacuous (its endpoints gave the same answer
under both UTC and local truncation, so it could not have caught the regression
it existed to prevent), and `predicatefns.RruleNext` plus the re-exported
sentinel had no non-test callers and were deleted.

**Unrelated changes**: none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 three functions with typed signatures | PASS | `Declare`/`Bind` agree; exercised throughout `date_test.go` |
| 2 `days_between` signed whole days, UTC | PASS | `TestDaysBetween` (8 cases incl. 0, ±1, year/leap boundaries), `_TimeComponentTruncated`, `_LocalMidnightBoundary`, `_DSTSpan`, `_NoInt64Saturation` |
| 3 `date_add` day/week, ± counts; month/year rejected | PASS | `TestDateAdd` (7 cases), `_LeapDay`, `_RejectsUnsupportedUnit`, `_RejectsFractionalAndHugeCounts` |
| 4 `rrule_next` next occurrence; malformed vs exhausted distinct | PASS | `TestRruleNext` (6 rules), `_Malformed`, `_ExhaustedThroughEval`; sentinel contract at `TestNextRrule_Exhausted` in metamodel |
| 5 end-to-end through a condition | PASS | `TestDateFuncs_ThroughEvaluator` (8 cases) against a metamodel-bound entity — the path where a date binding as `String` would fail |
| 6 gates | PASS | see Automated Checks |

AC5 was **substituted** deliberately and the reason recorded on the ticket: the
original wording called for an automation `when:` evaluating a date expression,
which was not reachable when this ticket was written — `when:` still ran through
`filter.Parse`. The integration test targets `Evaluator.Compile` +
`EntityRecord` instead, the same path conditions take. TKT-8GD41J then made the
original form reachable, and it is pinned there by
`TestEngine_Condition_DateArithmetic`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-O1DQ98 (covers both stacked tickets).

`docs/cli-reference.md` gained the date-function table and worked `--filter`
examples, the UTC-normalization note, the day/week restriction with its
rationale, and the malformed-vs-exhausted distinction for `rrule_next`.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1380

Single PR covering both stacked tickets (TKT-HQONQE then TKT-8GD41J): the
condition work builds directly on the date functions, and the second is
untestable without the first.
