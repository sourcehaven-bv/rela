---
id: REV-9EQUCEY
type: review-checklist
title: 'Review: Flow subject or create type given as an alias skips the visible: check'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (affected packages run locally: flowcfg, appbuild with -race, cli Flow tests; full suite runs in CI on the PR)
- [x] Lint clean (`just lint`) (golangci-lint on `internal/flowcfg`, the only changed package: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (only new checks and tests added in flowcfg; enforced in CI)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3XYIY3Y, RR-424NYLD, RR-4OUPZCR, RR-4R6P2Z0, RR-7ASEKL6

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: `subject: pers` is refused with "alias of person"
(TestParse_Problems). PASS: `create.type: todo` is refused. PASS: subject alias
with a salary template or guard does not load
(TestParse_AliasSubjectReadsNoHiddenProperty).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (GUIDE-flows says to name the type itself)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
