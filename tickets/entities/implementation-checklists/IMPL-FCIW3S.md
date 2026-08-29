---
id: IMPL-FCIW3S
type: implementation-checklist
title: 'Implementation: Hierarchical Gantt view for data-entry (gantts: config, recursive roll-up, drill-down)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Go: `validate_gantts_test.go` (config, one-defect-per-case),
`gantt_handler_test.go` (endpoint through the real App wiring — tree shape,
roll-up, breach, policies, cycle, ACL differentials, caps, drill 404-parity).
Frontend: `ganttLayout.test.ts` (13 pure-function cases: ticks, spans,
flatten/drill). Handler errors carry HTTP shapes via `ganttError`; bad DATA
degrades a bar (nil date), bad CONFIG refuses the load.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`newGanttTestApp(t, mutate...)` + `seedProject`/`seedEpic` builders;
`validGantt()` + `withGantt(fn)` mirror the calendar tests' one-defect pattern.
Date assertions are intentionally literal ("2026-03-01") — the expected value IS
the specification of the fold.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against a scratch project (5-level recursive containment:
project ⊃ project ⊃ project ⊃ epic, mixed `contains`/`has-epic`) and drove
`/gantt/delivery` in a real browser:

- sidebar entry minted (`/gantt/delivery`, gantt icon); route + fetch work
- roll-up envelope correct at every level; Core Build showed `breach.after`
(child ends 25d past planned end) with dotted-amber overrun + red striped
past-commit rule on separate tiers; Discovery showed `breach.before`
- drill-down re-roots with breadcrumbs; URL carries `?path=` (linkable,
back-button works); axis rescales to the subtree
- **found and fixed two real bugs by testing**: (1) unquoted YAML dates parse
to `time.Time`, which `GetString` silently drops — switched to the existing
`entityTimeValue` helper and added `TestGantt_TimeTypedDateProperties`; (2) my
invented CSS tokens didn't exist in the SPA's vocabulary — remapped to
`--accent-color`/`--border-color`/`--card-bg` etc.
- the truncation test caught a third: exact budget exhaustion incorrectly set
`truncated`, which would have been a leak-adjacent wrong signal

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Config mirrors `validate_calendars.go`; handler is an extracted struct
(plimsoll, RR-SVIJ11) following `viewsHandler`. The gate→redact→fold→cap
pipeline is documented on the type and pinned by
`TestGantt_ACLRollupExcludesHiddenChild` / `_FieldHiddenDateExcludedFromFold` /
`_TruncatedIsPostFilter`; recorded in CLAUDE.md as a load-bearing invariant.
Gates: `just lint` 0 issues, `just arch-lint`, `just plimsoll` (Config carries a
documented format-mirror directive), `just comment-lint`, full `go test ./...`,
frontend typecheck + 2018 unit tests, eslint 0 errors.
