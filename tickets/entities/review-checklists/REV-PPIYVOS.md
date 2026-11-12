---
id: REV-PPIYVOS
type: review-checklist
title: 'Review: Remote sync status, conflict list and guides'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on remotesync, scheduler, schedulerstate, dataentrywire, dataentry and cli; vitest (415 files); E2E `remotes.spec.ts`
- [x] Lint clean (`just lint`): golangci-lint 0 issues on the changed packages; plimsoll and arch-lint clean; ESLint, Prettier and vue-tsc clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): dataentrywire 72.2%, remotesync 81.7%, scheduler 85.2%, all above their floors

**Comment findings.** No new advisory finding is introduced by this diff.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky and rela-security-reviewer
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed: RR-XBAR16F, RR-2ZIS0R6, RR-RP3TNQD
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review RR-105G1ZB, RR-87P2GGM, RR-K008G0E
(significant, addressed), RR-83TRVEA, RR-BC4P60I (addressed); code review
RR-XBAR16F [security], RR-2ZIS0R6, RR-RP3TNQD (significant, addressed),
RR-SNN2K8Z, RR-KOFO6ZY, RR-2R0S4S7 (addressed), RR-JLN7M6W (deferred),
RR-FCGPKQW (wont-fix); RR-KUA77YM from TKT-8ZGAEC5 (addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-JD1ZX9S)

**Acceptance Status:**
1. PASS: `TestRemotes_ListCountsAfterGating` and E2E `remotes.spec.ts` (status page with gated total 3).
2. PASS: `TestRemotes_NeedsPermission` asserts 403 `permission_required` naming `remote:sync` on every route.
3. PASS: `TestRemotes_ConflictsPageAndActions` hides the hidden row and field and counts only shown entries; E2E hides SEC-1.
4. PASS: scheduler `remote_status_test.go` (run lock, no LastRun, audit ops and summaries); dataentrywire now requires an audited workspace.
5. PASS: `TestRemotes_ActAgreesWithActions` for take remote and link with and without edit rights.
6. PASS: `TestRemotes_ConflictsPageAndActions` and `TestRemotes_ActAgreesWithActions` for the uniform 404 on hidden and missing entries.
7. PASS: `TestLink_SettlesAnUncertainCreate` with the fake remote.
8. PASS: handler test and E2E for a superseded entry under retention 0.
9. PASS: `just docs` generates docs/remote-sync.md and docs/remote-sync-two-way.md.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: remote sync guides, data-entry guide, ACL guide, API reference, CLI reference
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1B3QXW8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done, see TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
