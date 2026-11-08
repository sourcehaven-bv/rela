---
id: REV-OR8FOO1
type: review-checklist
title: 'Review: rela test v1 (2/4): injectable clock and determinism seams (mail transport, HTTP client, AI provider, output dir)'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (every package passes. dataentry and sqlitestore hit the 10m per-package timeout under a load average of about 26 from other sessions, then passed when rerun alone with a longer timeout. The changed packages were rerun after the review fixes.)
- [x] Lint clean (`just lint`) (0 issues on all four build-tag runs)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (floors checked with go-test-coverage on a profile of the changed packages; the full coverage run was not repeated under the load)

**Comment findings.** Nothing new is introduced.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run on the diff)
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3UL6SLY RR-9PXHMIA RR-E4ATSX0 RR-JIXV9TC RR-SYGLCK1
RR-SZCN3ZH RR-TN64ISM RR-UI7PDOA RR-W20CGLE RR-YEQWPTP

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- WithClock reaches templates, all predicate evaluators, computed properties, Lua, copy/convert, memstore and leases: PASS (TestWithClock_ReachesEveryWritePathClock, TestWithClock_ValidationUsesWorldClock, package clock tests)
- Audit and version stay on the wall clock and godoc says so: PASS (WithClock godoc; allowlist reasons)
- Guard test fails on a new time.Now: PASS (TestWallClockReads plus its matcher self-test)
- Mail: transport replaced, mail.yaml policy kept: PASS (TestNewInMemory_InjectedSeamsWin, blocked recipient stays blocked)
- Per-runtime HTTP client, AI provider wins over ai.yaml, write_file honours the output dir: PASS (lua seams tests, appbuild seams tests)
- Safe by default (RR-8ULC8I2, RR-5PV25TX): PASS (DefaultsKeepTheWorldOffTheNetwork, ZeroOptionsKeepTheDefaults, DefaultOutputDirIsTemporary, tree-hash test with write_file)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing surface until the rela test command in parts 3 and 4; godoc covers the options)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-SWO714A

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line message by the user's standing rule; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (the PR is opened with gh after the ticket is done, as the brief instructs)
