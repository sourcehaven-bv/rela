---
id: PLAN-89N636M
type: planning-checklist
title: 'Planning: Browser notifications for channel inbox entries (mentions, replies, waiting sessions)'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** the inbox (mentions, replies, waiting sessions) and its sidebar
badge only reach a user who is looking at the rela tab. A mention or a session
waiting on the user goes unnoticed while they work elsewhere.

**Scope:**

In (v1, frontend only, no new server delivery path):
- Unread prefix in the tab title, `(N) <app title>`, where N is the inbox
summary's mentions plus waiting entries.
- An opt-in "Browser notifications" setting on the Settings page, stored per
browser in localStorage. Enabling it asks for the Notification permission inside
the click handler. Kinds: mentions and replies; waiting sessions.
- While the page is hidden or unfocused, one tab per browser (elected with
the Web Locks API) reads the first inbox page on a `live:changed` hint, diffs it
against a seen set, and shows one notification per new entry.
- A click focuses the tab and opens the entry's `links.message` (or
`links.channel`, or the inbox page).

Out (follow-up tickets):
- Web Push with no tab open: service worker, VAPID keys, push subscriptions
per principal in tenant tables / `state.KV`, a send job on `jobs.Queue`,
content-free payloads.
- Channel inbox entries in the desktop shell's native notifications and Dock
badge (`internal/desktopnotify`, `desktop.yaml`).
- Server-side notification preferences and per-channel mute.
- Favicon badge; mail digest of unread mentions.

**Acceptance Criteria:**
1. With unread mentions or waiting entries, the title reads `(N) <title>`;
with none it reads `<title>`. The visible tab and the elected notifier tab stay
current; other hidden tabs update when shown.
2. The setting is off by default. Turning it on requests permission; a denied
or unsupported permission shows an explanation and leaves it off. The setting is
hidden in the desktop shell.
3. With the setting on and every rela tab of the project unfocused, a new
mention, reply or waiting entry shows a notification, even with three tabs open.
At most one notification per channel is visible; the newest wins.
4. Entries present when the setting is turned on, or when no seen set exists
for this project and user, never notify.
5. While any rela tab of the project has focus, no notification is shown.
6. A message type with `unread: false` never notifies (it never reaches the
inbox).
7. Clicking a notification focuses the tab and opens the same SPA route the
inbox row links to.
8. An entry the principal can no longer read never notifies, because every
notification comes from the gated inbox GET.
9. Prefs and the seen set are per project base and per user: another user in
the same browser starts with the setting off.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (inbox design) covers the live path and rejects a
push stream of entries (g2). No new research doc: v1 reuses that design.

**Existing Solutions:**
- `frontend/src/composables/useInboxSummary.ts`: live summary query, refetched
through `runWhenVisible`, so a hidden tab does not refetch today.
- `frontend/src/composables/visibleRefetch.ts` (TASK-550RD): hidden tabs defer
refetches to avoid a wake-up stampede. v1 keeps that rule for every tab except
the one elected notifier, and debounces its reads.
- `internal/desktopnotify`: silent first evaluation and a seen set. v1 uses
the same model in the browser.
- Libraries: none needed. Notification API and Web Locks API are native.
Web Push would need a Go library (for example `SherClockHolmes/webpush-go`);
deferred with Web Push.
- Slack, GitHub and Linear notify per mention and coalesce by thread; v1 uses
the notification `tag` (channel key) to replace rather than stack.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. **Hidden delivery for one key.** `registerLive(key, path, onChange,
{deliverHidden})`. `useEvents` asks `liveDeliversHidden(key)` and calls
`handleLiveChanged` directly for such a key; every other key stays behind
`runWhenVisible` (TASK-550RD). The post-PUT baseline in
`useLiveSubscription.send` follows the same flag.
2. **Shared routes.** Move `rowKey()` and the route builder from
`InboxView.vue` to `utils/inboxRoutes.ts` (`inboxRowKey(c)`, `inboxRoute(c,
seq?)` returning the path before `space.href`). InboxView and the notifier both
use it.
3. **Scope.** `notificationScope(user)` = `relaBase()` + user; it names the
localStorage keys (`rela.notify:<base>:<user>` for prefs,
`rela.notify.seen:<base>:<user>`), the Web Lock
(`rela-inbox-notify:<base>:<user>`) and the BroadcastChannel
(`rela-focus:<base>`). Nothing starts until `useMe` answers with a known user.
4. **Prefs.** `utils/notificationPrefs.ts`: `{enabled, mentions, waiting}`,
defensive parse (malformed means off), plus `notificationSupport()`:
`unsupported` when `Notification` is missing or the page runs in the desktop
shell (`'_wails' in window`), else the permission.
5. **Elected notifier** (`useInboxNotifications`, mounted in the sidebar next
to `useInboxSummary`):
   - Active when prefs enabled, permission `granted`, inbox not hidden, user
known. Requests the Web Lock; only the holder does the rest. Without Web Locks
it does nothing (title badge still works when visible).
   - While holding the lock it re-registers the summary live key with
`deliverHidden: true`. On a hint (and at start) it reads `getInbox()` with
default parameters after a 2 s debounce plus jitter, at most once per 10 s, and
writes the result with `storeInboxSummary(cache, inbox)`. That updates the
summary, sidebar badge and title from one request (the summary endpoint computes
the full inbox anyway).
   - Identity: `m:<rowKey>#<seq>` for mentions and replies,
