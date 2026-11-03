---
id: PLAN-FQNR4A
type: planning-checklist
title: 'Planning: Inbox 4: SPA inbox page and live updates'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slices 1 to 3 (TKT-ZZ4PNX, TKT-2B2AIA, TKT-6RXAC0) built the inbox
as an API: `GET /api/v1/_inbox`, MCP `list_inbox` and `rela channel inbox` list
the sessions that wait on the caller, the channels with unread messages and the
unread mentions and replies. The web app shows none of it. A person learns that
something waits for them only by opening the right entity. RES-B0MJAZ slice 4
adds the web surface: a sidebar "Inbox" item with a badge, an inbox page, and
live updates so the badge and the page move without a reload. The research
picked option g1: the inbox becomes a shape of the live hub, so it uses the same
replay-and-hash rule as every other live path and needs no new push channel.

Terms used below:
- **Principal key**: `channels.Reader.User` of the caller, as slice 3
defines it. Addressed rows and inbox wakes use it.
- **Inbox subscription**: a live subscription whose path is `/api/v1/_inbox`
or `/api/v1/_inbox/summary`.
- **Inbox wake set**: what an inbox subscription listens to. Its last 200
replay reports it: the principal key and reader, whether its kinds ask for
mentions or replies, the channel types that have a waiting state, and the
channel keys of the caller's gated read window and gated mention entries
(`InboxView.WaitingTypes` and `InboxView.WakeKeys`, which exist since slice 2
for the MCP watch).
- **Durable change**: a channel change that a stored write made (a message,
an edit, a removal, a session or lifecycle move, an action). Typing and
ephemeral signals, and their expiry, are not durable changes.
- **Inbox grid**: the ticks on which inbox subscriptions may replay. The hub
ticks every 500 ms; an inbox subscription replays only on a tick whose number is
a multiple of 4, so at most every 2 s. The grid is the same for every
subscription and never moves.
- **Safety replay**: every 30th tick (15 s, a multiple of the grid) the hub
replays every inbox subscription that has a wake set, whether or not anything
changed in this process.
- **Round memo**: a per-round, per-principal cache of one `Service.Inbox`
call, shared by the summary and page replays of that principal in that round. It
never outlives the round and is never shared across principals.
- **Badge count**: waiting sessions plus unread mentions and replies.

**Scope:**
- In: `GET /api/v1/_inbox/summary`. It answers counts only:
`{waiting, waiting_truncated, unread_channels, mentions, degraded}`. It is a
projection of the same `Service.Inbox` call the inbox route makes with default
parameters. Same 404 with realtime off, same errors. Wire type, OpenAPI, wire
golden, TypeScript type.
- In: two new live shapes. `/api/v1/_inbox` takes the query keys `limit`
and `kinds` (no `cursor`), each at most once. `/api/v1/_inbox/summary` takes
none.
- In: the inbox wake set, learned by the hub from the route's own 200
replay, and the selection rule that replays an inbox subscription when a durable
change can move it (Technical Approach 4).
- In: `realtime.Local` tells durable changes apart from typing and signal
changes, so the hub can skip inboxes on keystrokes.
- In: `Service.Post` and `Service.MarkRead` wake the caller's own inbox
whenever they write the caller's marker.
- In: `Local.SubscribeReads` renamed to `SubscribeLive` and opted into
addressed targets, so the hub sees mentions and replies.
- In: the inbox grid, the safety replay and the round memo in the hub.
- In: SPA: API functions, live path builders, a Pinia Colada query for the
summary with one fixed-key live registration, a pinned sidebar item "Inbox" with
a badge, a route `/inbox` with an `InboxView` page, mark read per row, and a
channel deep link (`?channel=` on the entity route) that also opens closed and
archived channels.
- In: rela-components: `RlInbox` (presentational, three sections, empty
states), Messages keys in en and nl, and `RlInbox.stories.ts` modelled on the
realtime mockup (self-contained, no import of the untracked mockup fixtures).
- In: one e2e scenario in `e2e/tests/channels.spec.ts`.
- In: docs (realtime guide, API reference, data-entry guide, ACL doc).
- Out: the composer `@` picker (TKT-00QR1J).
- Out: mail for mentions or waiting sessions (TKT-Z3PS9U).
- Out: delivery across processes (TKT-YK30D1). A post over stdio MCP or the
CLI in another process does not wake the web hub at once. The safety replay
picks it up within 15 s; any graph write in the server process picks it up at
the next grid tick. The docs say so.
- Out: scrolling to the mentioned message. The deep link opens the channel
and scrolls the channel panel into view; it does not scroll to `seq`.
- Out: paging the waiting list on the page. The page shows the first page
(50 entries) and says "Showing 50 of N" when there are more. The API cursor
stays available to other clients.
- Out: a separate Local hint for session moves (`i:sessions` in the
research). The wake set listens to every durable change in a channel of a
waiting type instead, as the MCP watch already does. See Open questions.

**Example:** Bob posts "`@alice` can you approve?" in the `case` channel of
TKT-12, which has no waiting state. At the next grid tick (within 2 s), Alice's
sidebar shows "Inbox 1". She opens `/inbox`. "Mentions and replies" lists "Bob
mentioned you in Case on TKT-12 Fix login, 10:04". She clicks the row and lands
on `/entity/ticket/TKT-12?channel=case` with the channel open and in view.
Opening the channel marks it read, a live frame arrives, and the badge
disappears. Carol, who cannot read TKT-12, gets no frame at any point.

**Acceptance Criteria:**

