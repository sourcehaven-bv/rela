---
id: REV-7ird
status: done
title: 'Review: Add template parameter to create_entity automation action'
type: review-checklist
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: coverage increased from 75.3% to higher with new tests)

## Code Review

- [x] Critical code review completed (use cranky-code-reviewer agent)
- [x] All critical/significant issues from review addressed
- [x] Self-reviewed the diff for unrelated changes

**Code Review Summary:**
- RR-47km (critical): Missing template name validation - Fixed with `isValidTemplateName()` function
- RR-v6iz (critical): Missing integration tests - Added 3 workspace integration tests
- RR-wjy2 (significant): Missing template not surfaced as error - Added check in `createEntityNoAutomation`
- RR-lier (minor): Missing property interpolation test - Added `TestEngine_CreateEntity_TemplateMissingProperty`

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `template` field in YAML parsed correctly - TestEngine_CreateEntity_WithTemplate
2. PASS: Template interpolation works - TestEngine_CreateEntity_WithTemplate with `{{new.kind}}`
3. PASS: Empty template uses default - TestCreateEntity_AutomationWithEmptyTemplate
4. PASS: Missing template reports error - TestCreateEntity_AutomationWithMissingTemplate
5. PASS: Path traversal rejected - TestEngine_CreateEntity_TemplatePathTraversal

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use
