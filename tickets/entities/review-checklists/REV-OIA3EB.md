---
id: REV-OIA3EB
type: review-checklist
title: 'Review: Add --access-log for per-request timing to syslog or stderr'
started: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (total 81.5%, thresholds pass)

**Comment findings.** `just comment-report` flagged one finding introduced by
this diff (nil-contract on openAccessLog); fixed with the `Nil:` form. No other
findings in the changed files.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** code review (cranky-code-reviewer and
rela-security-reviewer, no critical or significant): RR-VX8OYQ (security: method
cap), RR-BK9RQ2, RR-1LWG14, RR-1G7AZ4, RR-RNVZLR, RR-07WGUZ, RR-CZ06NK,
RR-9QYWL1, RR-JAL80Q, RR-DK20QJ, RR-7PZJM2, RR-IZL0AZ, RR-HSQI9C. Design review:
see PLAN-VH2ZTI. All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- syslog, tag rela-access, nothing in app log: PASS (probe on atlas
journald; AccessLogAtInfoWithoutHeader; 0 request lines on stderr in the e2e
run).
- stderr: PASS (e2e run with --access-log=stderr).
- Written under --quiet: PASS (AccessLogIgnoresQuiet; e2e with --quiet).
- No Server-Timing from the access log, Debug unchanged: PASS
(AccessLogAtInfoWithoutHeader, AccessLogUnderDebugLogsToBoth, AccessLogSSE).
- No query string: PASS (unit test with ?token=secret; e2e).
- Unknown value exits 2; unusable sink stops startup: PASS (e2e
--access-log=bogus; CheckAccessLogDest; openAccessLog exits 1).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YZFRHE

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1773 already open; pushing the update and monitoring CI)