1. **Summary route answers counts.** `GET /api/v1/_inbox/summary` answers
200 with `waiting`, `waiting_truncated`, `unread_channels`, `mentions` and
`degraded`, equal to `count`, `truncated`, `unread_count`, `mention_count` and
`degraded` of `GET /api/v1/_inbox` for the same caller. Its query string is
ignored: `?limit=abc` still answers 200. Test: `TestInboxSummaryHandler_Shape`
seeds 3 waiting sessions, 2 unread channels and 1 mention and compares both
answers, then sends `?limit=abc`; `TestInboxView_Summary` (pure table,
realtime).
2. **Summary refusals match the inbox.** Realtime off or channels not wired
is 404 with the same title; a non-GET is 405; a caller without identity gets the
inbox's 403 `no_principal`; `inbox_too_large` passes through with the inbox's
status and code. Test: `TestInboxSummaryHandler_Errors`.
3. **Summary reveals nothing hidden.** A project with extra waiting
sessions, unread channels and mentions on rows the caller cannot read answers
the summary byte for byte like a project without them. Test:
`TestInboxSummary_HiddenRowsChangeNothing` (real declarative ACL, reuses the
`TestInbox_HiddenRowsChangeNothingRealACL` fixture).
4. **Summary cost does not grow with rows.** Store calls for the summary are
equal at 10 and 50 rows of each kind, with one `ListWaiting`, one `ListReads`,
one `Reads`, one `ListAddressed`, at most one `ResolveHeaders`, and zero `Get`,
`Head`, `List` and `ListEntity`. Test: `TestInboxSummaryBudget_10vs50`.
5. **One inbox computation per principal per round.** A full round in which
20 principals each hold a summary subscription and a default page subscription
(two tabs each, so 80 subscriptions) makes exactly 20 `Service.Inbox` calls. A
page subscription with non-default `limit` or `kinds` adds one call for that
principal. Test: `TestLive_InboxRoundMemo` with a counting `inboxReader`.
6. **The round stays inside a stated time bound.** On the perf project
(`prototypes/perf/project`, seeded to 1000 channels, 20 principals with summary
and page subscriptions), one full round takes under 2 s on fs and under 500 ms
on postgres, measured by `BenchmarkLive_InboxRound` (`-tags postgres` for the
second). The numbers are recorded in the implementation checklist before review;
a miss blocks review and reopens the grid size decision.
7. **Live paths accept the two shapes and nothing more.** `PUT` accepts
`/api/v1/_inbox`, `/api/v1/_inbox?limit=10&kinds=waiting,mention` and
`/api/v1/_inbox/summary`. It refuses with 400 `cursor`, `q`, any unknown key, a
repeated key (`kinds=a&kinds=b`, `limit=5&limit=6`), any query on the summary,
`/api/v1/_inbox/other` and a bad `limit` or `kinds` value. Equal requests
normalise to one URL: `kinds=mention,waiting` and
`kinds=waiting,mention,waiting` share one replay pair, and so do `limit=05` and
`limit=5`. Test: rows in `TestParseLivePath` and `TestLive_PutValidation`;
`TestLivePath_InboxCanonical`.
8. **Replay equals the GET.** For both shapes the replay hash equals the
hash of the caller's own GET, for row gating, a redacted field and the
principal_property mapping. Test: new paths in `TestLive_ReplayMatchesGet`.
9. **A mention frames the target only.** Bob posts a message that mentions
Alice in a channel of a type without a waiting state and outside Alice's window.
Alice's inbox and summary subscriptions get a frame; Carol's do not, and Carol's
inbox is not replayed. A `kinds=waiting` subscription of Alice is not replayed
either. Test: `TestLive_InboxFramesOnMention`.
10. **Own writes keep the wake set current.** Alice posts first in a
non-waiting channel she had no marker on; Bob then posts there. Alice's inbox
and summary frame (unread_channels moved). Likewise after Alice's first `PUT
_read` on an empty channel. Bob's own mark read does not replay Alice's inbox.
Test: `TestLive_InboxOwnPostLearnsChannel`, `TestLive_InboxFramesOnOwnMarkRead`.
11. **Activity in the window frames.** A new message by Bob in a channel in
Alice's read window (non-waiting type) frames Alice's inbox. A message in a
channel of a non-waiting type outside her window does not replay it. Test:
`TestLive_InboxWindowTopics`.
12. **Session moves frame.** A post that moves a session into a waiting
state where Alice holds the waiting role frames her inbox. A timer that moves a
session out of it frames it again. Test: `TestLive_InboxFramesOnSessionMove`
(one case per writer: person post, timer).
13. **Typing does not replay inboxes.** Typing start, typing expiry, an
ephemeral signal and its expiry in a waiting-type channel replay no inbox
subscription, while the channel head subscription on that channel still frames.
Test: `TestLive_InboxIgnoresEphemeral` with a replay counter;
`TestLocal_DurableKeys` (realtime).
14. **Hidden changes send no frame.** With a replay counter around the
replay handler, per case on a channel Alice cannot read (row-hidden anchor, or
`read:` rule denies): (a) a plain message in a non-waiting channel outside her
window: no replay, no frame; (b) a message that mentions Alice: replay runs, no
frame; (c) a session move in a waiting-type channel: replay runs, no frame.
Test: `TestLive_InboxHiddenChangeNoFrame`.
15. **Frame timing does not depend on hidden changes.** A visible mention
at t0 + 1 s frames on the same tick whether or not a hidden change (hidden
mention, hidden waiting-type message, graph write to a hidden type) happened at
t0. Inbox replays happen only on grid ticks, for topic rounds and full rounds
alike; a selected subscription waits in pending until the next grid tick and
gets no fallback frame. The PUT baseline does not wait for the grid. Test:
`TestLive_InboxGridTimingIndependent`, `TestLive_InboxOnlyOnGrid`.
16. **The safety replay runs on the grid.** With no change in the process,
every inbox subscription with a wake set replays on every 30th tick; a write
made directly in the channel store (simulating another process) frames by then.
Subscriptions without a wake set are not replayed. Test:
`TestLive_InboxSafetyReplay`.
17. **The wake set comes only from a 200.** With realtime off the inbox
subscription replays to 404, learns no wake set, and a channel change does not
replay it. A full round (graph write) still replays it at the next grid tick, so
turning realtime on frames it. A replay that did not finish keeps the previous
wake set. Test: `TestLive_InboxWakeSetFromReplay`.
18. **A resync replays inbox subscriptions.** When the channel subscription
lost count (resync), every inbox subscription with a wake set replays at the
next grid tick; one without a wake set does not. Test: case in
`TestLive_ResyncSkipsTopiclessSubscriptions`, rows in `TestRoundFilter_Selects`.
19. **The hub takes addressed targets and durable keys.**
`Local.SubscribeLive` delivers channel changes, durable keys, reads tagged by
reader, and addressed targets; an overflow of any of them resyncs. Test:
`TestSubscribeLive` (realtime).
20. **Sidebar shows the Inbox item with a badge.** The item shows under
"Search" when the channel schema store has loaded at least one channel type, so
it does not pop in after the summary request. The badge shows the badge count
with tone `warning` when `waiting > 0`, else tone `new`. With only unread
channels it shows a dot (tone `info`, no count). With nothing it shows no badge.
A 404 or 403 `no_principal` from the summary hides the item. Test:
`Sidebar.inbox.test.ts`, `inboxStatus.test.ts` (pure mapping).
21. **The badge moves live and keeps held content on transient errors.** A
`live:changed` frame for the fixed key `inbox-summary` refetches the summary. A
refusal for which `shouldDropHeldContent(err)` is true drops the badge; a
network error or 5xx keeps the last badge. The key is registered once and keeps
its place in the subscription map. Test: `useInboxSummary.test.ts`.
22. **The inbox page lists three sections.** `/inbox` (and
`/s/:space/inbox`) shows "Waiting on you", "Mentions and replies" and "Unread
channels", each row linking to its channel, with empty states and a "Showing 50
of N" note (with "20000+" when truncated). Test: `InboxView.test.ts`,
`RlInbox.test.ts`.
23. **Rows open the channel, open or closed.** A row links to
`/entity/<entity_type>/<anchor>?channel=<type>[/<name>]`. The entity page
accepts both `case` and `case/` for a cardinality-one channel. It opens the
channel when it is in the open list; when it is not, it switches the filter to
all once and opens it from there (closed and archived channels). It scrolls the
channel panel into view. An unknown value selects nothing and makes no request
with it. The query is read when the section loads and when it changes on the
same page. Test: `ChannelsSection.deeplink.test.ts` (open, closed, archived,
both key forms, unknown, query change).
24. **Mark read per row.** "Mark read" on an unread channel row sends
`PUT .../_read` with `last_seq`. On a mention row it sends the mention's `seq`:
this also marks every earlier message in that channel read, and on a channel
without a marker it creates one, so the channel can later appear under "Unread
channels". The page refetches. A failure goes through `classifyChannelError`.
Test: `InboxView.test.ts`, including a mention on a channel without a marker
(`TestInbox_MentionMarkReadCreatesMarker` in dataentry pins the server side).
25. **Strings are localized.** Every new Messages key has an en default in
`useMessages.ts` and an nl entry in `messages.nl.ts`; every new app-level string
is in `en.json` and `nl.json` with plural pairs in both; the bare-strings
ratchet does not grow. Test: the type check of `messages.nl.ts` (typed full
`Messages`), the i18n key parity test, and the bare-strings check.
26. **Stories cover the states.** `RlInbox.stories.ts` has Default, Empty,
Only unread, Degraded, Long titles and Many (truncated note) stories; the
sidebar story gains an Inbox badge variant. Test: Storybook build in CI and the
story smoke test.
27. **End to end.** Two browser contexts: Bob mentions Alice; Alice's badge
shows 1 without a reload; she opens the inbox, clicks the row, lands on the
channel, and the badge clears. Test: new case in `e2e/tests/channels.spec.ts`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (inbox research; option g1 and slice table row 4;
SPA surface in section i).

**Existing Solutions:**
- No library. The live hub (`internal/dataentry/live.go`,
`live_topics.go`) already does replay-and-hash with topic rounds; the inbox
becomes one more shape.
- Fixed clock rule: `live.go:569-578` (a round's start never depends on
when a change arrived). The inbox grid extends it.
- Topics learned from a 200 replay: `recordTopic` and `applyTopic`
(`live_topics.go:153`, `:163`), called by the channel handler
(`channels_handler.go:326`, `:337`). The inbox copies this.
- The MCP watch already wakes an inbox on the same signals
(`internal/mcp/tools_watch.go:206` `addressed` with `mentionKinds`, `:233`
`learnInbox`, `tools_inbox.go:22-29` `inboxPollEvery` 15 s and `inboxWakeGap` 2
s). The hub follows the same rule, on a fixed grid.
- `InboxView.WaitingTypes` and `WakeKeys` (`internal/realtime/inbox.go:154`)
exist for that watch. The summary and the hub reuse them unchanged.
- Read markers reach the hub tagged by reader through
`Subscription.TakeReads` (`local.go:191`) and `roundFilter.readTopics`
(`live_topics.go:84`). Today only `MarkRead` calls `markFor` (`reads.go:79-81`);
`Post` appends with `MarkRead: true` (`service.go:421-424`) but does not call
it. This slice adds that call.
- Addressed targets: `Local.markAddressed` (`local.go:261`) and
`Subscription.TakeAddressed` (`local.go:172`), today only for
`SubscribeAddressed` (MCP).
- Ephemeral marks: `typing.go:75`, `service.go:588,592` (`postSignal`),
`local.go:384-385` (`expire`), `local.go:436-448` (`dropAnchor` of ephemeral and
typing state).
- Budget tests: `TestInboxBudget_10vs50`
(`internal/dataentry/inbox_handler_test.go:234`).
- Hidden-row equality: `TestInbox_HiddenRowsChangeNothingRealACL`
(`inbox_handler_test.go:287`).
- SPA live: `registerLive(key, path, onChange)`
(`useLiveSubscription.ts:133`); registering an existing key replaces it in
place, so a fixed key keeps its slot among the 16 sent
(`useLiveSubscription.ts:59`). `useNavStatus.ts` shows the Pinia Colada query
pattern for a sidebar badge shared across components.
- SPA sidebar: pinned group in `Sidebar.vue:206`, `toNavGroups` with a
status lookup (`sidebarNav.ts:104`), `RlNavStatus` (99+ cap). Icon `inbox`
exists in `icons.ts:36`. `ChannelsPanel.vue` gates on the channel schema store;
the sidebar uses the same store for visibility.
- SPA channel list: `ChannelsSection.vue:46,52-55` (filter `open` by
default), `channelKey` returns `case/` for a cardinality-one channel
(`channelView.ts:497`), `parseChannelKey` (`channelView.ts:538`).
- SPA mark read: `markChannelRead(target, seq)` and `useMarkRead.ts`.
- Unread wording: `channelUnreadCount` in `useMessages.ts:517` and
`messages.nl.ts:212`.
- The mockup (main checkout, untracked) `fixtures/realtime.ts` and
`RlRealtimeEntityChannels.vue`: a "Needs you" item with `status: { tone:
'warning', label: '2 waiting for you', count: 2 }`, and rows with icon, title,
type, last line and an unread `RlNavStatus`. The stories copy the look, not the
files.
- Reference: Slack and GitHub notifications both separate "needs action"
from "unread" and count only the first in the badge.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Summary projection (realtime).** In `internal/realtime/inbox.go`:

   ```go
   // InboxSummary counts what an inbox first page holds, for a badge.
   type InboxSummary struct {
       Waiting          int
       WaitingTruncated bool
       UnreadChannels   int
       Mentions         int
       Degraded         bool
   }

   // Summary projects v. It reads only fields the first page already
   // carries, so it discloses nothing the page does not.
   func (v InboxView) Summary() InboxSummary
   ```

A nil `UnreadCount` or `MentionCount` counts 0. Also add `Reader channels.Reader
\`json:"-"\``to`InboxView`, set from `callReader(c)`in`Service.Inbox`, so the
handler can report whose inbox the answer is. No new `Service` method (plimsoll
cap 42 holds).

