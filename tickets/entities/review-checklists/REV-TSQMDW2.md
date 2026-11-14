---
id: REV-TSQMDW2
type: review-checklist
title: 'Review: Indented --- line closes frontmatter early and truncates properties'
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (scoped: go test for frontmatter, renametype, markdown, store/..., analysis, conflict, fsimport, templating; full `just test` left to CI)
- [x] Lint clean (golangci-lint on internal/frontmatter and internal/renametype: 0 issues; go-arch-lint: no warnings)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (internal/frontmatter at 100%; package floors checked by CI)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer agent on the diff)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed (RR-8J85HFO fixed in renametype)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-8J85HFO (addressed), RR-V94HM7F (deferred), RR-KMGG0CH
(wont-fix), RR-XT30JK2 (wont-fix), RR-JOUGMDI (addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (fuzz seed 03748c2413d59a63 round-trips; 45s FuzzPropertyValuesTypeZoo and 20s FuzzSplit clean)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: property name "---\n" and values with a "---" line
round-trip through fsstore and renametype.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR opened (#2252) without /pr; CI is tracked on GitHub
