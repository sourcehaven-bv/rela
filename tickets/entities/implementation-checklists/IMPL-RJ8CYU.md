---
id: IMPL-RJ8CYU
type: implementation-checklist
title: 'Implementation: output.Writer: delete dead/internal/single-caller surface — plimsoll directive deleted (23 → 14)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (schema-JSON tests re-pointed to the extracted type; WriteRelations tests removed with the dead method)
- [x] ~~Integration tests written~~ (N/A: no new behavior — moved code plus deletions)
- [x] Happy path implemented
- [x] Edge cases from planning handled (coverage floor for internal/output watched explicitly, since deleting the WriteRelations tests removes an exerciser of the shared table machinery)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: existing tests re-pointed, not rewritten)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (output + cli packages, full suite)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1469, commit 63c0b9e9), re-run by the coordinator
with `-count=1` in the agent's worktree:**
- `output.Writer` exported methods: **14** — under the default 20 line, so
the `//plimsoll:max-exported-methods` directive is **DELETED outright**, not
ratcheted (the TKT-45QYI outcome the epic celebrates).
- `just plimsoll` passes with no directive on the type.
- `go test -count=1 ./internal/output/... ./internal/cli/...` all pass.
- `just coverage-check`: package floor (50%) and total floor (65%) both
PASS; total 78.3%.
- kong.go comment corrected to the true count (46 = 4 global flags + 42
subcommands) and reframed as a documented structural exception rather than a
ratchet target, per the survey's NO-GO verdict on restructuring the CLI.

## Quality

- [x] Code follows project patterns (schema serialization moved next to its single consumer, per the consumer-side-interface rule)
- [x] Checked for DRY opportunities (three schema interfaces moved out of output's public API along with the methods that needed them)
- [x] No security issues introduced (CLI stdout rendering only)
- [x] No silent failures
- [x] No debug code left behind
