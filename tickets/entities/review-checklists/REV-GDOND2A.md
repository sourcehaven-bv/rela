---
id: REV-GDOND2A
type: review-checklist
title: 'Review: RELA_AFFORDANCE_PROFILE=none/demo disables client field ceilings on REST'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./internal/dataentry ./cmd/rela-server pass; TestSPAAssetsNotInjected needs the built frontend and is left to CI)
- [x] Lint clean (`just lint`) (golangci-lint on touched packages: 0 issues; go-arch-lint OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: only tests were added; CI runs the floor check)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and rela-security reviewers; security found nothing)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-6MNU1IQ (design), RR-JVF3220, RR-KPS173P, RR-Z5E9Q13,
RR-16V8ZFZ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (none/demo with role grants and with a ceiling refused; without grants, nil declarative, empty and unknown profile unchanged)
- [x] Test evidence documented in implementation checklist (IMPL-6FGQPC2)

**Acceptance Status:**
- none/demo + field rules refused with a message naming RELA_AFFORDANCE_PROFILE, the value and acl.yaml field rules: PASS (TestResolverFromProfile_Selection, TestNewServer_AffordanceProfileWithFieldRules, manual run)
- Without field rules or declarative the profiles still work: PASS
- Unknown profile warns and falls through: PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry/api-reference.md)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR is opened with gh right after the bug is done)
