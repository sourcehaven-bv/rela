---
id: IMPL-NENH0MQ
type: implementation-checklist
title: 'Implementation: Pre-commit full race run times out on the sqlite packages'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: shell hook and justfile recipe; no shell test harness in the repo)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: verified by hand, see evidence)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no Go staged, go.mod only, build-tag-only dirs, syntax errors, deleted dirs, renames, testdata, odd file names)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no automated tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no automated tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no automated tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no automated tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no automated tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Trivial edit staged in `internal/principal`: hook tested only that package with -race and passed in 16s.
- Forced failing test in `internal/calfeed` plus a passing `internal/principal`: hook printed the FAIL package line and the `--- FAIL` and `_test.go:13` lines, then the log path.
- Same test sleeping with `RELA_PRECOMMIT_TEST_TIMEOUT=5s`: hook printed the timeout hint and the stuck test name instead of the goroutine dump.
- Tickets-only staged: linter and tests skipped, hook passed.
- `just install-hooks` in a scratch repo: stubs written, a commit ran `scripts/pre-commit` from the tree; a non-executable script gave a clear message.
- golangci-lint was stubbed out during these runs (local runner overloaded).

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the staged-file gating in scripts/pre-push)
- [x] Checked for DRY opportunities (one `staged` helper for the three git diff calls)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind (forced tests removed)
