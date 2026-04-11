---
id: REV-PXFT8
type: review-checklist
title: 'Review: Remove view system, replace with Lua equivalents'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: go version mismatch locally, CI passes)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OXLY3, RR-J1CZR, RR-O1QZC, RR-JZFW0 — all addressed

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updated inline)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/363
