---
id: REV-D4SLYF
type: review-checklist
title: 'Review: Realtime 2/2b: SPA presence avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend test:run 4140 tests; dataentry and presence race tests; e2e presence spec 10 of 10)
- [x] Lint clean (`just lint`) (npm run lint 0 errors; golangci-lint on dataentry for the live fix)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (no Go floor touched; frontend has no coverage gate)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (avatar cap, leave bound, live change lost during baseline)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BQR4MJ RR-15ETGO RR-9JYOPF RR-54XVPD RR-7XTZ6K
RR-VY21JW RR-2NGYPG RR-BES5TI RR-K0X70N RR-SJUH2L RR-93OUWG RR-IWI1YX

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 to AC7 PASS; evidence in IMPL-TCW61K.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YFBDST

## Final Checks

- [x] Commit message explains the why, not just what (minimal messages per user preference; ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: stacked PR on the presence backend PR is opened by a sub-agent after done)
