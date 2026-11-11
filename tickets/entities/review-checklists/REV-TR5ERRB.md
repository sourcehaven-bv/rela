---
id: REV-TR5ERRB
type: review-checklist
title: 'Review: Browser notifications for channel inbox entries (mentions, replies, waiting sessions)'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): one run failed only on `internal/store/sqlitestore`, which hit the 10-minute test timeout while two other worktrees ran the same package; rerun alone it passes (`ok ... 552.682s`). The change has no Go code. Frontend `npm run test:run`: 410 files, 6093 tests pass. E2E `channels.spec.ts`: 17/17 pass after the review fixes.
- [x] Lint clean (`just lint`): `0 issues` on every build tag; frontend `npm run lint` 0 errors; e2e eslint clean; `vue-tsc -b` clean
- [x] Comment lint gate clean (`just comment-lint`): no unresolvable doc links; no Go comments changed
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: the change has no Go file, and the frontend has no coverage gate. Two runs did not reach the floor check, because `appbuild` TestOutputs_TriggerFinalCloseOnly and the `sqlitestore`/`sqlchannels` 10-minute timeouts failed while other worktrees ran the same packages; `appbuild` passes 3/3 and `sqlitestore` passes when run alone.)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer both ran on the full diff
- [x] All critical review-responses addressed: none in code review (the two design-review criticals RR-DK9X06X and RR-OUWRZL3 were addressed in planning)
- [x] All significant review-responses addressed: RR-1CJNO8P (generation guard), RR-UXCDH43 (seen set keeps earlier ids and skips late rows), RR-GP86VNV (e2e proves exactly one tab notifies)
- [x] Self-reviewed the diff for unrelated changes: only the notifier, its helpers, the inbox route share, the settings card, i18n keys, docs and tests

**Review Responses:** Code review: RR-1CJNO8P, RR-UXCDH43, RR-GP86VNV
(significant, addressed); RR-LQCWWJJ, RR-Z4D8CLL, RR-N932P8K, RR-O8GTNTS,
RR-T2HHQSG (minor, addressed); RR-VLU2ME3 (minor, wont-fix: ids are guessable,
so hashing does not hide existence, and sign-out cannot be hooked; the set is
cleared on a refusal); RR-JTXYGNL, RR-G7A444D, RR-UJSP1QI, RR-G3YW4EX (nit,
addressed). Design review: RR-DK9X06X, RR-OUWRZL3, RR-HAGWCT2, RR-46BEVRX,
RR-ED0HQA6, RR-Z5WZ2ED, RR-V4VVR1F, RR-4DYGAZ2, RR-YPJ2D27, RR-1PQFI1Y
(addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 title badge: PASS. Manual: "(1) rela" within 3 s of bob's mention, back to "rela" after reading; `useTabTitleBadge.test.ts`.
- AC2 setting off by default, asks permission, hidden or disabled where unsupported: PASS. Manual: Dutch card with blocked notice; `NotificationSetting.test.ts` (7 tests incl. no storage).
- AC3 one notification across tabs, at most one visible per channel: PASS. E2E: two background tabs, holder shows two, the other none; tag per channel row.
- AC4 silent baseline: PASS. `useInboxNotifications.test.ts` baseline test; enabling clears the seen set.
- AC5 no notification while a tab has focus: PASS. Unit test; focus shared over BroadcastChannel, peers asked on start.
- AC6 `unread:false` types never notify: PASS. Notifications come only from inbox entries, which exclude them.
- AC7 click opens the inbox route: PASS. E2E click opens `/entity/feature/FEAT-001?channel=...`; unit test checks the full route with `seq`.
- AC8 only gated content: PASS. Only the gated inbox GET feeds notifications; a refusal clears the seen set and closes shown notifications.
- AC9 per base and user: PASS. Scope is `JSON.stringify([base, user])`; `notificationPrefs.test.ts`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-data-entry.md and GUIDE-realtime.md, regenerated with `just docs`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-P11GB3U

## Final Checks

- [x] Commit message explains the why, not just what: to be written at commit time as a minimal message naming the ticket, per the user's standing preference
- [x] No TODOs or FIXMEs left unaddressed: none in the diff
- [x] Ready for another developer to use: documented in the guides and in module comments

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: deferred; commit and PR only when the user asks)
