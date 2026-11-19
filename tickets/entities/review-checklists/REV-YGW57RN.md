---
id: REV-YGW57RN
type: review-checklist
title: 'Review: Analyze reports hidden fields as missing'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: the changed packages pass, `go test ./internal/dataentry/ ./internal/mcp/ ./internal/metamodel/`; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on internal/dataentry, internal/mcp and internal/metamodel: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: adds tests and no new package; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BUJ1PCS (significant, addressed: MCP analysis had the
same bug), RR-A1VD7PW (minor, addressed), RR-HICOIG3 (minor, wont-fix),
RR-30F4YNC (nit, addressed), RR-AN0BUDU (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a hidden field gives no finding on `/_analyze` and
in the MCP analysis (TestACLAnalyze_PropertiesSkipsHiddenFields,
TestAnalyzeProperties_SkipsHiddenFields). PASS: a property finding starts with
the field label, or its name when it has none (TestValidateReadable).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; docs/data-entry.md and GUIDE-data-entry got one paragraph anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)
