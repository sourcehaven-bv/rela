---
id: REV-LX4I
type: review-checklist
title: 'Review: Add shebang support to Lua scripts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**
- RR-X4B5: Clarify line number preservation (minor, design review) - addressed
- RR-UWZW: RunFile error messages lose filename context (minor, design review) - addressed
- RR-V824: Add explicit test for RunFile with shebang (minor, design review) - addressed
- RR-Z9TR: RunFile did not preserve filename in chunk name (significant, code review) - addressed
- RR-VPXU: Missing test for Windows-style line endings (significant, code review) - addressed
- RR-300X: Documentation lacked context (nit, code review) - addressed

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. AC1 - Scripts with shebang execute: PASS - TestRunFile_WithShebang, TestRunString_WithShebang
2. AC2 - Shebang only stripped from first line: PASS - TestStripShebang_ShebangInMiddle
3. AC3 - Scripts without shebang work: PASS - All existing tests pass
4. AC4 - All entry points covered: PASS - RunString calls StripShebang, all paths funnel through it
5. AC5 - Error line numbers accurate: PASS - TestRunFile_ErrorLineNumbers_WithShebang

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A - internal change)
- [x] ~~User-facing documentation updated~~ (N/A - scripts "just work")
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A - Internal enhancement, no user-facing docs needed

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** #287
