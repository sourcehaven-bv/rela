---
id: IMPL-KO3GL2
type: implementation-checklist
title: 'Implementation: Fuzz sweep files one issue per failing target'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is a GitHub Actions
  `run:` block, not Go — there is no package to add a `_test.go` to. Covered by
  the harness under Manual Verification instead.)
- [x] Integration tests written (test full flow, not just units) — the step's
  `run:` body is extracted from the parsed YAML and executed end-to-end with
  `gh` stubbed on `PATH`, so the real shell logic runs
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: fixtures are
  verbatim `fuzz-failures.txt` rows copied from real sweep runs — a builder
  would add indirection over three-token lines)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded — the `gh`
  stub matches on `$TITLE` from the environment, the same channel the real
  lookup uses, rather than re-hardcoding the title format
- [x] ~~Property comparisons use original object~~ (N/A: no domain objects; the
  assertions are over stdout lines)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Harness: parse `fuzz-sweep.yml`, write the filing step's `run:` body to
`step.sh`, put a stub `gh` on `PATH`, run against a real summary file.

| Scenario | Expected | Result |
|---|---|---|
| AC1/AC2: recurrence + 2 new targets (2026-09-07 summary) | 1 comment, 2 creates | pass |
| AC3: `FuzzPropertyValuesTypeZoo` on fsstore/memstore/sqlitestore | 3 distinct creates, no aliasing | pass |
| AC4: no `fuzz-failures.txt` | nothing filed, exit 0 | pass |
| `error` vs `fuzz-crash` kind | body explains the matching kind | pass |
| `gh` fails on first of 3 targets | other 2 still file, warning logged, exit 1 | pass |
| Summary with no trailing newline | last row still files (RR-EAW6EL) | pass |
| Blank/whitespace rows | skipped, no junk issue | pass |
| Dedup query fails | no duplicate filed, warning, exit 1 (RR-LERUY1) | pass |
| `gh label create` fails | issue still files, exit 0 (RR-513JAO) | pass |
| Issue list returns a full 200-item page | warns dedup may duplicate (RR-SWV734) | pass |
| Package path with `"`, `$`, backtick, `*` | passes through literally, no expansion | pass |

Live check of the dedup lookup against this repo (not a stub):
`TITLE="Weekly fuzz sweep found failures" gh issue list --label fuzz-failure
--state open --json number,title --jq 'map(select(.title == env.TITLE)) |
.[0].number // empty'` → `993`; a non-matching title → empty.

Lint: `actionlint .github/workflows/fuzz-sweep.yml` → clean, with shellcheck
0.11.0 installed so the `run:` block is genuinely inspected. Confirmed the
check is live by injecting an unquoted expansion into a copy and observing
SC2086 fire.

The suite was re-run after the code-review fixes and again after the final
comment edits: 10/10 green.

Not verified: the step running under real GitHub Actions against the real API.
It only executes on a failing weekly sweep, so its first real exercise is the
next sweep that finds something.

## Quality

- [x] Code follows project patterns (check similar code) — mirrors the existing
  step's structure (heredoc-built body, `gh issue comment`/`create`, the
  untouched setup-error guard)
- [x] Checked for DRY opportunities — the body heredoc is built once inside the
  loop and reused for both the comment and create paths; the kind explanation
  is a `case` rather than duplicated prose. No further extraction: the block is
  one linear procedure and hoisting it into a script would split the workflow's
  logic across two files for no gain.
- [x] No security issues introduced — the title reaches jq via `env.TITLE`
  instead of filter interpolation; all expansions quoted; no corpus bytes or
  token material in issue bodies
- [x] No silent failures (errors logged AND returned) — a failed comment/create
  logs a `WARNING:` naming the target, increments a counter, and the step exits
  1 with a summary line
- [x] No debug code left behind