2. **Summary route (dataentry).** In `inbox_handler.go`:
   - `inboxSummaryPath = "/api/v1/_inbox/summary"`.
   - `serveInboxSummary` does the same off check and `allowMethod(GET)`,
then reads the inbox with `realtime.InboxRequest{}` (the default page, so the
round memo can share it with a default page replay), the same `ErrRealtimeOff`
and `writeChannelError` handling, then `writeV1JSON(w, 200,
wireInboxSummary(got.Summary()))`. The summary ignores its query string. The
live path refuses any query on it (step 3).
   - Both handlers read through `h.inboxFor(ctx, req)` (step 6) and call
`recordInbox(ctx, got, req)` before writing a 200 (step 4).
   - Register in `api_v1.go` next to line 173:
`mux.HandleFunc(inboxSummaryPath, ...)`, with `//archplan:auth=required`.
   - Wire type in `internal/apiwire/v1/channels.go`:

     ```go
     type InboxSummary struct {
         Waiting          int  `json:"waiting"`
         WaitingTruncated bool `json:"waiting_truncated"`
         UnreadChannels   int  `json:"unread_channels"`
         Mentions         int  `json:"mentions"`
         Degraded         bool `json:"degraded"`
     }
     ```

   - OpenAPI: `addInboxSummaryPath` in `internal/openapi/channels.go`,
