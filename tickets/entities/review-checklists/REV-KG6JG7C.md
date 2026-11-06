---
id: REV-KG6JG7C
type: review-checklist
title: 'Review: Backend: validation: advisory|strict config that blocks on error-severity (DEC-OYV2AK)'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: run by CI; every new file has table-driven tests)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-29D5B1O, RR-7RN4ZTS, RR-07SZ6BW, RR-WM10ST6,
RR-8ZH0BSE, RR-Q3WJ5CZ, RR-85C1FDK, RR-RQ2QJMM, RR-1YR710L (cranky-code-reviewer
and rela-security-reviewer). Two significant ones are wont-fix with a documented
reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1-6 PASS: TestValidationMode_*, TestValidationModeFor,
TestWriteWriteRefused_FieldErrors,
TestTaskItemConvert_RequiredFieldsComeFromTheCaller (strict); manual run in
IMPL-RDT1PZ7.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D3LEJ8D

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
