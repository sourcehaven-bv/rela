---
id: REV-0M0XAM8
type: review-checklist
title: 'Review: Calendar date buckets for grouped lists (day/week/month/year, near-then-far, newest first)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`) (dataentryconfig and the SPA suites pass; dataentry fails only TestSPAShellInjection, which compares the locally built shell and fails on develop too)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: CI runs the gate on the PR)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: CI runs the coverage floor on the PR)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1KDERHX, RR-7KK8TSH, RR-8STIZXV, RR-9VCADV8,
RR-BTT32EY, RR-KHRJ2TG, RR-KP5BGTY, RR-UW9VZ22, RR-YWCT9P4

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: config decode and validation tables; SPA bucket
tests over DST, Monday weeks, year boundary, desc order, near-then-far and
prefill.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