called beside `addInboxPath`, operation `getInboxSummary`.
   - Wire golden: add `InboxSummary` to
`testdata/channel_wire_fields.json` and `channel_wire.json`; the TypeScript
mirror `InboxSummary` in `frontend/src/types/channels.ts` with a `wireKeys` row,
checked by `wireFields.test.ts`.

3. **Live paths (`livepath.go`).** Add two kinds, parsed before the check
that refuses `_`-prefixed plurals:
   - `liveInbox`: path `/api/v1/_inbox`. Allowed keys `limit` and `kinds`,
each at most once (a repeated key is 400). `limit` is parsed with `strconv.Atoi`
and checked with `realtime.CheckInboxLimit`; `kinds` is split on commas and
checked with `realtime.CheckInboxKinds`. `cursor`, `q` and any other key are
4. `cursor` is refused because a later page is positioned by a cursor from an
earlier answer, and a frame on it would add nothing the first page does not
show.
- `liveInboxSummary`: path `/api/v1/_inbox/summary`. Any query is 400.
- `livePath.String()` writes the canonical form: `limit` as
`strconv.Itoa` of the parsed value, `kinds` sorted and without duplicates, keys
sorted. Equal requests therefore share one replay pair.
- `livePath` gains `inbox bool`; `liveTarget` and `liveSub` carry it.

4. **Wake set (hub).** In `live_topics.go`:

   ```go
   // inboxWake is what an inbox subscription listens to. Its replay
   // reports it on a 200 only; it never comes from the client.
   type inboxWake struct {
       user         string          // principal key: addressed targets
       reader       channels.Reader // own marker writes
       mentionKinds bool            // kinds ask for mention or reply
       types        map[string]bool // channel types with a waiting state
       keys         map[string]bool // channelTopic of each WakeKey
   }

   func recordInbox(ctx context.Context, v realtime.InboxView, req realtime.InboxRequest)
   ```

   - `topicProbe` gains `inbox *inboxWake`; `replayResult` carries it.
`recordInbox` builds the sets from `v.Reader`, `v.WaitingTypes`, `v.WakeKeys`
(`channelTopic(entity.FormatStateRef(k.ID, k.Face), k.Type, k.Name)`) and
`req.Kinds` (empty or containing `mention` or `reply` sets `mentionKinds`; the
summary always sets it). `WakeKeys` holds at most `MaxReadWindow` (500) plus
`MaxInboxMentions` (50) keys, so the map is bounded.
   - `applyTopic` handles `s.inbox` like `s.channel`: a 200 sets
`s.wake = res.inbox`, another status clears it, an unfinished replay keeps it.
   - `roundFilter` gains `durable map[string]bool` (the channel topics of
the durable keys), `types map[string]bool` (the types of the durable keys) and
`addressed map[string]bool` (from `TakeAddressed`), plus `safety bool` (step 7).
   - `selects` for an inbox subscription:

     ```go
     if s.inbox {
         w := s.wake
         if w == nil {
             return false
         }
         if f.allChannels || f.safety || len(f.readTopics[w.reader]) > 0 ||
             w.mentionKinds && f.addressed[w.user] {
             return true
         }
         for t := range f.types { if w.types[t] { return true } }
         for t := range f.durable { if w.keys[t] { return true } }
         return false
     }
     ```

The loops run over the round's durable changes, which
`realtimelimits.MaxSubscriberChanges` bounds. Typing and signals never enter
`durable` or `types`, so they replay no inbox.
- `channelChanges` gains `TakeDurable() []channels.Key` and
`TakeAddressed() []string`. The worker calls them next to `TakeReads` and fills
the filter. A non-empty durable, addressed or reads set starts a filtered round
like keys do today.
- In `collect`, the rule that marks a channel subscription for recheck
also covers `s.inbox`.

5. **realtime changes.**
   - `Local.SubscribeReads` becomes `Local.SubscribeLive`, and
`Service.SubscribeReads` becomes `Service.SubscribeLive` (count stays 42; update
the plimsoll comment at `service.go:63`). It sets `allReaders`, `allAddressed`
and `allDurable`. Callers: `channels_handler.go:139` and the tests in
`internal/realtime/reads_test.go` and `mentions_test.go`. `SubscribeAddressed`
stays for the MCP watch.
   - Durable keys: `Subscription` gains a `durable` set filled by `mark`
and not by a new `markEphemeral`. `markEphemeral` adds to `changed` only, so
channel heads still see typing. Sites that switch to `markEphemeral`:
`typing.go:75`, `postSignal` (`service.go:588`, `:592`), `Local.expire`
(`local.go:384-385`), and the ephemeral and typing parts of `dropAnchor`
(`local.go:436-448`). Every other `mark` call stays durable.
`Subscription.TakeDurable()` returns and empties the set; overflow uses the same
cap and resyncs.
   - Own marker writes: in `Service.Post`, after a successful `Append`
with `MarkRead: true`, call `s.local.markFor(reader, c.key)` with the caller's
reader (when `callReader(c)` yields one). In `Service.MarkRead`, call `markFor`
after every successful `SetRead`, not only when `moved`, so a first marker at 0
also wakes the inbox (the read rate limit bounds it). Check `Edit` and
`CreateMany`: if either writes the caller's marker, it calls `markFor` too; a
test pins each path that does.

6. **Round memo.** `replayRound` attaches a fresh `inboxMemo` to the
context of every replay in the round (also the PUT baseline round). It maps
`(principal.ShareKey(), canonical InboxRequest)` to a `sync.OnceValues` that
calls `Service.Inbox` once. `channelsHandler.inboxFor(ctx, req)` uses the memo
when the context has one and calls the service directly otherwise (a normal
GET). The memo is dropped when the round ends. The shared `InboxView` is
read-only for its users; the race detector in CI covers the parallel replays
(`liveParallel` 4).

