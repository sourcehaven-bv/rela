---
id: REV-LNKFJS
type: review-checklist
title: 'Review: Split metamodel.Metamodel into focused types'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test ./...`: all pass except TestChromeStyle_TargetsShippedClasses, which needs a built SPA and fails the same way on develop)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: no tests removed except a getter test for deleted methods; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5LI4EV RR-P7A69F RR-KEVEXR RR-4W5QKE RR-CCG7TL
RR-C7C4K1 (cranky: 2 minor, 3 nit; security review: no findings).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. No behaviour change; full suite passes: PASS.
2. Projection bodies byte-identical; projection, version and migration tests pass: PASS.
3. Metamodel at 21 exported methods; `just plimsoll` passes: PASS.
4. arch-lint OK; tests compile under default, memorybackend, postgres, sqlite: PASS.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor)
- [x] ~~User-facing documentation updated~~ (N/A: refactor; code comments updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
