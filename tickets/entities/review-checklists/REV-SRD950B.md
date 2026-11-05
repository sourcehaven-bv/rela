---
id: REV-SRD950B
type: review-checklist
title: 'Review: Conflict resolve skips field rights and errors quote hidden values'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test ./internal/...` clean)
- [x] Lint clean (`just lint`) (golangci-lint on every changed package, one at a time; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: every new function is covered by the regression tests; the CI coverage job enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OMSFIIS, RR-9PQSSUL, RR-DB847HT, RR-IL05NDW
(significant, addressed); RR-DYEE08G, RR-ZV6MQ1M (minor, addressed); RR-21276I8,
RR-2L111RH, RR-K8D5K4Y (minor, deferred with reason); RR-Q0EUJM5 (nit,
wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: resolve runs the PATCH field check
(TestV1ConflictResolve_FieldWriteGate, TestV1ConflictResolve_FieldWriteCases,
TestV1ConflictResolve_RelationMetaWriteGate). PASS: validation messages never
quote values, on resolve and PATCH
(TestValidatePropertyValue_MessageNeverQuotesValue,
TestV1ConflictResolve_ValidationErrorHidesValue,
TestV1UpdateEntity_WarningHidesHiddenValue). PASS: status and sync list only
readable data paths (TestGitStatusResponse_ConflictFiles).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (acl-security guide, regenerated docs/acl-security.md)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened after done; CI is tracked on GitHub)
