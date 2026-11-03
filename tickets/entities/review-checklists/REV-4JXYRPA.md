---
id: REV-4JXYRPA
type: review-checklist
title: 'Review: Read state, actor and reason on declared pile items; unread badge and expiry'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` ok; `just lint` 0 issues; `just arch-lint`, `just plimsoll` and `just comment-lint` clean; frontend 5691/5691 tests and `vue-tsc` clean; pgpiles conformance passed against a real PostgreSQL database.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RVJ2PH2 (minor, addressed: RenameOwner moves the actor too), RR-NEWH5VD (minor, addressed: DeleteOwner clears the actor), RR-SDBQN6I (minor, deferred), RR-BTFCP1G (minor, deferred), RR-0CIXPZC (minor, wont-fix), RR-KCXR09W (nit, wont-fix), RR-JFZ63ZA (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** every criterion in PLAN-OG0M118 PASS; each names its test, and IMPL-DFMQ6B7 records the manual end-to-end run.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-JVQWAMS

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M. -->
