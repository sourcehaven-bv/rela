---
id: REV-9UUA49B
type: review-checklist
title: 'Review: Declared piles: operator-defined, locked piles in schema.yaml'
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

**Review Responses:** RR-O5V09FC (critical, addressed: a foreign push reaches a declared pile only by key onto a read-state pile), RR-7NBYMTS (minor, wont-fix), RR-10OQ37N (minor, deferred), RR-CQZY983 (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** every criterion in PLAN-F18MOSO PASS; each names its test, and IMPL-S0V3T1R records the manual end-to-end run.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CR4T9WU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M. -->