7. **Inbox grid and safety replay.** Constants `liveInboxGrid = 4` ticks
and `liveInboxSafety = 30` ticks. In `collect`:
   - On a tick where `h.ticks.Load() % liveInboxGrid != 0`, a selected
inbox subscription goes to `h.pending` and is not replayed. It gets no fallback
frame (deferred, not skipped). This holds for topic rounds and full rounds.
   - On a grid tick, pending inbox subscriptions are collected with the
rest.
   - The PUT baseline (`kick`, changed = false) is not gated; its timing
depends only on the client's own PUT.
   - In `run`, on a tick where `ticks % liveInboxSafety == 0` and some
subscription is an inbox subscription, the filter sets `safety` and a filtered
round runs even when nothing else changed.
   - `hasPending` already makes a tick run a filtered round; deferred inbox
subscriptions use that path. The ticker itself always runs.
   - No per-subscription timer state: replays are shared per (principal,
URL) pair, and the grid is global, so no replay moves the next one.

8. **SPA data layer.**
   - `src/api/channels.ts`: `getInbox(params?: {limit?, kinds?})` and
`getInboxSummary()`.
   - `src/api/live.ts`: `liveInboxPath(params?)` and
`liveInboxSummaryPath()`.
   - `src/composables/useInboxSummary.ts`: a Pinia Colada query, key
