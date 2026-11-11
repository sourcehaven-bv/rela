---
id: IMPL-JYD3OG4
type: implementation-checklist
title: 'Implementation: Browser notifications for channel inbox entries (mentions, replies, waiting sessions)'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `inboxNotify`, `inboxRoutes`, `notificationPrefs`, `tabFocus` (incl. no report after pagehide), `useInboxNotifications` (11 tests with fake Web Locks and a recording Notification), `useTabTitleBadge`, `NotificationSetting` (6), `useInboxSummary` hand-off and refusal cases, `deliverLive` hidden-delivery opt-in
- [x] Integration tests written: e2e `channels.spec.ts` "a mention notifies one background tab of the target, and the notification opens it" (two background tabs of one user, bob mentions alice, exactly one notification, click routes to the channel); passed 4/4 with `--repeat-each=4`; full channels spec 17/17
- [x] Happy path implemented: setting on the Settings page, one notifier tab per base and user via Web Locks, title badge, click opens the inbox route
- [x] Edge cases from planning handled: silent baseline on enable and on a missing seen set; turned-off kinds still enter the seen set; overflow above 5 becomes one "N more" notification; focus in any tab suppresses; refusal hides the inbox and stops the notifier; hidden reads debounced (2 s) and floored (10 s) with jitter; desktop shell reports unsupported; signed-out scope disables the switch
- [x] Error handling in place: a failed inbox read keeps the seen set and logs a warning, so the next hint catches up; a Notification constructor that throws (Android Chrome) is logged; a refusal drops held content as `shouldDropHeldContent` asks

## Test Quality

- [x] Using fixture builders or factories for test data: `mention()`, `waitingEntry()`, `page()`, `chan()` builders; e2e uses `NotificationsPage` and `ChannelsPage` page objects
- [x] No hardcoded values in assertions when object is in scope: e2e title is built from `CHANNEL_USERS.member`; route assertions use `FEATURE`
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Manual run: built `rela-server` on the e2e room project, `-principal-header`, a
local proxy signing the in-app browser in as alice; bob posted through the API.

- AC1 title badge: after bob's mention the tab title changed from "rela" to "(1) rela" within 3 s without reload; after opening the channel it went back to "rela" and the Inbox badge cleared.
- AC2 setting: the "Browsermeldingen" card renders in Dutch on Settings; the in-app browser blocks notifications, so the card shows the blocked notice and the switch is disabled.
- AC3/AC5/AC7: e2e with two background tabs: one notification in total, titled "Mentioned by bob@example.com", click opens `/entity/feature/FEAT-001?channel=...`. Unit test: focus in a tab updates the seen set and shows nothing.
- AC4 silent baseline: unit test "reads a silent baseline first"; seen set saved, nothing shown.
- AC6: `unread:false` types never produce inbox entries (server); notifications come only from inbox entries.
- AC8: content comes only from the gated inbox GET; the body never carries message text.
- AC9: prefs, seen set, lock and focus channel are keyed by `relaBase()` and user (unit tests in `notificationPrefs.test.ts`).
- The e2e run found a defect: on navigation the old page posted "unfocused" on pagehide and then "focused" on visibilitychange, which silenced notifications for 30 s. Fixed in `tabFocus.ts` (no reports after pagehide, resume on a bfcache restore) with a unit test.

Checks: frontend `npm run test:run` 410 files / 6080 tests pass; `npm run lint`
0 errors; `vue-tsc -b` clean; changed files formatted with prettier.

## Quality

- [x] Code follows project patterns: `registerLive` options, `runWhenVisible`, `shouldDropHeldContent`, i18n keys in en and nl, page objects for e2e locators
- [x] Checked for DRY opportunities: inbox row key and route shared by InboxView and the notifier (`utils/inboxRoutes.ts`); scope helpers in one module
- [x] No security issues introduced: only gated inbox data, no message bodies, scoped storage, no new server surface
- [x] No silent failures: failed reads and failed notifications are logged, and the seen set is not advanced on failure
- [x] No debug code left behind: debug lines added to the e2e test while diagnosing were removed (file restored from backup)
