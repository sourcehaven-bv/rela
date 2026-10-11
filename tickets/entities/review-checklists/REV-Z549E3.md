---
id: REV-Z549E3
type: review-checklist
title: 'Review: Move the version sweep out of the store into a versioning service'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (local-only: SPA test needs a frontend build; dataentry and sqlitestore hit the 10m timeout under load and pass alone)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (floors met, 82.6% total, with a longer timeout)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-8KBMQU RR-DMDR67 RR-XHOZ52 RR-U2ZUPN RR-EXS075
RR-F0CRNA RR-YJ374I RR-0D72UG RR-6QLFMW RR-9AV99M RR-LFMYU9 RR-4L9TZ1 RR-422N2F
(cranky: 1 significant, 8 minor, 4 nit; security review: no findings).
Out-of-scope leak filed as BUG-821TEB.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. No new imports beyond internal/store/bgloop; arch-lint clean: PASS.
2. RunVersionTests on sqlite and postgres: PASS.
3. Existing sweep tests unchanged in intent: PASS.
4. Close stops only its own sweep: PASS (sqlite multi-tenant test; built-sweep test on postgres and sqlite).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor)
- [x] ~~User-facing documentation updated~~ (N/A: refactor; rules files updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
