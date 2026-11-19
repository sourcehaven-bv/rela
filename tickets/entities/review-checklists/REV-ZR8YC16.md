---
id: REV-ZR8YC16
type: review-checklist
title: 'Review: View section permission and hiding sections with only redacted fields'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: the touched packages pass, `internal/dataentry` and `internal/dataentryconfig`, plus the new Playwright spec; the full suite runs in CI)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: no new package; the coverage floors run in CI)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-U915AIM, RR-FK771VE, RR-TWVLUTK, RR-9A594EA (minor); RR-G2UPDP1,
RR-8TDOBUT, RR-Q2SQ9I5, RR-YLW8G76, RR-4G0RF2W (nit). All addressed. No critical
or significant findings. rela-security-reviewer found no security issue.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestWithheldSections_PerReader`, `TestWithheldSections_NoPolicyShowsAll`, Playwright staff and customer cases.
2. PASS: `TestAllFieldsRedacted`, `TestWithheldSections_PerReader`, Playwright customer case.
3. PASS: `TestWithheldSections_PerReader` create assertion.
4. PASS: `TestSectionPermission_Config`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-O7M7EMR

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
