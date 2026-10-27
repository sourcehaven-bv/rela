---
id: PLAN-LEN9V3
type: planning-checklist
title: 'Planning: Inbox 2: read markers and unread counts'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slice 1 (TKT-ZZ4PNX) answers "which sessions wait on me". It cannot
answer "which of my channels have new messages", because rela keeps no
per-reader state for channels. RES-B0MJAZ slice 2 adds that state: one read
marker per (channel, reader), an exact unread count capped at 100, the count on
the channel list and head, a route and an MCP tool that move the marker,
`unread_channels` in the inbox, and badges in the SPA channel panel. The
decisions of 2026-10-27 fix the frame: markers live inside `channels.Store`;
counts exist only for channels the reader opened or posted in; the cap is 100;
every count is taken after the gate; nothing is cached across principals.

Terms used below:
- **Reader**: whose marker a row is. It is the pair (user, organization)
that `channels.Author` records for a post, through `authorOf`: `User` is
`principal.User` (the user entity id in the web app, the raw login over CLI and
stdio, the job identity for a job), `OrgID` the verified org claim or `""`. The
tool is not part of it, as in `sameAuthor`.
- **Marker**: per (channel, reader), the highest `seq` the reader has
read, and when it last moved (`at`). Seq 0 means "opened, nothing read".
- **Counting message**: a stored message that is not removed and is not a
session notice (a store-written message whose `triggered_by` starts with
`channels.SessionMarker`). Reminders, trigger posts and ordinary posts count.
- **Unread**: the number of counting messages with a seq above the marker,
at most 100 (`channels.UnreadCap`).
- **Last activity** (`last_at`): when the channel last gained a stored
message. A new column on `channels`; the file tier reads it from the log tail.
- **Window**: at most 500 of the reader's markers on live channels
(`channels.MaxReadWindow`). Channels with messages above the marker come first,
then the rest; within each group the channel with the newest last activity comes
first, then key order. The inbox and the watch's wake keys look only here. The
marker's own `at` plays no part in the order, because a marker that has not
moved for weeks is exactly the one most likely to sit under new messages.

**Scope:**
- In: store types and three `channels.Store` methods (`SetRead`, `Reads`,
`ListReads`) on every backend (filechannels on disk and on MemFS,
sqlitechannels, pgchannels through sqlchannels), pinned by `channelstest`.
- In: an own post moves the poster's marker, in the same store step as the
append (`NewMessage.MarkRead`). Creating a named channel records the creator's
marker at 0 (`NewChannel.MarkRead`).
- In: schema. sqlite rung v27 to v28 and postgres migration 0035: table
`channel_reads`, column `channel_messages.counts` with a backfill, column
`channels.last_at` with a backfill, and two indexes. On the file tier,
`reads.json` in each channel directory.
- In: `read: {seq, unread}` on the channel object of the channel list and
the channel head (REST, MCP `list_channels`, CLI `rela channel list`).
- In: `PUT /api/v1/{plural}/{ref}/_channels/{type}[/{name}]/_read`
(`Service.MarkRead`), wire types, golden fields, OpenAPI.
- In: MCP `mark_read`; `unread_channels`, `unread_count` and a `kinds`
filter on `list_inbox` and `GET /api/v1/_inbox`; the watch wakes on changes to
channels in the caller's window.
- In: SPA: unread badge per channel in the entity channel panel; the open
channel marks itself read while the tab is visible.
- In: CLI `rela channel mark-read`, unread in `rela channel list` and
`rela channel inbox`, because each reuses the existing channel binding.
- In: docs (`docs/realtime.md`, `docs/mcp-server.md`,
`docs/cli-reference.md`, `docs/acl-security.md`, `docs/data-entry.md`).
- In: budget tests (list, head, inbox, mark read) and an ACL test that
hidden channels change nothing.
- Out: mentions and replies, and their entries (TKT-6RXAC0).
- Out: the SPA inbox page, sidebar badge, live shape for `/_inbox` and
`i:` topics (TKT-9YRRM5). The SPA in this slice shows counts only in the entity
channel panel.
- Out: mail and desktop notifications (TKT-Z3PS9U); cross-process live
frames (TKT-YK30D1).
- Out: "mark unread" (moving a marker back), per-message read flags, public
read receipts, a per-thread marker.
- Out: erasing one reader's markers on request (a "forget reader" store
method). Markers go with their channel; see Retention. A follow-up ticket is
drafted in `.ignored/followup-2b2aia.md` for the coordinator to file and link;
this slice lists markers with the other per-user data in the docs.
- Out: per-message visibility. Realtime has none today: a reader of a
channel reads every message in it, so the channel gate is the message gate. A
later per-message gate must also gate the count.

**Example:**

```json
GET /api/v1/risicos/RISK-1/_channels
{ "anchor": "RISK-1", "types": [ { "type": "breach-response", "cardinality": "many", "create": true,
  "channels": [
    { "anchor": "RISK-1", "type": "breach-response", "name": "0k3x…", "state": "open",
      "last_seq": 42, "rev": 57, "read": { "seq": 39, "unread": 2 } },
    { "anchor": "RISK-1", "type": "breach-response", "name": "9q1a…", "state": "open",
      "last_seq": 7, "rev": 7 }
  ] } ] }
```

The first channel has a marker at 39; seqs 40 to 42 hold one message from a
colleague, its session notice and a reminder, so `unread` is 2. The second
channel was never opened by this reader, so it has no `read`.

```json
PUT /api/v1/risicos/RISK-1/_channels/breach-response/0k3x…/_read
{ "seq": 42 }
→ 200 { "seq": 42, "unread": 0, "last_seq": 42 }
```

`GET /api/v1/_inbox` (first page) adds:

```json
"unread_channels": [
  { "channel": { "anchor": "RISK-2", "entity_type": "risico", "anchor_title": "Server room",
                 "type": "breach-response", "name": "7m2c…", "title": "Breach 13", "state": "open" },
    "seq": 10, "last_seq": 14, "unread": 3,
    "links": { "entity": "/api/v1/risicos/RISK-2",
               "channel": "/api/v1/risicos/RISK-2/_channels/breach-response/7m2c…" } }
],
"unread_count": 1
```

**Acceptance Criteria:**

