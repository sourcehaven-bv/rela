---
id: IMPL-ZUZLYO
type: implementation-checklist
title: 'Implementation: CI Fuzz timeout classification (BUG-1VVXHZ)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — CI shell has no unit-test harness; each decision branch was instead exercised directly, see evidence
- [x] Integration tests written (test full flow, not just units) — all three production fuzz targets run end-to-end through the new helper
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed) — an unrecognised failure still fails, with `::error::`

## Test Quality

- [x] Using fixture builders or factories for test data — branches driven by *real captured CI output*, not invented strings
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified

**Verification Evidence:**

| Scenario | Input | Result |
|---|---|---|
| Slow-runner timeout | the exact captured CI output (`--- FAIL` + `context deadline exceeded`) | exit 0, `::notice::` — tolerated |
| Genuine finding | crash output containing `Failing input written to testdata/fuzz/…` | exit 1, `::error::` |
| Genuine finding, live | a real always-failing fuzz target in a scratch Go module | exit 1 — findings still break the build |
| Build error | `[build failed]` output | exit 1, `::error::` unrecognised |
| Production targets | all three real targets, 10s each | pass |

Also: `yaml.safe_load` parses the workflow; the extracted shell passes `bash
-n`.

**The bug I nearly shipped:** my first draft matched `Failing input written
to|--- FAIL`. The timeout output contains **both** `--- FAIL` and `context
deadline exceeded`, so that version would have re-failed precisely the case it
existed to tolerate — the fix would have been a no-op while appearing correct.
Caught by testing the branch against real output rather than my assumption of
it. The final version matches only positive reproducer evidence, and a comment
at the call site warns against re-widening it.

## Quality

- [x] Code follows project patterns (matches the workflow's existing step style)
- [x] Checked for DRY opportunities — one `run_fuzz` function replaces three near-identical bare commands, and adding a target is now one line
- [x] No security issues introduced — no `github.event.*` interpolation; only static literals (checked against the workflow-injection guidance)
- [x] No silent failures — this is the crux: only a *classified* timeout is tolerated, never a blanket ignore
- [x] No debug code left behind