`['inbox-summary']`, shared by the sidebar and the page. Enabled when the
channel schema store has channel types. On a 200 the composable calls
`registerLive('inbox-summary', liveInboxSummaryPath(), refetch)` once (a fixed
key, so it keeps its slot; registering again replaces in place). On a 404 or 403
`no_principal` it sets `hidden` and unregisters. On an error where
`shouldDropHeldContent(err)` is true it clears the data. On any other error it
keeps the last data. It also refetches on the app's `refresh` event and when the
tab becomes visible (`runWhenVisible`, as `useNavStatus.ts`). The registration
is owned by `Sidebar.vue` (mounted once for the app's life) and removed in its
`onBeforeUnmount`.
   - `src/composables/useInbox.ts`: page data, same pattern as
`useEntityChannels.ts`: fetch `getInbox()`, subscribe `liveInboxPath()` while
mounted (`useLiveSubscription`, which is correct inside a component), refetch on
a frame, drop held content only when `shouldDropHeldContent(err)`. After a page
fetch it also writes the counts into the `['inbox-summary']` query cache, so the
badge matches the page without a second request.
   - `src/components/common/inboxStatus.ts`:
`inboxStatus(s: InboxSummary, t): NavItemStatus | undefined`. Count = `waiting +
mentions`. Tone `warning` when `waiting > 0`, else `new`. Label: the non-empty
parts of `t('nav.inboxWaiting', waiting)` and `t('nav.inboxMentions', mentions)`
joined with ", ". When the count is 0 and `unread_channels > 0`: tone `info`, no
count, label `t('nav.inboxUnread', unread_channels)`. Else undefined.

9. **Sidebar.** In `Sidebar.vue` the pinned items become Search, Inbox,
Analysis. Inbox is shown when the channel schema store has loaded channel types
and the summary has not marked it hidden: `{ label: t('nav.inbox'), href:
'/inbox', icon: 'inbox', status_key: 'inbox' }`. `toNavGroups` for the pinned
group gets a `statusFor` that returns `inboxStatus(...)` for `'inbox'`.
`activeNavId` highlights it on `/inbox`. The store loads once at sidebar mount
(`ensureLoaded`), so the row appears with the rest of the pinned rows on a warm
load and only the badge arrives later.

10. **Route and page.** `router/index.ts`: `{ path: '/inbox', name:
'inbox', component: () => import('@/views/InboxView.vue') }`, inside the routes
the `/s/:space` alias copies. `InboxView.vue` builds view rows from the wire:
    - Waiting row: anchor title, entity type label (schema store), channel
title or type label (channel schema store), session label or state, `since`
formatted with `formatDatetime`, link.
    - Mention row: author display name, kind, channel and anchor titles,
`at`, link, "Mark read".
    - Unread row: channel and anchor titles, `channelUnreadCount`, link,
"Mark read".
    - Link: `spaceStore.href(\`/entity/${entity_type}/${encodeURIComponent(anchor)}?channel=${encodeURIComponent(channelKey(type, name))}\`)`.
`anchor`carries`@face`for a faced type, as`InboxChannel.anchor` renders it; the
entity route accepts that form.
    - Mark read: `markChannelRead({type: entity_type, address: anchor,
channelType: type, name}, seq)` with `last_seq` for unread rows and `seq` for
mention rows, then refetch. Errors through `classifyChannelError`; toast per its
rule.
    - Page title and the load error notice are app strings (vue-i18n).
    - `degraded` shows the `inboxDegraded` notice.

11. **Channel deep link.** `ChannelsSection.vue` watches
`route.query.channel` (immediate). It parses the value with `parseChannelKey`,
which accepts `case` and `case/`, and rebuilds the key with `channelKey`. When
the rows have loaded:
    - if a row has that key, set `selected`;
    - else, if the filter is `open` and this value has not tried `all`
yet, set the filter to `all` and try again when the rows reload;
    - else select nothing.
After selecting, scroll the section root into view (`scrollIntoView({block:
'start'})`), since the panel sits below the comments (`EntityDetail.vue:3157`).
Selecting another channel does not rewrite the URL.

12. **Library component.** `rela-components/src/components/channel/RlInbox.vue`
with props `waiting: InboxWaitingRow[]`, `mentions: InboxMentionRow[]`, `unread:
InboxUnreadRow[]`, `waitingMore?: {shown: number, total: string}`, `degraded?:
boolean`, `busy?: Set<string>`. Rows carry pre-formatted strings and an
`as`/`attrs` link like `NavItem`, so the library stays free of routing, fetching
and dates. Emits `markRead(rowId)`. Types in `components/channel/types.ts`,
exported from the package index. Messages keys (en default, nl):

    | Key | en | nl |
    | --- | --- | --- |
    | `inboxWaitingHeading` | Waiting on you | Wacht op jou |
    | `inboxMentionsHeading` | Mentions and replies | Vermeldingen en reacties |
    | `inboxUnreadHeading` | Unread channels | Ongelezen kanalen |
    | `inboxWaitingEmpty` | Nothing waits on you. | Er wacht niets op jou. |
    | `inboxMentionsEmpty` | No unread mentions or replies. | Geen ongelezen vermeldingen of reacties. |
    | `inboxUnreadEmpty` | All channels are read. | Alle kanalen zijn gelezen. |
    | `inboxMentioned({author})` | {author} mentioned you | {author} heeft je genoemd |
    | `inboxReplied({author})` | {author} replied to you | {author} heeft op je gereageerd |
    | `inboxMarkRead` | Mark read | Markeer als gelezen |
    | `inboxShowing({shown, total})` | Showing {shown} of {total} | {shown} van {total} getoond |
    | `inboxDegraded` | Some channel settings could not be loaded. The list may be incomplete. | Sommige kanaalinstellingen konden niet worden geladen. De lijst is mogelijk onvolledig. |

`total` is a preformatted string, so the app passes `"20000+"` when `truncated`
is set.

App strings (`en.json` / `nl.json`):

    | Key | en | nl |
    | --- | --- | --- |
    | `nav.inbox` | Inbox | Inbox |
    | `nav.inboxWaiting` | {n} waiting \| {n} waiting | {n} wachtend \| {n} wachtend |
    | `nav.inboxMentions` | {n} mention \| {n} mentions | {n} vermelding \| {n} vermeldingen |
    | `nav.inboxUnread` | {n} unread channel \| {n} unread channels | {n} ongelezen kanaal \| {n} ongelezen kanalen |
    | `inbox.title` | Inbox | Inbox |
    | `inbox.loadError` | The inbox could not be loaded. | De inbox kon niet worden geladen. |

13. **Stories.** `RlInbox.stories.ts` next to the component, with inline
sample rows shaped like the mockup (incident room, design review, approvals).
Add an `InboxBadge` story to the existing sidebar stories with the mockup's
pinned "Inbox" item.

14. **Error codes.** No new error code. Update the comment in
`channelErrors.ts` for `invalid_cursor` and `inbox_too_large` to say the inbox
page now reads the inbox; `inbox_too_large` stays `TOAST`, and the page shows
the message from the problem detail. `invalid_cursor` cannot occur from the page
(it sends no cursor) and keeps its rule. `no_principal` keeps `TOAST` for
actions; the summary composable treats it as "hide the item".
`channelErrors.test.ts` keeps checking the list against
`internal/realtime/errors.go`.

**Alternatives rejected:**
- *A Local hint `i:sessions` marked at every session move (research
text).* Session moves happen in Post, the system writer, timers, `CreateMany`,
lifecycle reopen, promote and the outputs runner. Missing one site loses a frame
silently. Listening to durable changes of waiting types needs no new call sites
and matches the MCP watch.
- *A 2 s gap per subscription, anchored at its last replay.* A hidden
change would start a replay and push back the next visible frame, which is a
timing oracle. The global grid has no such state.
- *Per-principal topics `i:<key>` sent by `realtime.Local`.* The hub
already gets reads tagged by reader and can get addressed targets; it maps both
to the inbox subscription itself. A new Local topic API would add a method to a
type at its plimsoll cap.
- *A separate SSE event `inbox:changed` with counts.* It would bypass the
replay-and-hash rule and need its own gate. Rejected by the research.
- *Badge counts unread channels too.* Unread chat in many rooms would keep
the badge lit; the research names the item "Needs you". Unread channels get a
dot.
- *Drop the summary subscription while the page is mounted.* The round memo
already removes the second computation on the server, and the sidebar stays
simple.
- *A module-level singleton around `useLiveSubscription`.* It registers
`onBeforeUnmount` outside a component and makes a new key per call, so the badge
would lose its slot first. The Colada query plus a fixed key avoids both.
- *Cursor paging live.* Not useful for a badge-first page; out of scope.

**Dependencies:** No new Go or npm packages. Types used: `realtime.InboxView`,
`channels.Reader`, `channels.Key`, `v1.InboxSummary` (new), `NavItemStatus`,
`ChannelTarget`, Pinia Colada `useQuery`.

**Files to modify:**
- `internal/realtime/inbox.go` (Summary, InboxSummary, Reader field)
- `internal/realtime/local.go` (SubscribeLive, durable set,
markEphemeral, TakeDurable)
- `internal/realtime/service.go` (SubscribeLive, plimsoll comment, Post
markFor, postSignal markEphemeral)
- `internal/realtime/typing.go` (markEphemeral)
- `internal/realtime/reads.go` (markFor on every SetRead, doc reference)
- `internal/realtime/inbox_test.go`, `local_test.go`, `reads_test.go`,
`mentions_test.go` (rename, new tests)
- `internal/apiwire/v1/channels.go` (InboxSummary)
- `internal/openapi/channels.go` (summary path)
- `internal/dataentry/inbox_handler.go` (summary handler, inboxFor,
recordInbox calls)
- `internal/dataentry/api_v1.go` (route)
- `internal/dataentry/livepath.go`, `livepath_test.go`
- `internal/dataentry/live.go` (liveSub fields, grid, safety, memo,
collect)
- `internal/dataentry/live_topics.go` (inboxWake, filter, selects,
channelChanges)
- `internal/dataentry/channels_handler.go` (SubscribeLive)
- `internal/dataentry/live_test.go`, `live_channels_test.go`,
`inbox_handler_test.go`, new `live_inbox_test.go`, new
`live_inbox_bench_test.go`, and any test fake of `channelChanges`
- `internal/dataentry/testdata/channel_wire_fields.json`,
`channel_wire.json`, `channels_wire_golden_test.go`
- `frontend/src/types/channels.ts`, `src/types/wireFields.test.ts`
- `frontend/src/api/channels.ts`, `src/api/live.ts` (+ tests)
- `frontend/src/composables/useInboxSummary.ts`, `useInbox.ts` (+ tests)
- `frontend/src/components/common/Sidebar.vue`, `inboxStatus.ts` (+ tests)
- `frontend/src/router/index.ts`
- `frontend/src/views/InboxView.vue` (+ test)
- `frontend/src/components/entity/channels/ChannelsSection.vue` (+ test)
- `frontend/src/components/entity/channels/channelErrors.ts` (comment)
- `frontend/src/i18n/locales/en.json`, `nl.json`
- `frontend/packages/rela-components/src/components/channel/RlInbox.vue`,
`RlInbox.stories.ts`, `RlInbox.test.ts`, `types.ts`, package index
- `frontend/packages/rela-components/src/composables/useMessages.ts`,
`messages.nl.ts`
- the sidebar stories file in rela-components (Inbox badge story)
- `e2e/tests/channels.spec.ts`, `e2e/pages/channels.page.ts` (inbox
helpers)
- `docs/realtime.md`, `docs/data-entry/api-reference.md`,
`docs/data-entry.md`, `docs/acl-security.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- *Summary route query string.* Ignored. No input reaches the service.
- *Live PUT path.* Allowlist in `parseLivePath`: two exact paths, keys
`limit` and `kinds` each at most once, validated by the same functions as the
route, normalised in `String()`, all else 400. Per-stream (16) and per-identity
(64) caps apply unchanged.
- *Wake set.* Server-side only: built by the handler from its own gated
answer during a replay as the subscriber, kept only after a 200. The client
never names a topic, a reader or a principal key.
- *Addressed targets, reads, durable keys.* Come from `realtime.Local`,
written by the service after it authorized the write.
- *SPA `?channel=` query.* Parsed with `parseChannelKey` and compared
against the row keys the gated channel list returned; an unknown value selects
nothing. It never builds a request path by itself.

**Security-Sensitive Operations:**
- *Summary disclosure.* The summary is a projection of the first page of
the caller's own gated inbox. Every field is already in `GET /_inbox`. It
reveals no more than polling that GET (AC 3).
- *Live frames.* A frame fires only when the caller's own replay hash
changes. A wake only schedules a replay; the replay re-gates. A hidden change
either does not replay or replays with the same hash (AC 14).
- *Frame timing.* Inbox replays happen only on the global grid (every 4th
tick of the fixed 500 ms clock) and the safety replay on every 30th tick. No
state per subscription or per pair moves the next replay, so a hidden change
cannot delay or advance a visible frame (AC 15). This is the `live.go:569-578`
rule applied to the inbox. Round overflow (256 pairs, 10 s budget) behaves as
for every other shape.
- *Wake set as a side channel.* Selection is never visible to the client;
only a hash change is.
- *Addressed targets of others.* The hub sees every principal key a post
addresses. It uses them only to pick subscriptions and never sends them.
- *No cache across principals.* The round memo is keyed by
`principal.ShareKey()` and the canonical request, and lives for one round.
Replays otherwise run per (principal, path) pair as today. The SPA query holds
only the logged-in user's summary. Sign out and switching organisation are
full-page navigations (`AccountMenu.vue:93,98`), which drop all module and query
state; no reset hook exists or is needed.
- *Cost.* Summary budget equal at 10 and 50 (AC 4); one inbox computation
per principal per round (AC 5); a time bound per round (AC 6); typing replays no
inbox (AC 13).
- *Errors.* The summary returns the inbox's error codes with the same
titles. `inbox_too_large` keeps its documented disclosure; the summary adds no
new signal because it makes the same call. 404 with realtime off names no
entity.
- *Config is not secret.* `WaitingTypes` comes from `realtime.yaml`; it is
config, never sent, and needs no concealment. The sidebar item's visibility from
the channel schema is a UX choice, not concealment.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Package |
| --- | --- | --- |
| 1 | `TestInboxView_Summary`, `TestInboxSummaryHandler_Shape` | realtime, dataentry |
| 2 | `TestInboxSummaryHandler_Errors` | dataentry |
| 3 | `TestInboxSummary_HiddenRowsChangeNothing` | dataentry |
| 4 | `TestInboxSummaryBudget_10vs50` | dataentry |
| 5 | `TestLive_InboxRoundMemo` | dataentry |
| 6 | `BenchmarkLive_InboxRound` (fs, postgres) | dataentry |
| 7 | `TestParseLivePath`, `TestLive_PutValidation`, `TestLivePath_InboxCanonical` | dataentry |
| 8 | `TestLive_ReplayMatchesGet` rows | dataentry |
| 9 | `TestLive_InboxFramesOnMention` | dataentry |
| 10 | `TestLive_InboxOwnPostLearnsChannel`, `TestLive_InboxFramesOnOwnMarkRead` | dataentry |
| 11 | `TestLive_InboxWindowTopics` | dataentry |
| 12 | `TestLive_InboxFramesOnSessionMove` | dataentry |
| 13 | `TestLive_InboxIgnoresEphemeral`, `TestLocal_DurableKeys` | dataentry, realtime |
| 14 | `TestLive_InboxHiddenChangeNoFrame` | dataentry |
| 15 | `TestLive_InboxGridTimingIndependent`, `TestLive_InboxOnlyOnGrid` | dataentry |
| 16 | `TestLive_InboxSafetyReplay` | dataentry |
| 17 | `TestLive_InboxWakeSetFromReplay` | dataentry |
| 18 | `TestLive_ResyncSkipsTopiclessSubscriptions`, `TestRoundFilter_Selects` | dataentry |
| 19 | `TestSubscribeLive` | realtime |
| 20 | `Sidebar.inbox.test.ts`, `inboxStatus.test.ts` | frontend |
| 21 | `useInboxSummary.test.ts` | frontend |
| 22 | `InboxView.test.ts`, `RlInbox.test.ts` | frontend, rela-components |
| 23 | `ChannelsSection.deeplink.test.ts` | frontend |
| 24 | `InboxView.test.ts`, `TestInbox_MentionMarkReadCreatesMarker` | frontend, dataentry |
| 25 | type check, i18n parity, bare-strings ratchet | frontend |
| 26 | Storybook build and smoke test | rela-components |
| 27 | `channels.spec.ts` inbox case | e2e |

Integration: the live tests use `newLiveEnv` with the real hub, the real
`realtime.Service` and `Local`, and the real handlers, driving ticks through the
hub's test clock. A replay counter wraps the replay handler for AC 5, 13, 14 and
15. AC 3 uses the real declarative ACL and redactor. The e2e case runs the built
server with two browser contexts.

**Edge Cases:**
- Caller with no waiting role, no markers and no mentions: summary all
zeros, sidebar item without badge, page shows three empty states.
- Caller without identity: summary 403 `no_principal`; item hidden.
- `degraded: true`: summary passes it; the page shows the notice; the badge
still shows counts.
- `waiting_truncated: true` (over 20 000): badge shows 99+ (RlNavStatus
cap); the page shows "Showing 50 of 20000+".
- Badge count over 99: `99+`.
- Realtime turned off by a `realtime.yaml` reload while the page is open:
full round, replay 404 at the next grid tick, frame, refetch 404, item hidden,
page shows the off notice.
- Realtime turned on: the channel schema store reloads on `refresh`, the
item appears, and the summary query runs.
- `WakeKeys` empty (no markers): selection still works by types, addressed,
reads and the safety replay.
- Two tabs of one user: one replay pair per (user, path) and one memo entry;
both tabs get frames.
- Sidebar and page subscribe to different paths: two pairs, one
`Service.Inbox` call per round.
- Faced anchor (`TKT-1@public`): link and mark read use the anchor as the
wire renders it; deep link opens the right face page.
- Mention in a channel later removed or the channel deleted: the durable
key is in `WakeKeys` and frames; the row disappears.
- Deep link to a closed channel: the filter switches to all once and the
channel opens. Deep link to a channel deleted meanwhile: nothing opens, filter
stays at all.
- A channel row whose type is gone from the schema store: the row shows the
raw type name.
- Mark read while a frame arrives: both refetch; the later answer wins
(fetch sequence guard as in `useEntityChannels`).
- Subscription budget: the badge takes one fixed slot, registered early;
the page adds one. The worst case (entity page with channels plus sidebar) stays
under 16; noted in `useInboxSummary.ts`.
- A deferred inbox subscription removed by a PUT before the grid tick: it
leaves pending with the subscription, as today.

**Negative Tests:**
- Live PUT with `/api/v1/_inbox?cursor=x`, `?q=x`, `?foo=1`, `?limit=0`,
`?limit=abc`, `?kinds=bogus`, `?kinds=a&kinds=b`, `?limit=5&limit=6`,
`/api/v1/_inbox/summary?limit=1`, `/api/v1/_inbox/x`: 400 each.
- Summary POST: 405. Summary with realtime off: 404 with
`realtimeOffTitle`. Summary without identity: 403 `no_principal`.
- Summary when the service returns `inbox_too_large`: same status and code
as the inbox route.
- A mention of Alice in a channel Alice cannot read: replay, no frame, and
the summary is unchanged.
- Bob marks a channel read: Alice's inbox not replayed.
- Typing in a waiting-type channel: no inbox replay.
- A hidden change at t0 does not shift a visible frame at t0 + 1 s.
- `?channel=../x` or an unknown key on the entity route: nothing selected,
no request made with it.
- Mark read on a channel that became hidden: the PUT answers like the head
GET (404); the page shows the toast rule's message and refetches; the row is
gone.
- A network error on the summary keeps the badge; a refusal that
`shouldDropHeldContent` accepts clears it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Replay cost on the fs tier.* An inbox replay walks channel files
(`ListWaiting`, `ListReads`, `ListAddressed`). Every graph write starts a full
round. Mitigation: one computation per principal per round (AC 5), inbox replays
only on the 2 s grid, typing excluded (AC 13), and a measured bound per round
(AC 6). If AC 6 fails, the decision is to widen the grid (for example 8 ticks)
for full rounds; that choice is made before review, not left open.
- *Inbox replays crowd out other shapes.* They share `liveParallel` (4)
and the 10 s round budget. Grid ticks concentrate inbox replays; AC 6 measures a
round that includes them. Other subscriptions that overflow fall back to a
refetch frame, as today.
- *Safety replay load.* Every 15 s all inbox principals replay. With the
memo this is one computation per principal per 15 s, the same cadence the MCP
watch already uses.
- *Busy waiting-type channels.* Durable chat in a channel of a waiting type
wakes every inbox with that type, at most once per grid tick.
- *Addressed flood resyncs the hub.* The addressed set holds principal keys
(few distinct values); a resync is a correct fallback, only more replays.
- *Missed own-marker sites.* If a write path other than Post and MarkRead
sets the caller's marker without `markFor`, the wake set learns the channel only
at the next safety replay (15 s). The safety replay bounds the damage.
- *Process locality.* Writes in another process (stdio MCP, CLI) reach the
badge within 15 s (safety replay). Documented; TKT-YK30D1 removes the delay.
- *Rename `SubscribeReads`.* Any branch in flight that calls it breaks at
compile time, which is the safe failure.
- *Badge semantics.* Users may expect unread chat in the count. The dot
keeps it visible. Recorded under Open questions.

**Effort:** l (hub grid, memo, safety replay and durable keys, summary route,
SPA page, deep link, library component, stories, e2e, benchmark).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes: an "Inbox" section near "Channels"
(sidebar item, badge rules, the page, deep link to open and closed channels,
mark read and its effect on earlier messages, delay of up to 15 s for writes
from other processes).
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows existing
live and gate-before-fold rules)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] docs/realtime.md - Inbox section: the summary route, live
subscriptions on both paths, what wakes them, the grid and the safety replay,
process locality; route table row for `/api/v1/_inbox/summary`.
- [x] docs/data-entry/api-reference.md - live paths list (four GETs become
six, with the inbox keys, refusals and normalisation) and the summary route.
- [x] docs/acl-security.md - "The inbox" section: the summary is a
projection of the gated first page; live frames follow the replay-and-hash rule
on a fixed grid; the wake set comes only from a 200; the round memo is per
principal and per round.

## Open questions

Each question records the decision and how it was resolved. "Design review"
means the reviewer agreed or the plan was changed to follow the review; the user
may still overrule any of them before implementation.

1. **What wakes an inbox?** Decision: durable changes in channels of a
waiting type, window keys, own marker writes (now also from Post), addressed
targets when kinds ask for them, and a 15 s safety replay. No Local session
hint. Resolution: accepted in design review, conditional on the own-post and
typing fixes, which the plan now has.
2. **How often may an inbox replay?** Decision: on a global grid, every
4th tick (2 s), for every round kind; the per-subscription gap is dropped.
Resolution: changed in design review (the gap was a timing oracle).
3. **Badge count.** Decision: waiting plus mentions and replies; unread
channels as a dot without count. Resolution: accepted in design review.
4. **Marker writes and mentions in the hub.** Decision: reuse
`TakeReads` and `TakeAddressed`; no new Local topic API. Resolution: accepted in
design review.
5. **`SubscribeReads` rename.** Decision: rename to `SubscribeLive`, which
also takes addressed targets and durable keys; `Service` stays at 42 methods.
Resolution: accepted in design review.
6. **Summary route and its cost.** Decision: a separate summary route
reading the default page through a per-round, per-principal memo, with a numeric
bound (AC 5, AC 6). Resolution: accepted in design review; memo and bound added.
7. **Badge state in the SPA.** Decision: a Pinia Colada query plus one
`registerLive` with the fixed key `inbox-summary`; visibility from the channel
schema store; hide on 404 and `no_principal`; drop held content only per
`shouldDropHeldContent`. Resolution: changed in design review (was a singleton
around `useLiveSubscription`).
8. **Deep link and page scope.** Decision: `?channel=<type>[/<name>]` on
the entity route, both key forms accepted, falls back to the all filter for
closed and archived channels, scrolls into view; no scroll to `seq`; no cursor
paging on the page or in live paths. Resolution: accepted in design review with
the closed-channel and key-form fixes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 findings in `.ignored/dr-9yrrm5.tsv` (1 critical,
5 significant, 8 minor, 5 nit). All are addressed in this plan; resolutions per
finding are in `.ignored/dr-9yrrm5-res.txt`. The review-response entities are to
be created by the coordinator.