1. `SetRead(key, reader, seq, at)` stores `max(old, min(seq, LastSeq))`
and creates a missing marker, even at 0. It never moves a marker back. It
reports `moved` only when the stored seq rose or the marker was created; `at`
changes only then. It answers the stored state, with `Unread` counted in the
same step. A missing or tombstoned channel is `ErrNotFound`; a negative seq or
an invalid reader is `ErrInvalidMessage`; an invalid key is `ErrInvalidKey`. It
does not raise the channel's rev and writes no audit record. Test: `testSetRead`
in `channelstest` (create, raise, lower is a no-op, equal is a no-op with `at`
unchanged, beyond head clamps, empty named channel at 0, closed and archived
channels accept it, tombstoned is `ErrNotFound`, rev and `Audit` unchanged, two
readers keep separate markers, the same user in two orgs keeps two markers, 20
concurrent `SetRead` calls with random seqs end at their maximum). On
filechannels, a `SetRead` whose seq is not above the marker read without the
lock returns at once and takes no lock: `TestSetRead_NoOpTakesNoLock` (counts
lock acquisitions).
2. `Reads(reader, keys)` returns, for each key that names a live channel on
which the reader has a marker, the marker and `Unread`: the counting messages
above the marker, at most `UnreadCap`. Keys without a marker, unknown keys and
tombstoned channels are absent. More than `MaxReadKeys` keys, or an invalid key
or reader, is `ErrInvalidKey` / `ErrInvalidMessage`; an empty key list returns
an empty map without a query. Test: `testReadsCount` in `channelstest` (rows:
plain messages from another author; a removed message; a session notice from a
`SessionStep`; a timer notice; a reminder message (counts); a reply (counts); an
edited message below the marker (does not count again); 150 messages above the
marker give 100; 1 200 keys across chunks; a key after `Rekey` of its anchor
still has its marker; a key after `Tombstone` is absent, and present again after
`Untombstone`). On filechannels the cost follows the unread messages, not the
segment size: a key whose last seq is at or below the marker reads only the log
tail, and a 2 000-message channel with one unread message reads one tail chunk.
Test: `TestReads_TailCost` in filechannels (counts bytes read through a wrapping
FS).
3. `ListReads(reader, limit)` returns the reader's markers on live
channels, each with its full `Channel` and `LastAt`, in window order (see
Terms), at most `limit` (1 to `MaxReadWindow`, else `ErrInvalidMessage`). Test:
`testListReads` in `channelstest` (order: unread first, then last activity, then
key; limit; tombstoned and deleted channels absent; another reader's markers
absent; a `Rekey` keeps the marker under the new id; 501 markers where the
channel with the oldest marker is the only one with new messages: it is
returned, first).
4. An append with `MarkRead` moves its author's marker to the last seq the
append wrote (the message, or the session notice after it), in the same store
step, creating the marker. An append without it, and every timer, reminder and
trigger write, touches no marker. `CreateMany` with `MarkRead` records the
creator's marker at 0. Test: `testAppendMarksRead` in `channelstest` (post below
an existing higher marker keeps it; post that moves the session sets the marker
to the notice's seq; a cardinality-one channel's first append creates both
channel and marker; `Reads` right after shows unread 0 for the poster and 1 for
another reader at the previous head). On filechannels, a failed `reads.json`
write after the log records is logged as a warning (key and user, no content)
and swallowed: `Append` returns the stored message, so a client never retries a
post that was stored. Test: `TestAppend_MarkReadWriteFailure` in filechannels
(inject a write failure on `reads.json`; `Append` succeeds; the marker is
unchanged).
5. `Delete` and `Purge` erase a channel's markers; `Rekey`, `Tombstone`
and `Untombstone` carry them. A deleted cardinality-one channel that a later
post recreates starts without markers. Test: rows in `testSetRead` and
`testListReads`, plus a `Raw` check on the SQL backends that no `channel_reads`
row survives a `Delete` or `Purge`.
6. Upgrade: a sqlite database at v27 and a postgres schema at 34 gain
`channel_reads`, `channel_messages.counts` and `channels.last_at`. The backfill
sets `counts = 0` on removed messages and session notices and leaves every other
row at 1; a doc that is not valid JSON, or holds `\u0000`, keeps 1 and the
migration succeeds. It sets `last_at` to the latest of the channel's created,
closed, archived and reopened times. Running it again changes nothing. The
`session:` literal in both migrations equals `channels.SessionMarker`. Tests:
`TestMigrate_v27ToV28ChannelReads` in `internal/sqlitedb`,
`TestMigration0035_ChannelReads` in pgstore (schema-pinned DSN),
`TestChannelReadsMigrations_SessionMarker` (reads both migration sources), the
ladder well-formedness test, and `TestDBImportFS_SQLiteTablesAreAccounted` with
`channel_reads` listed.
7. The channel list and the head carry `read: {seq, unread}` for each
readable stored channel on which the caller has a marker, and no `read`
otherwise (no marker, an unstored cardinality-one channel, a caller with no
identity). Lifecycle, create and message-page answers carry no `read`. The head
never answers `read.seq` above `last_seq`: when the `Reads` answer has a higher
`LastSeq` (an own post landed between the two calls), the head takes it. Test:
`TestHead_ReadNotAboveLastSeq` (realtime, store fake that appends between `Head`
and `Reads`), and: `TestChannelRead_ListAndHead` in `internal/dataentry` (real
router, fsstore, filechannels; two users; one marks read, the other does not; a
post by the second user raises the first user's `unread` and leaves the poster's
at 0).
8. `PUT …/_read` with `{}` marks the channel read up to its head; with
`{"seq": n}` up to `min(n, last_seq)`; it answers 200 `{seq, unread, last_seq}`
with the stored values, which may be higher than `n`. A channel the caller
cannot read gets the same answer as the head GET (403 `read_denied` or 404) and
stores nothing. An unstored cardinality-one channel answers 200 `{seq: 0,
unread: 0, last_seq: 0}` and stores nothing. A negative or non-integer seq, an
unknown field, or trailing data is 400. Realtime off is the uniform 404. A
method other than PUT is 405 with `Allow: PUT`. Marks have their own limiter,
apart from posts: `ReadRate` per reader and channel is 2 per 2 seconds (one per
second on average, a burst of 2), under a separate per-reader aggregate
`ReadIdentityRate` (300 per minute) with its own bucket caps. A third mark
within the burst is 429 with `Retry-After`. Marks never use up the post budget
or the post limiter's 64 buckets per identity. A caller with no identity is 403
`no_principal`. Tests: `TestChannelMarkRead_Handler` table in
`internal/dataentry`; `TestMarkRead_OwnLimiter` in realtime (64 marks on 64
channels in one second, then a post: the post succeeds).
9. Gate before count, per principal: in the list, the head and the inbox,
a channel whose anchor the caller cannot read, whose `read:` role the caller
fails (named, permission and creator roles), or whose anchor face the caller
cannot read, has no `read`, is absent from `unread_channels`, and does not
change `unread_count`. The inbox answer is byte-identical to the answer for a
project where those channels and their markers do not exist. Test:
`TestInbox_UnreadHiddenChangesNothing` (realtime, fakes) and
`TestInboxUnread_ACL` (dataentry, real `acl.yaml`): the caller holds markers on
hidden channels with new messages; compare JSON.
10. Cost. The channel list makes one `Reads` call beside its
`ListEntity` (one per `MaxReadKeys` stored channels; the service chunks); the
head one `Reads` beside its `Head` and `Get`; the inbox one `ListReads` and one
`Reads` beside slice 1's calls, with one shared anchor batch and one shared role
prime for waiting and unread candidates; `PUT …/_read` one `Head` and one
`SetRead`. Each count is the same at 10 and 50 channels. Tests:
`TestChannelsBudget_List10vs50` (extended: every channel has a marker and new
messages), `TestChannelsBudget_HeadRead`, `TestInboxBudget_10vs50` (extended:
half the marker channels hidden; the read role is a relation-conferred
`permission:` role), `TestChannelsBudget_MarkRead`, `TestListAnchor_ReadsChunks`
(2 001 stored channels: two `Reads` calls, no error). The SQL plans are pinned:
the count subquery uses `channel_messages_counts_idx`, `Reads` is one statement
per 500-key chunk, and `ListReads` uses `channel_reads_reader_idx`. Tests:
`reads_explain_test.go` in sqlitechannels (the `due_explain_test.go` pattern, on
the modernc driver the binary uses) and `channel_reads_explain_test.go` in
pgstore.
11. `unread_channels` lists the gated window's channels whose unread is
above 0, in window order (newest last activity first, then key), at most 50
entries; `limit` does not bound it. `unread_count` (a pointer, so a real 0 is
sent) is the number of such channels in the gated window. Both appear on the
first inbox page only (no cursor), and only when `kinds` includes `unread`.
`kinds` is a subset of `waiting` and `unread` (REST `?kinds=waiting,unread`, MCP
array); the default is both; an unknown kind is 400 `invalid_request`. With
`kinds=waiting` the inbox makes no `ListReads` or `Reads` call; with
`kinds=unread` no holder query and no `ListWaiting`, and `entries`, `count`,
`truncated` and `next_cursor` are omitted rather than read as "0 waiting". A
cursor with `kinds=unread` is 400 `invalid_cursor`, because a cursor positions
waiting entries only. An `inbox_too_large` 503 from the waiting half names
`kinds=unread` in its detail, so a busy agent can still fetch its unread
channels. A degraded configuration answers as in slice 1, without unread. Tests:
`TestInbox_UnreadChannels`, `TestInbox_Kinds` (rows for each point above),
`TestInboxHandler_Kinds` (400 on `kinds=mention`, 400 on a cursor with
`kinds=unread`, `unread_count: 0` present).
12. MCP `mark_read` takes the channel address and an optional `seq`, and
answers `{seq, unread, last_seq}`. It works over stdio and remote MCP.
`read_channel` never moves a marker, also when a watch replays it. The
unavailable channel deps refuse `mark_read`. Test: `TestMarkRead_Tool`,
`TestReadChannel_DoesNotMoveMarker` (read, watch replay, then `Reads` shows no
marker), `TestUnavailableChannels_MarkRead`.
13. A `watch` on `list_inbox` wakes for chat only by exact channel key:
the inbox answer carries `WakeKeys` (the window's channel keys, at most 500)
beside the waiting types; a channel event wakes the watch when its key is in
`WakeKeys`, or when its type is a waiting type and it is a session change, as in
slice 1. It does not wake for a post in a channel outside the window, even of a
type the window holds, nor for any chat when the watch's args carry `kinds:
["waiting"]`. A channel the caller first marks after the last replay is learned
on the next replay or the 15 s poll. Tests: `TestWatch_ListInboxWakesOnUnread`,
`TestWatch_ListInboxNoWakeOnChat` (marker on channel X of type T, post in
channel Y of type T: no wake), `TestWatch_ListInboxWaitingOnlyIgnoresChat`.
14. A mark does not broadcast. A moved marker wakes only the live
subscriptions bound to the same reader (`Local.markFor`); other principals'
subscriptions take nothing and replay nothing. A mark that does not move the
marker wakes nobody. Test: `TestLive_MarkReadFramesHead` in `internal/dataentry`
(A and B subscribe to the head; A marks read: A's second tab gets one frame with
the new `read`, B replays zero times; A marks again with the same seq: no
frame).
15. The SPA channel panel shows a badge with the unread count on each
channel row that has `read.unread > 0`, "99+" above 99, with the accessible name
"N unread". The open channel calls `PUT …/_read` with its newest seq when the
newest page is held, the tab is visible and the newest seq is above the held
marker, at most once per second. The PUT answer becomes the held marker.
`setChannel` keeps `read` from the held head when a message page's channel
(which has none) replaces it. A head frame whose rev and last seq equal the held
ones (a mark by the same reader in another tab) only updates `read` and fetches
nothing. A 429 keeps the newest seq pending and sends it once after
`Retry-After` (trailing edge); 403 and 404 are dropped. None of this is shown to
the user. Tests: `channelView.test.ts` (row view carries `unread`),
`RlEntityChannels.stories.ts` play test (badge text and accessible name, "99+"
at 100), `useMarkRead.test.ts` (marks once after load; not while hidden; not
when newer messages were dropped; debounced; a 429 then one retry with the
newest seq), `useChannel.test.ts` (`read` survives a follow; a mark-only head
frame causes no request; one incoming message causes at most one PUT),
`wireFields.test.ts` (TS mirror of the new wire fields).
16. CLI: `rela channel mark-read RISK-1 breach-response 0k3x… [--seq N]`
prints `marked read up to 42 (0 unread)`; `rela channel list` adds `(2 unread)`
to a row with unread; `rela channel inbox` prints an "Unread channels" section
and takes `--kinds`. Tests: `TestChannelMarkRead_Text`,
`TestChannelList_Unread`, `TestChannelInbox_Unread` in `internal/cli`.
17. A job identity (`--run-as integration:triage-bot`) that posts in a
channel gets a marker there; a later post by another principal shows in its
`list_inbox` `unread_channels`; `mark_read` clears it. Test:
`TestInbox_JobIdentityUnread` (stdio wiring through the CLI harness, fs
backend).
18. Docs describe markers, the counting rule, the cap, when a marker moves
(never on a read), the route, the tool, the inbox fields, per-transport reader
identity and retention. They say that an agent posting as the user moves the
user's marker and recommend a job identity (`--run-as`) for agents that must not
touch it, and they list read markers with the other per-user data rela keeps.
Test: review (docs checklist).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (options a1 and d2, gate e1; decisions 2026-10-27).

**Existing Solutions:**
- No library: this is a table and a count over rela's own channel stores.
- `channels.Store.ListWaiting` and `WaitingQuery`
(`internal/channels/waiting.go`, `sqlchannels/waiting.go`): the pattern for a
new store query with validation in the `channels` package, chunked `IN (VALUES
…)` SQL shared by both databases, and a file-backend tree walk. `Reads` uses the
same chunking with a 4-column row value.
- `sqlchannels.insertMessage` / `insertNotice` / `update`
(`internal/channels/sqlchannels/sql.go:480-665`): where `counts` and the marker
upsert are written. Delete and Purge already delete `channel_messages` rows
explicitly before the channel row (`sql.go:837-840`, `retention.go:340-343`);
`channel_reads` follows the same order, with `ON DELETE CASCADE` as a backstop
(`foreign_keys(1)` is on in sqlite, `sqlitedb.go:283`).
- Schema rungs: `addChannelCreator` and `channelCreatorDDL`
(`internal/sqlitedb/migrate.go:289-314`, `sqlitedb.go:760-769`); pg
`0034_channel_created_by_user.sql` (temporary plpgsql helper that catches a bad
cast per row).
- File layout: one directory per channel, so a file inside it moves with
rename, tombstone, untombstone and purge for free
(`internal/channels/filechannels/file.go:1-75`). `ListSessionsDue`'s walk is the
model for `ListReads`.
- Session notice identity: `channels.SessionMarker` is written only by the
store (`PrepareMessage` refuses it), so the store can classify a notice without
`realtimecfg` (`internal/channels/session.go:253-260`,
`internal/realtime/session.go:478-492`).
- Author identity: `authorOf` and `sameAuthor`
(`internal/realtime/authz.go:360-370`).
- Gate: `inboxRun.anchors`, `inboxRun.item`, `roleAdmits`, `Access.Prime`
(`internal/realtime/inbox.go`, slice 1). `call.listType` for the list
(`internal/realtime/service.go:722-760`).
- Rate limits: `limiter.take` with a per-channel op bucket under the
identity bucket (`internal/realtime/ratelimit.go:152`), as typing uses.
- Live: `Local.mark` wakes subscriptions; the live hub replays the head
and list as the subscriber (`internal/realtime/local.go:150-180`,
`internal/dataentry/live_topics.go`).
- MCP watch: `watchChannel.matches` and `learnTypes`
(`internal/mcp/tools_watch.go:160-200`), `noteInbox`.
- Budget harness: `countingChannels`, `TestChannelsBudget_List10vs50`,
`TestInboxBudget_10vs50` (`internal/dataentry`).
- SPA: `useChannel` already calls `onSeen` with every head and page it
takes in (`frontend/src/composables/useChannel.ts:52,191`); `RlNavStatus`
already renders a count beside a glyph
(`frontend/packages/rela-components/src/types/index.ts:136-170`); the untracked
"Mockups/Realtime" Storybook in the main checkout shows the per-channel count
(`RlRealtimeEntityChannels.vue:59`).
- Prior art: Slack (one private marker per channel, an own post marks the
channel read, counts only for channels you belong to), Matrix (`m.fully_read`:
private, never moves back).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Store types** (`internal/channels/reads.go`, new):
   ```go
   // Reader is whose read marker a row is: a principal's user and
   // organization, as Author records them. The tool is not part of it, so
   // a user's browser and agent share one marker.
   type Reader struct{ User, OrgID string }
   func ReaderOf(a Author) Reader
   func (r Reader) Validate() error // ErrInvalidMessage: the checkAuthor rules (empty user, length, UTF-8, NUL)

   // ReadMarker is a reader's marker on one live channel.
   type ReadMarker struct {
       Channel Channel   // the channel as stored, for the caller's gate
       Seq     int64     // highest seq read; 0 is opened, nothing read
       At      time.Time // when the marker last moved
       LastAt  time.Time // when the channel last gained a message; orders the window
   }
   // ReadState is a marker and what lies above it.
   type ReadState struct {
       Seq     int64
       At      time.Time
       LastSeq int64
       Unread  int // counting messages above Seq, at most UnreadCap
   }
   const (
       UnreadCap     = 100
       MaxReadKeys   = 2000 // keys per Reads call
       MaxReadWindow = 500  // most markers one ListReads returns
   )
   func IsSessionNotice(m Message) bool // strings.HasPrefix(m.TriggeredBy, SessionMarker)
   func Counts(m Message) bool          // m.Removed == nil && !IsSessionNotice(m)
   func CheckReads(r Reader, keys []Key) error
   ```
The reader's byte bounds reuse the author limits that `checkAuthor` applies, so
any author the store accepted is a valid reader. `Counts` is the one definition
of a counting message; the file backend calls it in Go and the SQL backends
store its result in `counts`.

2. **Store methods** (`internal/channels/store.go`):
   ```go
   SetRead(ctx context.Context, key Key, r Reader, seq int64, at time.Time) (ReadState, bool, error)
   Reads(ctx context.Context, r Reader, keys []Key) (map[Key]ReadState, error)
   ListReads(ctx context.Context, r Reader, limit int) ([]ReadMarker, error)
   ```
`NewMessage.MarkRead bool` and `NewChannel.MarkRead bool` as in AC 4. The
plimsoll directives on `sqlchannels.Store` and `filechannels.Store` go from 30
to 33, with the history line, as a required-interface exception.

3. **sqlchannels** (`internal/channels/sqlchannels/reads.go`, new; edits
in `sql.go`, `retention.go`):
   - `insertMessage`, `insertNotice`, `SetSession`'s notice and reminder
inserts write `counts = channels.Counts(m)`, and so does the shared `update`
statement for edit, remove and `SetAction` alike, so the one rule decides every
write. Every insert also sets `channels.last_at` to the message's time.
   - Marker upsert, shared by `SetRead` and `Append`:
     ```sql
     INSERT INTO channel_reads (channel_id, reader_user, reader_org, seq, at) VALUES (?, ?, ?, ?, ?)
     ON CONFLICT (channel_id, reader_user, reader_org)
     DO UPDATE SET seq = excluded.seq, at = excluded.at WHERE excluded.seq > channel_reads.seq
     ```
Both databases support this form; rows affected tells `moved`. `SetRead` runs in
`s.write(ctx, []string{k.ID}, …)` so it serializes with `Rekey` under the entity
lock; it reads the channel row (`tomb = ''`), clamps to `last_seq`, upserts,
then reads the marker back and counts in the same transaction. `Append` with
`MarkRead` upserts after `bumpSession`, at the last seq written. `CreateMany`
with `MarkRead` inserts at 0.
- Count, one scalar subquery per row, capped inside:
  ```sql
  (SELECT COUNT(*) FROM (SELECT 1 FROM channel_messages m
     WHERE m.channel_id = c.id AND m.seq > r.seq AND m.counts = 1 LIMIT 100) u)
  ```
Tried by hand on sqlite and postgres 18 during planning; the explain tests in AC
10 pin it on the modernc driver and on pg. The partial index
`channel_messages_counts_idx ON channel_messages(channel_id, seq) WHERE counts =
1` makes each count a range scan of at most 100 index entries; the query writes
the literal `counts = 1` so the planner can match the index predicate.
- `Reads`: `SELECT c.anchor_id, c.anchor_face, c.type, c.name,
c.last_seq, r.seq, r.at, <count> FROM channels c JOIN channel_reads r ON
r.channel_id = c.id WHERE c.tomb = '' AND r.reader_user = ? AND r.reader_org = ?
AND (c.anchor_id, c.anchor_face, c.type, c.name) IN (VALUES (?, ?, ?, ?), …)`,
in chunks of 500 keys, all in one call.
- `ListReads`: `SELECT chanCols, c.last_at, r.seq, r.at FROM
channel_reads r JOIN channels c ON c.id = r.channel_id WHERE r.reader_user = ?
AND r.reader_org = ? AND c.tomb = '' ORDER BY (c.last_seq > r.seq) DESC,
c.last_at DESC, c.anchor_id, c.anchor_face, c.type, c.name LIMIT ?`. The reader
filter uses `channel_reads_reader_idx (reader_user, reader_org)`; the sort runs
over that reader's markers only. Unread candidates fill the window first, so a
channel whose marker is old but which has new messages is never cut; caught-up
channels fill the rest and give the watch its wake keys. Damaged channel rows
are skipped with the `DamageWarner`.
- `Delete` and `Purge` run `DELETE FROM channel_reads WHERE channel_id
= ?` before they delete the messages. `Rekey`, `Tombstone` and `Untombstone`
change only `channels` columns, so markers travel.

4. **Schema.**
   - sqlite: `schemaVersion` 28. `channelsDDL` gains `counts INTEGER NOT
NULL DEFAULT 1` on `channel_messages` and a `channelReadsDDL` (`channel_reads`
with `PRIMARY KEY (channel_id, reader_user, reader_org)`, `REFERENCES
channels(id) ON DELETE CASCADE`, STRICT). Rung v27 to v28 (`addChannelReads`):
create the table, add the column through a probe like `addChannelColumns` but
for `channel_messages`, then `UPDATE channel_messages SET counts = 0 WHERE
counts = 1 AND json_valid(doc) AND instr(doc, '\u0000') = 0 AND (json_type(doc,
'$.removed') IS NOT NULL OR substr(json_extract(doc, '$.triggered_by'), 1, 8) =
'session:')`. `channelReadsIndexDDL` (both indexes) runs after the ladder, like
`channelCreatorDDL`, because it names the new column. The rung also adds
`channels.last_at INTEGER NOT NULL DEFAULT 0` through `addChannelColumns` and
sets it to `max(created_at, closed_at, archived_at, reopened_at)`; the real
value arrives with the next message. This orders existing channels only roughly
until then and needs no JSON parsing.
   - postgres: `0035_channel_reads.sql`: the table (`channel_id BIGINT`,
`reader_user TEXT COLLATE "C"`, `reader_org TEXT COLLATE "C"`), `ALTER TABLE
channel_messages ADD COLUMN IF NOT EXISTS counts INTEGER NOT NULL DEFAULT 1`, a
backfill through a temporary plpgsql function `rela_message_counts(doc text)`
that returns 1 on `\u0000` or a failed cast, 0 when `doc::jsonb ? 'removed'` or
`doc::jsonb ->> 'triggered_by'` starts with `session:`, else 1. The `UPDATE`
prefilters with plain text so the function, and its jsonb cast, runs only on
candidate rows: `WHERE counts = 1 AND (strpos(doc, '"removed":') > 0 OR
strpos(doc, '"triggered_by":"session:') > 0) AND rela_message_counts(doc) = 0`.
Both prefixes are what `docOf` writes (`encoding/json`, no spaces). `DROP
FUNCTION`; `ADD COLUMN IF NOT EXISTS last_at BIGINT NOT NULL DEFAULT 0` on
`channels` with the same `GREATEST(…)` backfill as sqlite; both indexes. Header
comment in the style of 0034. The `ALTER TABLE` holds an exclusive lock on
`channel_messages` until the migration commits; the prefilter keeps that short.
   - A row the backfill cannot parse keeps 1: it counts, which shows one
extra unread at worst and never hides a message.
   - The `datamigration` rule does not apply: these are schema rungs.
TKT-6RXAC0 (mentions) will also add a rung. The slices land in order and the
later one renumbers on rebase. Both ladders catch a clash: the pg prefix-unique
test rejects a second 0035, and `TestMigrationLadderIsWellFormed` requires
contiguous `to` versions, so two rungs to v28 fail it.
   - `internal/cli/db_importfs_sqlite_test.go` lists `channel_reads` with
the TKT-DD8B6I "not copied yet" reason, as for the other channel tables.
   - File tier: no schema change (see 5).

5. **filechannels** (`internal/channels/filechannels/reads.go`, new):
   - `reads.json` beside `meta.yaml` in each channel directory: `{"v":1,
"readers":[{"user":…,"org":…,"seq":…,"at":…}]}`, sorted by user and org, written
atomically (temporary file, rename) through the rooted FS. A damaged file reads
as no markers, with a warning, and the next `SetRead` replaces it.
   - `SetRead`: first, without the lock, read `reads.json` and the log
tail; if the seq (clamped) is not above the reader's marker, answer the current
state and take no lock. The marker only rises, so a stale read costs at most one
redundant lock. Otherwise take the semaphore and the file lock, as every
mutation; read the last seq from the log (not `meta.yaml`), clamp, read, merge,
write. No `meta.yaml` write, no rev. The lock is project-wide, so a moving mark
serializes with every write in the project; the fs scale note says so.
   - `Append` with `MarkRead`: after the log records, in the same lock
span. A failed `reads.json` write there is logged as a warning (key, user, no
content) and swallowed, and `Append` returns the message: the message is already
durable, and an error would make the client post it again. A crash between the
two leaves the poster's own message unread for them; never a marker above the
log.
   - `CreateMany` with `MarkRead`: writes `reads.json` after `meta.yaml`,
best effort in the same way.
   - `Reads`: per key, no lock (as `ListSessionsDue`): read `reads.json`;
read the last seq through the existing tail reader and skip the key when it is
at or below the marker. Otherwise scan backward from the end of the log with the
doubling tail `lastMessage` uses, folding patches into the messages seen, and
stop at a message whose seq is at or below the marker or at `UnreadCap` counting
messages. A patch to a message above the marker is written after that message,
so the backward scan has seen it by then. Count with `channels.Counts`.
   - `ListReads`: one tree walk as `ListSessionsDue`: per channel
directory, read `reads.json` (a missing file is one failed open); for a
directory that holds the reader, read `meta.yaml` and the log tail for `LastSeq`
and `LastAt`. Sort into window order and cut in Go.
   - Delete, purge, rekey, tombstone and untombstone move or erase the
directory, so markers follow with no new code. The `~detached` and `~trash`
trees are outside the walk.
   - The MemFS store is the same code.

6. **realtime service** (`internal/realtime/reads.go`, new; edits in
`service.go`, `inbox.go`, `render.go`, `ratelimit.go`):
   - `ChannelView.Read *ChannelRead` (`json:"read,omitempty"`),
`ChannelRead{Seq int64; Unread int}`.
   - `Service.Post` sets `NewMessage.MarkRead = true` for a log post.
`Service.Create` sets `NewChannel.MarkRead = true`. The trigger engine, timers,
reminders and outputs do not.
   - `ListAnchor`: after `listType` collects the readable channels,
`Reads(ReaderOf(authorOf(c.p)), keys)` for the stored ones, in chunks of
`MaxReadKeys` (one call for any realistic entity), skipped when the caller has
no identity or there are no stored channels; fill `Read`.
   - `Head`: after `open`, one `Reads` for `c.key` when `c.head` is set
and the caller has an identity. When the `Reads` answer's `LastSeq` is above the
head's, the view takes it, so `read.seq` never exceeds `last_seq`.
   - `Service.MarkRead(ctx, addr RawAddress, seq *int64) (MarkReadResult,
error)`: `open` (the read gate, as the head); `requireCaller`; a nil `c.head`
answers zero values and stores nothing; a negative seq is `invalid`; then
`s.readLimits.take` (a second `limiter` instance with `ReadIdentityRate` as its
aggregate and its own bucket caps, so marks never touch the post limiter; per
reader and channel at `ReadRate` = 2 per 2 s); then `SetRead(c.key, reader,
seqOr(head.LastSeq), c.now)`; on `moved`, `s.local.markFor(reader, c.key)`.
`ErrNotFound` from the store (the channel went away after `open`) is
`channelNotFound()`. `MarkReadResult{Seq, Unread, LastSeq}`. Mark read needs the
read gate only, not a post role or the client ceiling's update (decision 2).
   - `limiter.take` gains the aggregate rate as a field of the limiter
(today it names `IdentityRate` directly), so the read limiter can use
`ReadIdentityRate`.
   - Reader-scoped wake (`local.go`): `Subscription` gains an optional
`reader channels.Reader`, set by `SubscribeAs(reader)`. `markFor(reader,
keys...)` adds the keys only to subscriptions with that reader. `mark` is
unchanged. The live hub subscribes with the connection's reader
(`channelsHandler.changes` takes the request principal). The other channels of
other principals replay nothing on a mark. The SPA's own tab has the PUT answer
already; the reader's other tabs get one head frame with the new `read`.
   - **Inbox** (`inbox.go`):
     - `InboxRequest.Kinds []string`; `CheckInboxKinds`. Empty means both.
     - `InboxView` gains `UnreadChannels []InboxUnread` (`json:
"unread_channels,omitempty"`), `UnreadCount *int`
(`json:"unread_count,omitempty"`, set whenever unread is requested, so a real 0
is sent), and `WakeKeys []channels.Key` (`json:"-"`): the keys of every channel
in the caller's gated window. `WaitingTypes` stays as in slice 1.
     - With `kinds=unread`, `Entries`, `Count`, `Truncated` and
`NextCursor` are omitted (the wire type makes them omitempty pointers or
slices); a cursor is `invalid_cursor`.
     - The `inbox_too_large` error from the waiting half carries a detail
that names `kinds=unread`.
     - `InboxUnread{Channel InboxChannel; Seq, LastSeq int64; Unread int}`.
     - Steps when `unread` is requested and the cursor is empty:
       1. `ListReads(reader, MaxReadWindow)`.
       2. Drop channels whose type the configuration no longer declares.
       3. Merge their anchor addresses with the waiting candidates into
the one `Anchors` call slice 1 makes, then the one `Prime`. (`waitingItems` is
split so both candidate sets are collected before the batch.)
       4. Per marker channel: anchor present and of an attach type; reset
`c.ct`, `c.anchor`, `c.key`, `c.head = &ch`, `c.ops`; then `roleAdmits(c, &ch,
ct.Read, CapabilityRead)`.
       5. One `Reads(reader, keys)` for the gated channels whose `LastSeq`
is above the marker. None: no call.
       6. `WakeKeys` = the gated keys. Keep unread above 0, in window
order (last activity descending, then key); `UnreadCount` = len; keep the first
7. - A cursor page skips steps 1 to 6 and carries no unread fields.
   - Nothing is cached; every call answers for its own principal.

8. **REST** (`internal/dataentry`):
   - `channels_handler.go`: `readSegment = "_read"`; `parseChannelRoute`
accepts `{type}[/{name}]/_read`; `serve` routes it to `markRead` with
`allowMethod(w, r, http.MethodPut)`. A consumer-side `channelMarker` interface
(`MarkRead`), set by `SetChannels`. `markRead` decodes `{"seq"?: int}` strictly
(`decodeChannelBody`, empty allowed), answers `v1.ChannelMarkRead`.
   - `channels_wire.go`: `wireChannel` maps `Read`; `v1.Channel.Read
*ChannelRead` (`json:"read,omitempty"`); `v1.ChannelRead{Seq int64; Unread
int}`; `v1.ChannelMarkRead{Seq, Unread, LastSeq}`.
   - `inbox_handler.go`: parse `kinds` (comma list), map
`UnreadChannels` and `UnreadCount`, with links as for entries; `v1.InboxUnread`.
   - Golden wire file and the TS mirror list the new types and fields.
   - `internal/openapi/channels.go`: the PUT path (body, 200, 400, 403,
404, 405, 429), `read` on `Channel`, the inbox fields and `kinds`. Emitted only
when realtime is on.

9. **MCP** (`internal/mcp`):
   - `tools_channels.go`: `ChannelMarker` consumer interface
(`MarkRead`), field `ChannelDeps.Marker` (Nil: accepted; no tool),
`unavailableChannels` implements it. Tool `mark_read` (`id`, `channel_type`,
`name`, `seq`), not in `reads`, so not watchable. Description: it never moves
back, it clamps to the head, `read_channel` never moves it. The instruction text
gains "To clear unread, call mark_read with the last seq you handled."
   - `list_channels` answers `read` through `ListAnchor`.
   - `tools_inbox.go`: `kinds` argument (array of `waiting`, `unread`);
description of `unread_channels`.
   - `tools_watch.go`: `noteInbox` records `WaitingTypes` and `WakeKeys`.
`matches` wakes on a channel event whose key is in the wake keys, or whose type
is a waiting type (slice 1 rule). A channel the caller first marks after the
last replay has no unread right after the mark, so the answer does not change;
the next replay or the 15 s poll adds its key. `mark_read` does not wake the
caller's own watch: the tool answer already says what changed.
   - `mark_read` description: marks count toward their own rate limit,
not the post budget; an agent posting as the user also moves the user's marker,
so use a job identity (`--run-as`) to keep them apart.
   - Wiring that builds `ChannelDeps` (server and stdio) passes the
service as `Marker`.

10. **CLI** (`internal/cli`): `ChannelMarkReadCmd` in
`channel_mark_read.go` (args id, type, optional name; `--seq`; `--json`);
`channelWriter` gains `MarkRead`; `channel_render.go` adds `(N unread)` to list
rows; `channel_inbox.go` adds `--kinds` and an "Unread channels" section.

11. **SPA** (`frontend`):
    - `src/types/channels.ts`: `ChannelRead`, `Channel.read?`,
`ChannelMarkRead`, wire field lists.
    - `src/api/channels.ts`: `markChannelRead(t, seq?)` (PUT).
    - `src/components/entity/channels/OpenChannel.vue` (or a small
`useMarkRead` composable next to `useChannel`): watch the newest held seq, the
head's `read.seq`, `document.visibilityState` and `newerDropped`; when the
newest page is held, the tab is visible and newest > read seq, call
`markChannelRead` with the newest seq, at most once per second; the answer
becomes the held marker. On 429, keep the newest seq pending and send it once
after `Retry-After` (trailing edge). Drop 403 and 404. Re-check on
`visibilitychange`.
    - `src/composables/useChannel.ts`: `setChannel` keeps `read` from the
held head when the incoming channel has none (message pages), and `follow`
treats a head frame whose rev and last seq equal the held ones as a `read`-only
update: no `messages?after_rev`, no `refetchHead`, no `readNewest`.
    - `src/components/entity/channels/channelView.ts`: `channelGroups`
copies `c.read?.unread` to the row.
    - `rela-components`: `ChannelRowView.unread?: number`;
`RlEntityChannels.vue` renders a count badge (the `RlNavStatus` count style),
"99+" above 99, accessible name from `messages.channelText({ key: 'unreadCount',
count })`; the doc comment no longer says "no unread counts"; a story with
unread rows.
    - The list subscription is dropped while a channel is open (existing
behaviour), so counts refresh when the list is live again.

12. **Docs.** `docs/realtime.md`: new "### Read markers and unread counts"
(reader identity per transport, including that a remote agent posting as the
user moves the user's marker and that a `--run-as` job identity keeps them
apart; counting rule; cap; when a marker moves; retention; "what rela keeps
about a reader": user, org, seq and time per channel, kept until the channel
goes, with the forget-reader follow-up named), a row for `PUT …/_read` in the
HTTP API table, the `read` object, `kinds`, the unread fields and the `limit`
and cursor rules under Inbox, and the fs scale note (the project-wide lock on a
moving mark, the `ListReads` tree walk). `docs/mcp-server.md`: `mark_read` (own
rate limit), `list_inbox` kinds and unread, watch wake rule by key, agent loop
with the job-identity advice. `docs/cli-reference.md`: `rela channel mark-read`,
list and inbox changes. `docs/acl-security.md`: markers are private per reader,
counted after the gate, and the window's accepted one-bit channel (Security).
`docs/data-entry.md`: unread badges; opening a channel marks it read.

**Retention and cleanup:**

| Event | Markers |
| --- | --- |
| Anchor rename (`Rekey`) | Carried: `channel_id` stays; the fs directory moves |
| Tombstone (entity delete, missing-entity sweep, id reuse) | Carried and unreachable: every query filters `tomb = ''`; the walk skips `~detached` |
| Untombstone | Back with the channel |
| Purge, channel `delete`, retention erase | Erased with the channel |
| Cardinality-one channel deleted, then recreated by a post | New channel, no markers |
| Principal removed (user entity deleted) | Kept: the reader is a string, not a reference. They are never shown to anyone else, and go with the channel. A new user that reuses the id inherits read positions only, never content |
| User entity renamed | Not followed, as message authors are not: the renamed user starts without markers |
| Message edit | Seq unchanged; not unread again |
| Message removal | Stops counting at once (`counts = 0` / `Counts`) |

**Unread rule, decided per message:**

| Message | Counts |
| --- | --- |
| Post by another principal | Yes |
| Own post | Never above the marker: the post moved the marker past it |
| Session notice (message move or timer move) | No |
| Reminder from `system:realtime` | Yes |
| Trigger post | Yes |
| Removed message | No |
| Ephemeral signal | Never stored, never counted |

**How identities get markers:**

| Caller | Reader key | Notes |
| --- | --- | --- |
| Web app, remote MCP | User entity id (or raw login without `user_entity_type`), org claim | The browser and a remote agent of one user share markers, so an agent's post as the user marks the user's channel read, including colleagues' messages the user never saw. Agents that must not touch the user's read state run as a job identity (`--run-as`) |
| Stdio `rela mcp`, `rela channel` | Raw login, org `""` | A different reader than the same person in the web app, as for authorship |
| Job identity (`--run-as integration:x`) | `integration:x` | Gets markers by posting or by `mark_read` |
| `system:realtime` | Never | Its writes do not set `MarkRead` |
| Read-only client | As its user | May mark read (decision 2) |

**Alternatives rejected:**
- A separate inbox service like piles and leases (research a2): rename,
tombstone and purge would need hooks; rejected by decision.
- `LastSeq - marker` (research d1): counts notices and removed messages.
- Author columns on `channel_messages` to exclude own posts in SQL: the own
post already moves the marker in the same step, so no own message lies above it;
one `counts` column suffices.
- Counting in Go from the docs on SQL backends: a per-channel row limit
cannot skip quiet rows without reading them; the column and the partial index
keep it in SQL.
- Moving the marker on `read_channel` or on any GET: the live hub and
MCP watch replay reads as the subscriber, so a read that writes would mark
messages read that nobody saw.
- `read` on the message page and in `read_channel`: every `mark_read`
would wake the agent's own `read_channel` watch with no new message.
- Raising the channel rev on a marker move: other readers' follows would
refetch for a change they cannot see.
- A `truncated` flag on the window: the window holds markers on channels
the caller can no longer read, so a flag would count them.
- Unread on every inbox page: later pages would repeat the same list.
- A cap on readers per channel: needs eviction on every backend; markers
are bounded by authenticated principals.
- Ordering the window by marker time (the first plan): the oldest markers
are the ones most likely to sit under new messages, so they would be cut first
(design review).
- Broadcasting a moved marker to every subscriber: N viewers times N
marks per message, all but one replay wasted (design review). The wake is scoped
to the reader.
- Matching inbox wakes by channel type: most configurations have one to
three types, so any chat would wake every inbox watch (design review). Wakes
match by key.

**Dependencies:** `internal/channels` and its backends, `internal/sqlitedb`,
`internal/store/pgstore/migrations`, `internal/realtime`,
`internal/realtimelimits`, `internal/dataentry`, `internal/apiwire/v1`,
`internal/openapi`, `internal/mcp`, `internal/cli`, `frontend` (SPA and
`rela-components`). No new imports across arch-lint boundaries.

**Files to modify:**
- `internal/channels/reads.go` (new), `store.go`, `channels.go`
(`NewMessage.MarkRead`, `NewChannel.MarkRead`)
- `internal/channels/channelstest/channelstest.go` (register),
`reads.go` (new)
- `internal/channels/sqlchannels/sql.go`, `retention.go`, `reads.go`
(new), directive
- `internal/channels/filechannels/ops.go`, `file.go`, `tail.go`,
`reads.go` (new), directive, tests (`TestReads_TailCost`,
`TestAppend_MarkReadWriteFailure`, `TestSetRead_NoOpTakesNoLock`)
- `internal/channels/sqlitechannels/reads_explain_test.go` (new)
- `internal/sqlitedb/sqlitedb.go`, `migrate.go` (+ migrate test)
- `internal/store/pgstore/migrations/0035_channel_reads.sql` (new) (+
test, `channel_reads_explain_test.go`)
- A test beside the migrations that the `session:` literal in both equals
`channels.SessionMarker`
- `internal/cli/db_importfs_sqlite_test.go` (`channel_reads` entry)
- `internal/realtime/reads.go` (new), `service.go`, `inbox.go`,
`render.go`, `ratelimit.go` (aggregate rate as a limiter field; the read
limiter), `local.go` (`SubscribeAs`, `markFor`), `errors.go` if a new type is
needed, tests (`reads_test.go`, `inbox_test.go`, `local_test.go`,
`helpers_test.go` fakes)
- `internal/realtimelimits/limits.go` (`ReadRate`, `ReadIdentityRate`)
- `internal/dataentry/channels_handler.go`, `channels_wire.go`,
`inbox_handler.go`, `channels_budget_test.go`, `inbox_budget_test.go`,
`channels_wire_golden_test.go` and testdata, new handler tests
- `internal/apiwire/v1/channels.go`
- `internal/openapi/channels.go` (+ test)
- `internal/mcp/tools_channels.go`, `tools_inbox.go`, `tools_watch.go`,
tests, and the server and stdio wiring of `ChannelDeps`
- `internal/cli/channel.go`, `channel_mark_read.go` (new),
`channel_render.go`, `channel_inbox.go`, tests
- `frontend/src/types/channels.ts`, `src/api/channels.ts`,
`src/composables/useChannel.ts`, `src/composables/useMarkRead.ts` (new),
`src/components/entity/channels/OpenChannel.vue`, `channelView.ts`, tests;
`frontend/packages/rela-components/src/components/channel/types.ts`,
`RlEntityChannels.vue`, `RlEntityChannels.stories.ts`, messages
- `docs/realtime.md`, `docs/mcp-server.md`, `docs/cli-reference.md`,
`docs/acl-security.md`, `docs/data-entry.md`
- Test fakes of `channels.Store` across packages (compile-driven:
`internal/realtime/helpers_test.go`, `internal/dataentry` counting store,
`internal/mcp/channels_fixture_test.go`, appbuild tests)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `seq` (PUT body, MCP arg, CLI flag): an integer of 0 or more; anything
else 400 / tool error before any write. Values above the head are clamped, never
stored.
- PUT body: strict JSON, at most `channelBodyLimit`, unknown fields 400.
- Channel address: parsed and gated by `open`, as every channel route.
The `_read` segment cannot collide with a channel name (names are minted
base36).
- `kinds`: allowlist `waiting`, `unread`; anything else 400.
- Reader: built from the bound principal through `authorOf`, never from
the request. Empty or `unknown` users are refused by `requireCaller`. The store
validates the reader again (`Reader.Validate`).
- Stored data: titles and anchor titles in `unread_channels` are entity
and channel data; the inbox answer keeps the `UntrustedNotice`. Entries carry no
message content.

**Security-Sensitive Operations:**
- Privacy of markers: a reader's markers are read only for that reader
(the store queries by reader key; the reader key comes from the bound
principal). No route, tool or count exposes another reader's marker or time.
- Gate before count: the list and head fill `read` only for channels the
existing gates admitted; the inbox gates every window channel with the anchor
batch (row gate, redaction, face, world), the primed role walk and `roleAdmits`,
and only then calls `Reads`, so no hidden channel is counted. `unread_count` is
computed on the gated set (CLAUDE.md aggregate rule). AC 9 pins it.
- The window is cut before the gate, an accepted one-bit channel: hidden
live channels on which the caller once held a marker occupy window slots
(ordered by activity, which the caller cannot see either), so with more than 500
markers a caller could infer that such a channel still exists or is active by
watching which visible channels drop out. It needs the caller to have read the
channel before, reveals no content and no property, and is recorded in
`docs/acl-security.md` next to slice 1's `inbox_too_large` note. Tombstoned
channels never occupy a slot.
- Mark read on a hidden channel answers what the head GET answers and
writes nothing. The marker write happens only after `open`.
- Write path: markers are channel-store state, outside the graph: no
entity audit record, no version, no search (comments rule).
- Personal data: a marker records when a named user read a channel. It
is never shown to another principal, but the operator holds it, and it outlives
the user entity until the channel goes. The docs list it with the other per-user
data; erasure on request is the forget-reader follow-up
(`.ignored/followup-2b2aia.md`).
- Rate limit: marks have their own limiter (`ReadRate` per reader and
channel, `ReadIdentityRate` per reader), so a client cannot flood store writes,
and marks cannot starve the same user's posts.
- Live: a moved marker wakes only the same reader's subscriptions
(`markFor`); other principals replay nothing, so a mark leaks no timing to them.
- Errors: store faults are logged with user, tool and channel key, never
content; the client gets the generic channel failure text.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Package |
| --- | --- | --- |
| 1 | `testSetRead`, `TestSetRead_NoOpTakesNoLock` | channels/channelstest (all four backend runs), filechannels |
| 2 | `testReadsCount`, `TestReads_TailCost` | channels/channelstest, filechannels |
| 3 | `testListReads` | channels/channelstest |
| 4 | `testAppendMarksRead`, `TestAppend_MarkReadWriteFailure` | channels/channelstest, filechannels |
| 5 | rows in 1 and 3, `Raw` check | channelstest, sqlchannels |
| 6 | `TestMigrate_v27ToV28ChannelReads`, `TestMigration0035_ChannelReads`, `TestChannelReadsMigrations_SessionMarker`, ladder test, `TestDBImportFS_SQLiteTablesAreAccounted` | sqlitedb, pgstore, cli |
| 7 | `TestChannelRead_ListAndHead`, `TestListAnchor_Read`, `TestHead_Read`, `TestHead_ReadNotAboveLastSeq` | dataentry, realtime |
| 8 | `TestChannelMarkRead_Handler`, `TestMarkRead_Service`, `TestMarkRead_OwnLimiter` | dataentry, realtime |
| 9 | `TestInbox_UnreadHiddenChangesNothing`, `TestInboxUnread_ACL`, `TestListAnchor_ReadHiddenType` | realtime, dataentry |
| 10 | `TestChannelsBudget_List10vs50`, `TestChannelsBudget_HeadRead`, `TestInboxBudget_10vs50`, `TestChannelsBudget_MarkRead`, `TestListAnchor_ReadsChunks`, `reads_explain_test.go`, `channel_reads_explain_test.go` | dataentry, realtime, sqlitechannels, pgstore |
| 11 | `TestInbox_UnreadChannels`, `TestInbox_Kinds`, `TestInboxHandler_Kinds` | realtime, dataentry |
| 12 | `TestMarkRead_Tool`, `TestReadChannel_DoesNotMoveMarker`, `TestUnavailableChannels_MarkRead` | mcp |
| 13 | `TestWatch_ListInboxWakesOnUnread`, `TestWatch_ListInboxNoWakeOnChat`, `TestWatch_ListInboxWaitingOnlyIgnoresChat` | mcp |
| 14 | `TestLive_MarkReadFramesHead`, `TestLocal_MarkForScopesReader` | dataentry, realtime |
| 15 | `channelView.test.ts`, `RlEntityChannels.stories.ts`, `useMarkRead.test.ts`, `useChannel.test.ts`, `wireFields.test.ts` | frontend |
| 16 | `TestChannelMarkRead_Text`, `TestChannelList_Unread`, `TestChannelInbox_Unread` | cli |
| 17 | `TestInbox_JobIdentityUnread` | cli (stdio wiring) |

Integration: the dataentry tests run the real router, real `acl.yaml`, fsstore
and filechannels; the budget tests also run on sqlite under the `sqlite` tag
where the channel budget tests already do. The pg conformance run covers the
three store methods, the `Append` marker and migration 0035 through the
schema-pinned DSN. One e2e check is optional: open a channel in the browser as
user A, post as user B through the API, see the badge on A's list, open it, see
the badge go.

**Edge Cases:**
- Marker at 0 on an empty named channel; first message makes unread 1.
- `seq` equal to the stored marker: no move, no frame, `at` unchanged.
- `seq` above the head: clamped; a later message still counts.
- 150 counting messages above the marker: `unread` 100; the SPA shows
"99+".
- Many removed messages above the marker: they do not count, and the
partial index skips them.
- Own post while the marker is ahead of it (a racing mark): marker stays
at the higher value.
- Own post that moves the session: marker at the notice's seq.
- Two posts racing from one user in two tabs: marker ends at the higher
seq (monotonic upsert).
- The same user in two orgs: two markers.
- A cardinality-one channel never stored: no `read`, PUT answers zeros.
- Closed and archived channels: marking allowed; counts shown.
- Anchor renamed between two reads: marker found under the new id.
- Entity deleted and the id reused: old markers are tombstoned with the
old channels; the new entity's channels start without markers.
- Channel type removed from `realtime.yaml`: its channels drop out of the
inbox (type check) and the list (no type).
- Face-anchored type: one marker per face channel; family type on a faced
entity: one marker, whichever face the reader views it through.
- A caller who loses read access keeps the marker; it is shown again when
access returns.
- Degraded configuration: list and head still carry `read` for channels
the read roles admit; the inbox answers degraded without unread.
- 1 200 keys in one `Reads` (crosses the chunk size); 500 markers in the
window; 501st not returned, unless it is an unread candidate, which sorts ahead
of every caught-up channel.
- 2 001 stored channels on one entity: the list makes two `Reads` calls.
- A channel with an old marker and new messages among 600 markers: in
the window and in `unread_channels`.
- An own post lands between the head's `Head` and `Reads`: `last_seq`
follows the higher `Reads` value.
- The SPA gets 429 on a mark: the newest seq is sent once after
`Retry-After`.
- A browser marks 60 times a minute while the same user's agent posts:
the agent's post budget is untouched.
- A channel upgraded from v27 without messages since: `last_at` is its
latest lifecycle time until the next message.
- A damaged `reads.json`: read as empty with a warning; the next mark
rewrites it.
- A pg or sqlite doc with `\u0000` or invalid JSON during the backfill:
keeps `counts = 1`.
- Concurrent `SetRead` and `Rekey` on fs and SQL: the marker ends under
the new id.
- The SPA tab hidden when a message arrives: no PUT until visible.

**Negative Tests:**
- PUT with `{"seq": -1}`, `{"seq": "x"}`, `{"seq": 1.5}`, `{"foo": 1}`,
trailing data: 400.
- PUT on a hidden anchor: 404 (same body as the head GET); on a channel
whose read role fails: 403 `read_denied`; nothing stored in either case (`Reads`
as the owner shows no marker).
- GET, POST, PATCH, DELETE on `…/_read`: 405 with `Allow: PUT`.
- Realtime off: 404 uniform.
- No identity: 403 `no_principal`.
- Three marks within a second on one channel: the third is 429.
- `kinds=mention` or `kinds=`: 400. A cursor with `kinds=unread`: 400
`invalid_cursor`.
- A failed `reads.json` write during an fs `Append`: the post succeeds.
- `SetRead` with an empty reader, a NUL in the user, or seq -1:
`ErrInvalidMessage`; with an invalid key: `ErrInvalidKey`.
- `Reads` with 2 001 keys: `ErrInvalidKey`. `ListReads` with limit 0 or
501: `ErrInvalidMessage`.
- Store fault in `Reads` during a list: 500 generic, no partial list.
- `mark_read` through `UnavailableChannels`: the refusal text.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **fs cost.** `ListReads` walks every channel directory on each inbox
call. `Reads` skips caught-up keys after a tail read and scans backward only as
far as the unread messages (at most 100), pinned by `TestReads_TailCost`. A
moving `SetRead` takes the project-wide lock and rewrites `reads.json`; a no-op
mark takes no lock. Mitigation: a watching agent replays at most every 15 s
(slice 1 pacing) or after the 2 s wake gap. Document the scale limit beside the
`ListSessionsDue` note; the database backends use indexes.
- **pg migration lock.** `ALTER TABLE` holds an exclusive lock on
`channel_messages` until 0035 commits. The text prefilter limits the jsonb casts
to rows that look removed or like notices, and the update writes only rows that
become 0. Channel tables are young and small. Rows it cannot parse keep 1 and
fail open (an extra unread).
- **Extra query per head and list.** Every live replay of a head or list
now costs one more indexed query. Mitigation: budget tests pin it; it is skipped
when the caller has no identity or no stored channel; marks no longer broadcast,
so they add no replays for other principals.
- **Watch wake rate.** `list_inbox` wakes for chat only in channels whose
key is in the caller's window. The 2 s wake gap and the 15 s poll pacing from
slice 1 still apply; `kinds: ["waiting"]` restores the slice 1 wake set for an
agent that only acts on sessions.
- **Agent clears the user's unread.** With decision 1, an agent posting
as the user moves the user's marker. Mitigation: docs and the `mark_read`
description recommend a job identity for such agents.
- **Rung clash with mentions.** TKT-6RXAC0 also adds a rung; land in
order and renumber on rebase. Both ladder tests catch a clash.
- **Behaviour change for the demo agent.** Its posts create markers, so
its default `list_inbox` now lists unread channels. Mitigation: docs and the
agent loop example pass `kinds` when only sessions matter; tell the agent-runner
session when this merges.
- **Interface growth.** `channels.Store` gains three methods; every fake
must follow. Mitigation: compile-driven; fakes embed the interface where they
can.
- **Identity per transport.** The same person over stdio and in the web
app has two readers. This is today's authorship rule; docs table.
- **Query plans.** Tried by hand on sqlite and postgres 18 during
planning. The explain tests (AC 10) pin index use on the modernc driver the
binary ships and on pg; the conformance run pins the results.

**Effort:** L, as the research estimated: three store methods on three backends
with two schema migrations, the inbox merge, REST, MCP, CLI and an SPA change.

**Design decisions (accepted by the user as recommended, with the design
review's changes folded in):**
1. Reader key: (user, org) as `channels.Author` records it, tool ignored,
so a user's browser and remote agent share one marker and "own post" means what
`sameAuthor` means. Alternative: `QueryIdentity.ID()`; in practice the same
string, but it has no org. Consequence: an agent posting as the user clears the
user's unread; use a job identity.
2. Read-only clients may mark read: the read gate is enough, because the
marker is the caller's own state and changes nothing another principal sees.
Alternative: require the client ceiling's update, which leaves a read-only agent
unable to clear its own unread.
3. Counting rule: session notices and removed messages do not count;
reminders and trigger posts do; own posts are covered by the marker move.
4. Creating a named channel opens it (marker at 0 for the creator).
5. `read` on the list and the head only, not on the message page or
`read_channel`.
6. Inbox unread: a `kinds` filter with both kinds by default; unread on the
first page only; a window of 500 markers, unread candidates first, then by
channel activity; at most 50 `unread_channels`, with `unread_count` for the
rest; ordered by channel activity, newest first.
7. Mark read rate: 2 per 2 seconds per reader and channel (a burst of 2),
on a limiter of its own with a 300-per-minute aggregate per reader, apart from
posts.
8. The CLI changes are in this slice.
9. A moved marker wakes only the same reader's live subscriptions; inbox
watches wake by channel key, not by type.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] docs/cli-reference.md - `rela channel mark-read`, unread in `list` and `inbox`, `--kinds`
- [x] docs/data-entry.md - unread badges in the channel panel; opening a channel marks it read
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the gantt aggregate rule and the slice 1 inbox pattern as written)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] docs/realtime.md - read markers section, HTTP API row, `read` object, inbox `kinds`, unread fields, `limit` and cursor rules, retention, identity table (agent posting as the user), what rela keeps about a reader, fs scale note (lock, walk)
- [x] docs/mcp-server.md - `mark_read` (own rate limit), `list_inbox` kinds and `unread_channels`, wake rule by key, agent loop with the job-identity advice
- [x] docs/acl-security.md - markers are private per reader and counted after the gate; marks wake only the same reader; the window's accepted one-bit channel

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 findings (7 significant, 8 minor, 4 nit), all
adopted. The resolutions are in `.ignored/dr-2b2aia-res.txt`.

| # | Severity | Finding | Where in this plan |
| --- | --- | --- | --- |
| 1 | significant | Window ordered by marker time hides busy channels | Terms (Window, Last activity), AC 3, Approach 3 to 5, decision 6 |
| 2 | significant | A mark broadcasts to every subscriber | AC 14, Approach 6 (reader-scoped wake), Security, decision 9 |
| 3 | significant | Inbox watch wakes by type | AC 13, Approach 6 (`WakeKeys`), Approach 8 |
| 4 | significant | SPA drops `read` on follow; three GETs per mark | AC 15, Approach 10 (`useChannel`) |
| 5 | significant | SPA ignores 429 | AC 8 (burst 2), AC 15, Approach 10 |
| 6 | significant | fs Append can fail a stored post | AC 4, Approach 5 |
| 7 | significant | fs Reads cost grows with segment size | AC 2, Approach 5 |
| 8 | minor | fs SetRead takes the project-wide lock | AC 1, Approach 5, docs scale note |
| 9 | minor | Marks share the post budget | AC 8, Approach 6 (read limiter), decision 7 |
| 10 | minor | ListAnchor can exceed `MaxReadKeys` | AC 10, Approach 6 |
| 11 | minor | Inbox kinds, cursor, limit and 503 | AC 11, Approach 6 (Inbox) |
| 12 | minor | Agent's post clears the user's unread | Identity table, AC 18, docs, decision 1 |
| 13 | minor | pg backfill casts every row under a lock | AC 6, Approach 4 |
| 14 | minor | SQL plans not pinned | AC 10 (explain tests), Approach 3 |
| 15 | minor | Markers are personal data, no erasure | Scope Out, Security, docs, `.ignored/followup-2b2aia.md` |
| 16 | nit | `update` should always write `counts` | Approach 3 |
| 17 | nit | Head `read.seq` above `last_seq` | AC 7, Approach 6 |
| 18 | nit | db import-fs table test | AC 6, Approach 4, Files |
| 19 | nit | Rung numbers vs mentions | Approach 4, Risks |
