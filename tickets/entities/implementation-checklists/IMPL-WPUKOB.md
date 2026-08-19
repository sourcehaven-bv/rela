---
id: IMPL-WPUKOB
type: implementation-checklist
title: 'Implementation: date arithmetic host functions (TKT-HQONQE)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`days_between`, `date_add` and `rrule_next` in `internal/predicatefns/date.go`,
registered in `Declare`/`Bind`. RRULE mechanics live in `metamodel.NextRrule`
beside the existing `ValidateRrule` — `just arch-lint` rejected a direct
`rrule-go` import in `predicatefns`, and colocating them means validation and
occurrence-stepping share one implementation with identical prefix handling and
error text.

Integration coverage is at the real seam: `TestDateFuncs_ThroughEvaluator`
drives `Evaluator.Compile` + `EntityRecord` against a metamodel-bound entity —
the path where a date property binding as `String` would fail at eval rather
than at compile.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Local helpers (`day`, `dateEnv`, `setVar`, `datePropMeta`) keep each case
stating only what it varies. `setVar` collapsed 19 repeated bind-and-check
blocks. The clock is injected via `NewEvaluatorWithClock` so `today()` cannot
drift with the calendar.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Evidence |
|----|----------|
| 1 typed signatures | `Declare`/`Bind` agree; exercised throughout `date_test.go` |
| 2 signed whole days, UTC | `TestDaysBetween` (8 cases), `_TimeComponentTruncated`, `_LocalMidnightBoundary`, `_DSTSpan`, `_NoInt64Saturation` |
| 3 `date_add` units and signs | `TestDateAdd` (7), `_LeapDay`, `_Months` (13), `_MonthEndIsStable`, `_RejectsUnsupportedUnit`, `_RejectsFractionalAndHugeCounts` |
| 4 `rrule_next` incl. malformed vs exhausted | `TestRruleNext` (6 rules), `_Malformed`, `_ExhaustedThroughEval`; sentinel at `TestNextRrule_Exhausted` in metamodel |
| 5 end-to-end | `TestDateFuncs_ThroughEvaluator` (8 cases) |
| 6 gates | `go test ./...`, `golangci-lint`, `just arch-lint`, `just coverage-check` all green |

Verified by experiment rather than inspection:

- **Saturation**: `9999-01-01` minus `1000-01-01` now returns 3286817 days, not
the saturated 106751 that `time.Duration` produced.
- **Truncation exactness**: negatives truncate toward zero correctly; both
operands are UTC-midnight aligned so the division is exact.
- **DST**: verified the test endpoints actually discriminate between UTC and
local-time truncation (5 vs 4) before relying on them — the first two candidate
cases did not.
- **Month clamping**: Jan 31 + 1 month → Feb 28; leap year → Feb 29; backwards
→ Feb 28; leap day + 1 year → Feb 28; + 4 years → Feb 29.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**DRY**: `utcDay` is the single place the UTC convention is applied, so
`today()`, `days_between` and `date_add` cannot drift apart; `addMonths` /
`daysInMonth` are shared by the month and year units. Deliberately not
extracted: each function keeps its own arg-validation preamble — they differ in
arity and types, and a shared validator would obscure the signatures.

**Security**: no new secrets, auth, crypto or file access. The one eval-time
parse (`rrule_next`) runs a rule already accepted by `ValidateRrule` and returns
a single occurrence, never an unbounded expansion. RR-A3EZR (no parse at eval)
holds for the other two, which operate on already-parsed `Date` values.

**No silent failures — the main defect found and fixed.** `days_between`
saturated silently past ±292 years; `date_add` truncated fractional counts and
wrapped backwards on overflow. Both now exact or an explicit error. Month/year
were withheld in v1 for the same reason and are now offered *with a stated clamp
rule* rather than inheriting `AddDate`'s unstated normalization.

**No debug code**: probe files removed; `git status` clean.
