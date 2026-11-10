---
id: REV-60UYMIG
type: review-checklist
title: 'Review: Remote pull: refuse symlinked credential files and hide unreadable conflict rows'
started: "2026-11-09"
completed: "2026-11-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test` on the changed packages and their callers: remotesync, scheduler, reconclient, cli, appbuild; CI runs the full suite)
- [x] Lint clean (golangci-lint on the changed packages, also with GOOS=windows for httpremote)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: only tests were added; CI enforces the package floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent; rela-security-reviewer also run)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TL8SYHI, RR-8WTTZJA, RR-LVZ32AU, RR-O09QR38,
RR-EWX83N5, RR-DB7TC6V, RR-N1FH3P6 (addressed); RR-8KRRF1L, RR-NM9DVTS
(deferred, minor).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestCredential_ReadDeclaredSymlink (symlink to another secret refused).
2. PASS: same test, secret mount case.
3. PASS: TestRun_ConflictHidesUnreadableLocalRow and TestRun_HiddenLocalRowsAreNotNamed.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore; docs/remote-sync.md and the ACL guide were updated in the diff)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