`w:<rowKey>@<session.generation>` for waiting entries.
   - Seen set: if none is stored (or the setting was just turned on) the read
is a silent baseline; otherwise new ids notify. After each read the set becomes
exactly the ids in that read, so it stays bounded by the page (50 waiting + 50
mentions).
   - Focus: every tab posts `focus`/`blur` on the scoped BroadcastChannel and
answers a `who-focused` ping. The holder skips notifying while any tab reports
focus, but still updates the cache and the seen set.
   - Per read at most 5 notifications; more become one "N more in your
inbox" notification.
   - Text: title = kind ("Mentioned by <author>", "Reply from <author>",
"Waiting on you: <state label>"), body = entity type label and `anchor_title`.
Never the message body. `tag` = rowKey.
   - Click: `window.focus()`, `router.push(inboxRoute(c, seq))`, close.
6. **Title badge** (`useTabTitleBadge`): captures the base title once and sets
`(N) <base>` from `summary.mentions + summary.waiting`.
7. **Setting UI**: `NotificationSetting.vue` on `SettingsView.vue` beside
`LanguageSetting`, with en/nl strings.

**Alternatives considered:**
- Diff the summary counts instead of the inbox page: a read and a new entry
in the same round cancel out, and the text could only say "N new". Rejected; the
page costs the same on the server.
- A second live key on the inbox path: unneeded once the elected tab
writes the page into the summary cache.
- Web Push in v1: key management, a subscription store per backend, a send
job and a service worker. Follow-up ticket.
- Every tab notifies and dedups with BroadcastChannel: every hidden tab would
read. Rejected for Web Locks.

**Files to modify:**
- `frontend/src/composables/useLiveSubscription.ts`, `useEvents.ts` (+ tests)
- `frontend/src/utils/inboxRoutes.ts` (+ test), `frontend/src/views/InboxView.vue`
- `frontend/src/utils/notificationPrefs.ts` (+ test)
- `frontend/src/composables/useInboxNotifications.ts` (+ test)
- `frontend/src/composables/useTabTitleBadge.ts` (+ test)
- `frontend/src/components/settings/NotificationSetting.vue` (+ test)
- `frontend/src/views/SettingsView.vue`, `frontend/src/components/common/Sidebar.vue`
- `frontend/src/i18n/locales/en.json`, `nl.json`
- `docs-project/` data-entry inbox guide; regenerated `docs/`
- `e2e/tests/` one spec with a stubbed `Notification`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Inbox GET response: already gated per principal; notification text uses
only its fields. Text is passed as plain strings (the Notification API does not
render HTML).
- localStorage prefs and seen set: parsed defensively; a malformed value
falls back to defaults (off).
- Click target: built by `inboxRoute` from the entry's type, anchor and
channel (URL-encoded), routed through the SPA router; never a URL taken from the
response.

**Security-Sensitive Operations:**
- OS notifications persist in the notification center and can show on a
lock screen. v1 never shows message bodies, only the entity title the principal
may read. Off by default.
- The seen set holds ids, not content. Prefs and the seen set are keyed by
project base and user, so another user of the browser starts with the setting
off and never inherits the set.
- No new server endpoint, no new delivery path (RES-B0MJAZ g2 holds).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: `useTabTitleBadge` unit test with summaries 0, 3, hidden.
- AC2: `NotificationSetting` tests with permission granted, denied,
default-then-denied, and `Notification` undefined.
- AC3: unit test with a fake lock manager: only the holder reads; e2e with
two pages and a stubbed `Notification` counting calls.
- AC4: first read silent; enable-after-load silent.
- AC5: visible + focused page: no read, no notification.
- AC6: covered by the inbox server tests; one unit test that an empty
inbox diff shows nothing.
- AC7: click handler calls `window.focus` and `router.push` with the link.
- AC8: fake API removes an entry between reads: no notification.

**Edge Cases:**
- Permission revoked in browser settings while enabled: stop, show state.
- Lock holder tab closed: next tab takes over and diffs against the stored
set, so nothing that arrived in between is lost or repeated.
- Burst of 50 mentions: 5 notifications, then one "N more".
- Inbox `degraded` or `truncated`: notify the entries present.
- Web Locks or BroadcastChannel unsupported: no hidden reads.
- Session leaves and re-enters the same state: new generation, notifies.
- Two projects open in the desktop shell: separate locks and keys.
- Another user signs in on the same browser: setting off for them.

**Negative Tests:**
- Corrupt localStorage JSON: defaults to off, no throw.
- Inbox GET 403/404: notifier stops, as the summary hides the inbox.
- Network error: keep seen set, retry on next hint.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Hidden-tab reads add server load. Mitigation: one tab per browser,
debounce and a 10 s floor; opt-in.
- Browsers throttle timers in hidden tabs; Chrome's intensive throttling
can stretch the 2 s debounce to about a minute. Accepted.
- Safari and the Wails webview handle the Notification API differently.
Mitigation: feature-detect; desktop shell is a follow-up.
- Notification spam. Mitigation: per-read cap and `tag` per channel.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via docs-project) - inbox notifications setting
- [x] ~~docs/metamodel.md~~ (N/A: no config change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DK9X06X and RR-OUWRZL3 (critical), four
significant and four minor/nit findings, all addressed in this plan (see the
ticket's has-review-response links).
