---
id: PLAN-RTS2UQ
type: planning-checklist
title: 'Planning: Server-known quiet message types (unread, inbox)'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-03DMXH added `display: collapsed | hidden` per message type in
the `channels:` block of `data-entry.yaml`. Only the SPA reads it. The server
does not read `data-entry.yaml`, so it still counts every message of a quiet
type as unread. A `run` channel of the agent-runner PoC shows "12 unread" while
the reader sees one fold line. The inbox lists the channel under "Unread
channels" for the same reason. An inbox mention or reply row links to
`?channel=<key>` without a seq, so a mention inside a hidden or collapsed
message opens a channel where that message is not shown. `GUIDE-data-entry.md`
documents both as a known limit (line 1471) and names this ticket.

**How it works today (from the code).**

- *Counting.* `channels.Counts(m)` (`internal/channels/reads.go:84`) is the
one definition of "counts toward unread": not removed and not a session notice.
The SQL backends store its result at write time in `channel_messages.counts`
(`countsOf`, `sqlchannels/sql.go:595`). An unread count is a range of at most
`UnreadCap` (100) entries over the partial index `channel_messages_counts_idx`
(`WHERE counts = 1`, `unreadSQL`, `sqlchannels/reads.go:20`). The file backend
computes it per record from `recordHead.counts()` (`filechannels/tail.go:81`)
during a tail scan that stops at the marker or at the cap.
- *What reads the count.* `Reads` and `SetRead` answer `ReadState.Unread`.
That value is `read.unread` on the channel list and head (REST, MCP
`list_channels`, CLI), the answer of `PUT .../_read` and MCP `mark_read`, and
`unread_channels` / `unread_count` in the inbox. `inboxRun.unread`
(`internal/realtime/inbox.go:699`) skips a channel whose `Unread` is 0. The SPA
badge per channel (`channelView.ts:774`), the inbox page and the sidebar badge
all show server values. The SPA counts nothing itself.
- *Inbox rows.* Mention and reply rows are written by the store at append
time (`addressed.go`), one per target, from `NewMessage.Mentions` and the reply
parent. `notify: false` on a `ref` property already keeps a value from
mentioning anyone (`realtimecfg` `Property.NoNotify`,
`realtime/mentions.go:129`).
- *Who appends.* Three paths write a typed message. Two build a
`channels.NewMessage` with a compiled `*realtimecfg.MessageType` in hand: a
person's post (`realtime/service.go:450`) and a trigger post
(`realtime/system.go:62`, called only from `trigger.go:1282`). The third is a
session reminder. It never reaches `Append`: `reminderStep`
(`realtime/reminders.go:246`) returns a `channels.ReminderPost`, and the store
builds the message from it with `channels.ReminderMessage`
(`channels/session.go:238`), called from `filechannels/ops.go:1385` and
`sqlchannels/sql.go:1247`.
- *Live inbox.* An inbox subscription is replayed when a durable change
touches a key in its wake set, and a frame goes out only when the answer's hash
changes (`dataentry/live_inbox.go`). MCP `watch` on `list_inbox` works the same
way (`mcp/tools_watch.go`).
- *SPA reveal.* `RlChannel` exposes `reveal(seq)`: it shows a held hidden
or collapsed message in full, scrolls to it and focuses it, and returns false
when the message is not held. `useChannelPaging.onJump(seq)` loads up to ten
older pages and then calls `onHeld`, which `OpenChannel` wires to `reveal`
(`OpenChannel.vue:481`). `RlChannel.jumpTo(seq)` (internal, `RlChannel.vue:449`)
calls `reveal` and, when that fails, sets a `jumping` flag and emits `jump`; the
flag keeps the older pages loaded for the jump from being announced to screen
readers. `ChannelsSection.vue` opens the channel named by `?channel=` in a watch
on that query key alone (`ChannelsSection.vue:145`); nothing reads a seq from
the URL.
- *Mark read.* `useMarkRead` marks up to the newest held seq, whatever
its display (`composables/useMarkRead.ts:56`). A hidden row is held, so opening
a channel moves the marker past a quiet tail today.
- *Cross-file checks.* `dataentryconfig.ValidateChannels` checks the
`channels:` block against a consumer-side `ChannelSchema`
(`dataentryconfig/channels.go:75`), supplied by
`appbuild.realtimeChannelSchema`. It returns errors and warnings. The warnings
are logged at load and reload and printed by `rela validate`. Configure's save
keeps only the errors (`cmd/rela-server/configure.go:135`). Configure cannot
write `realtime.yaml` at all (`TestSave_RefusesRealtime`).

**Terms used below.**

- *Quiet type*: a message type with `display: collapsed` or `display:
hidden` in `data-entry.yaml` (TKT-03DMXH).
- *Non-counting type*: a message type with the new `unread: false` in
`realtime.yaml`. Its messages never count toward unread.
- *Counting message*: as today, a stored message that is not removed and
not a session notice, and now also not posted under a non-counting type.
- *Message link*: an SPA URL `?channel=<key>&seq=<n>` that opens the
channel and reveals message `n`.

**Scope (in):**

- `realtime.yaml`: `channel_types.<ct>.message_types.<mt>.unread: false`,
checked at load, on reload and by `rela validate`. Refused-key hints for `quiet`
and `display` on a message type.
- Store: a stored flag on the message, set from the type at append time,
read by `channels.Counts` on every backend (filechannels on disk and on MemFS,
sqlitechannels, pgchannels through sqlchannels).
- Service: all three write paths set the flag from the compiled type: a
person's post, a trigger post and a session reminder.
- `data-entry.yaml` check: a warning when `display: hidden` names a type
that still counts as unread.
- SPA: inbox mention and reply rows link with `&seq=`; the entity page
opens the channel and reveals that message. `RlChannel` exposes its existing
`jumpTo` (one line in `rela-components`).
- Docs: `GUIDE-realtime.md` and `GUIDE-data-entry.md` (and the generated
`docs/realtime.md`, `docs/data-entry.md` through `just docs`).

**Scope (out):**

- Reading `data-entry.yaml` on the server, or deriving one key from the
other (Q2).
- Recounting messages already stored when the key changes (Q3).
- Dropping mention or reply rows for non-counting types. `notify: false`
already covers mentions per property (Q4).
- Serving `unread` in `Describe`, MCP channel type descriptions, OpenAPI or
`_config`. No client needs it; `notify:` is not served either.
- Loading a page around an old seq (`after=`) for a message link. A link
to a message more than ten pages back opens the channel at the newest page, as a
reply quote jump does today.
- A seq on "Unread channels" and "Waiting" rows.
- Surfacing `ValidateChannels` warnings in Configure (it drops all of them
today).

**Acceptance Criteria:**

1. **The key loads and is checked.** `unread: false` and `unread: true` on
a `persist: log` message type load; absent means `true`. The compiled
`MessageType.NoUnread` is set only by `unread: false`. A non-boolean value is
refused by the key checker with the path. `unread` (either value) on a `persist:
ephemeral` type is refused with `channel_types.<ct>.message_types.<mt>.unread`
and the reason "a signal is never stored, so it never counts as unread".
`quiet:` on a message type is refused with the hint "use unread: false to keep
the type out of unread counts; to fold or hide it in the web app, use display in
data-entry.yaml". `display:` on a message type is refused with the hint "display
belongs in data-entry.yaml: channels.<type>.messages.<mt>.display". A reload
that flips the key is accepted. The docs example loads. An unquoted `no`, `off`
or `n` is a YAML boolean and loads as `false`; that is the decoder's rule for
every boolean key and stays as it is. Test:
`internal/realtimecfg/unread_test.go` (new) `TestParse_Unread` (table: absent,
true, false, quoted `"no"` refused, unquoted `no` loads as false, on an
ephemeral type, `quiet: true`, `display: hidden`); `TestCheckReload_UnreadFlip`;
`TestDocsExample_Unread` in `docs_example_test.go`.
2. **A message of a non-counting type is stored as non-counting.** A
person's post and a trigger post of a type with `NoUnread` append with
`NewMessage.NoUnread` set. A session reminder whose `post` type has `NoUnread`
returns a `ReminderPost` with `NoUnread` set, and `ReminderMessage` copies it
onto the message. The store keeps it on `Message.NoUnread`. `channels.Counts`
returns false for it. An edit keeps the flag. A removal clears the message as
today and it stays non-counting. The flag is never on the REST, MCP or CLI
message. Test: `internal/realtime/unread_test.go` (new) `TestPost_NoUnreadType`
(fake store records the `NewMessage`), `TestTriggerPost_NoUnreadType` (a trigger
post of a non-counting type), `TestReminderStep_NoUnreadType` (the step's
`Remind` returns a post with `NoUnread` for a non-counting post type, and
without it for a counting one), `TestRender_NoUnreadNotOnWire` (rendered message
JSON has no `no_unread`); `internal/channels/reads_test.go` `TestCounts` new
rows `no unread`, `no unread removed`; `internal/channels/reminders_test.go`
`TestReminderMessage_CopiesNoUnread`.
3. **Every backend leaves it out of unread.** `Reads`, `SetRead` and
`ListReads` agree on all backends: a non-counting message above the marker does
not add to `Unread`; an edit of one does not make it count; an action change on
one (pending, done, clear) does not make it count; 150 non-counting messages
plus 3 counting ones above the marker give 3. A reminder posted through the
session step with `NoUnread` above the marker does not add to `Unread`. Test:
`channelstest` `testReadsNoUnread` (new, with rows `edit` and `action change`)
and `testReminderNoUnread` (new, in `channelstest/reminders.go`), both run by
the filechannels disk and MemFS harnesses, sqlitechannels and pgchannels with a
schema-pinned DSN.
4. **Cost does not grow.** SQL: a non-counting row has `counts = 0`, so
it is outside `channel_messages_counts_idx` and the unread statement is
unchanged; the existing query-plan tests stay green. File: the tail scan still
stops at the first record at or below the marker, so non-counting records above
the marker are read once and nothing below it is read. Test: `filechannels`
`TestReads_TailCost` new case `non-counting run above the marker` (2 000
non-counting messages above the marker in one segment, the marker in the
previous segment: no byte of the previous segment is read);
`TestInbox_UnreadBudget`, `TestInboxBudget_10vs50` and
`TestInboxSummaryBudget_10vs50` unchanged.
5. **Unread counts, the inbox and the badge follow.** With `progress`
non-counting: three `progress` posts by the runner above Alice's marker give
`read.unread` 0 on the channel list and head, no entry in `unread_channels`,
`unread_count` 0 and `unread_channels` 0 in the summary. One `ask` post then
gives 1 everywhere. `PUT .../_read` answers the same count. Test:
`internal/realtime/reads_test.go` `TestInbox_UnreadSkipsNoUnread`;
`internal/dataentry/inbox_summary_test.go` `TestInboxSummary_NoUnreadType` (real
fsstore wiring, HTTP).
6. **Mention and reply rows stay.** A message of a non-counting type that
mentions Bob, or replies to Bob's message, still writes Bob's addressed row and
shows under `mentions` with its seq. `notify: false` on the property still keeps
the mention out. Test: `internal/realtime/inbox_mentions_test.go`
`TestInbox_MentionInNoUnreadType`, `TestInbox_ReplyInNoUnreadType`.
7. **The live inbox does not wake on a non-counting post alone.** A
non-counting post into a channel that has no unread messages for Alice replays
her inbox subscription but sends no frame. A non-counting post into a channel
already listed under her unread channels sends a frame, because the row's
`last_seq` moves; the count does not change. The summary subscription
(`/_inbox/summary`, the sidebar badge) sends no frame in either case, because
the summary has no `last_seq`. Test: `internal/dataentry/live_inbox_test.go`
`TestLiveInbox_NoUnreadPostSendsNoFrame`,
`TestLiveInbox_NoUnreadPostMovesLastSeq`,
`TestLiveInbox_NoUnreadPostSummaryNoFrame`.
8. **The data-entry check warns on a hidden counting type.** `display:
hidden` on a message type that counts as unread gives one warning naming both
paths: `channels.run.messages.claim.display: hidden, but realtime.yaml counts
claim as unread; set unread: false on channel_types.run.message_types.claim`.
`display: collapsed` gives no warning. `display: hidden` with `unread: false`
gives none. It is a warning, never an error, at load, on either file's reload
and in `rela validate`. Test: `internal/dataentryconfig/channels_test.go`
`TestValidateChannels_HiddenCountsUnread` (table against the fake
`ChannelSchema`); `internal/appbuild/realtime_channels_test.go`
`TestRealtimeChannelSchema_MessageType`; `internal/cli/validate_test.go` case
`hidden type counts as unread` (prints the warning, exit 0).
9. **Inbox rows link to their message.** Mention and reply rows link to
`/entity/<type>/<id>?channel=<key>&seq=<seq>`. Unread and waiting rows keep
their link. Test: `frontend/src/views/InboxView.test.ts` case "mention row links
to its seq".
10. **A message link reveals the message once, in the linked channel.**
When `?channel=` opens a channel and `?seq=` is a positive integer,
`ChannelsSection` holds a pending reveal `{key, seq}` for that channel. It
passes `reveal-seq` to `OpenChannel` only while the open channel's key equals
`key`. `OpenChannel` waits until the first page is loaded and the `RlChannel`
ref is mounted, then one more tick so the mount's scroll to the bottom has run,
and calls `channelView.jumpTo(seq)`. A ref that is not mounted means "not
ready", never "not held". `jumpTo` reveals a held message, or sets `jumping` and
emits `jump`, so `onJump` loads older pages without announcing them and reveals
the target once held. After that one call `OpenChannel` emits `revealed`.
`ChannelsSection` then clears the pending reveal and drops `seq` from the URL
with `router.replace`, keeping `channel`. A hidden target is shown in full and
kept for the mount. A collapsed target's run expands. A shown target is scrolled
to and focused. A change of only `seq`, with the same channel, reveals the new
target without toggling, closing or re-fetching the channel. A click on the same
mention twice works, because the first reveal dropped `seq`. Picking another
channel clears the pending reveal, so its seq never goes to the wrong channel,
and closing and reopening the linked channel does not jump again. A missing,
zero, negative or non-numeric `seq`, or a seq that is never held, does nothing
and shows no error. A failed page load while paging for a link shows the
channel's usual load error, as any page load does. Test:
`frontend/src/components/entity/channels/ChannelsSection.deeplink.test.ts` cases
"seq reveals a hidden message on the first page", "seq expands a collapsed run",
"seq on an older page loads and reveals", "older pages for a link are not
announced", "bad seq is ignored", "seq change in the same channel reveals the
new target without re-fetch", "seq is dropped from the URL after the reveal",
"another channel gets no reveal", "reopening the linked channel does not jump
again".
11. **Docs.** `GUIDE-realtime.md` documents `unread` in the message type
table, adds a row to "Which messages count", and states that the key applies
from the next post. `GUIDE-data-entry.md` "Quiet message types" replaces the
known-limit paragraph with the pairing advice, the warning and the message link.
Test: `TestDocsExample_Unread` (AC 1); the prose is reviewed.
12. **Opening a channel reads past a quiet tail.** When the newest held
message of a channel is hidden, `useMarkRead` marks that seq, so the channel
leaves the behind set of the inbox. This is today's behavior; the test pins it
against a later "mark up to the newest shown row". Test:
`frontend/src/composables/useMarkRead.test.ts` case "marks a hidden newest
message".

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one key and one stored flag; the choices are recorded under Approach and Decisions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `notify: false` on a `ref` property (`realtimecfg/raw.go:257`,
`validate.go:526`, `Property.NoNotify`): an opt-out boolean, default true,
decoded as `*bool`, refused where it has no meaning, compiled to a `No*` field.
`unread: false` copies it exactly.
  - `channels.Counts` and the `counts` column: the write-time count
decision already exists; the flag is one more input to it.
  - `MovedSession` on `Message`: a store-kept boolean that rides in the
SQL `doc` and the file `line` record.
  - `BusyFacts` on `ChannelSchema`: a facts struct per name, so the
interface grows by one method and later facts need none.
  - `refusedKeys` (`realtimecfg/keys.go`): a named hint for a key a reader
might guess.
  - `RlChannel.reveal` and `useChannelPaging.onJump` (TKT-03DMXH): the
reveal path a reply quote uses; a message link reuses it as is.
  - `ChannelsSection` `?channel=` deep link: the seq rides beside it.
- Reference implementations: Slack lets bot messages skip the unread
badge per integration, and marks a channel "muted" without hiding mentions;
GitHub keeps a mention in a hidden timeline event in the notification list and
links to the event. Both separate "counts as new" from "addresses you", as this
plan does.
- rela prior art: TKT-03DMXH (Q5 names this follow-up), TKT-2B2AIA (read
markers, `counts` column), TKT-6RXAC0 (addressed rows), TKT-9YRRM5 (live inbox),
DEC-ZBI39P (gating), `.claude/rules/collection-reads.md`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config (`internal/realtimecfg`).**
   - `rawMessageType` gains `Unread *bool yaml:"unread"`. The key checker
picks it up from the struct.
   - `MessageType` gains `NoUnread bool` with the godoc "set by `unread:
false`: messages of the type never count toward a reader's unread". `clone()`
copies it (a value field, so the struct copy already does).
   - `validator.messageType`: when `raw.Unread != nil`, refuse it on an
ephemeral type with the reason in AC 1; otherwise `mt.NoUnread = !*raw.Unread`.
   - `refusedKeys`: `channel_types.*.message_types.*.quiet` and
`channel_types.*.message_types.*.display` with the hints in AC 1.
   - `CheckReload` needs no change: the key is not part of any identity
the reload guards.
2. **Store (`internal/channels`).**
   - `NewMessage.NoUnread bool` and `Message.NoUnread bool
json:"no_unread,omitempty"`, both with a godoc that points at `Counts`. The
store copies it from `NewMessage` when it builds the message
(`sqlchannels/sql.go:468`, `filechannels/ops.go:65`).
   - `Counts(m)` becomes `m.Removed == nil && !IsSessionNotice(m) &&
!m.NoUnread`. It stays the one definition.
   - sqlchannels: `doc` gains `NoUnread bool json:"no_unread,omitempty"`;
`docOf` and `scanMessage` copy it. `insertMessage` and every message `UPDATE`
(edits and action pending, done and clear, `sqlchannels/sql.go:715`) already
call `countsOf(m)` on a message that round-trips through `doc`, so the column
follows. A dropped `doc` field would silently make the row count again; the
`action change` row of `testReadsNoUnread` pins it. No migration: existing rows
have no flag and keep `counts` as stored.
   - filechannels: `line` embeds `Message`, so the record carries the
field. `recordHead` gains `NoUnread bool json:"no_unread,omitempty"` and
`counts()` passes it to `Counts`.
   - Edit: `Store.Edit` takes props and mentions only and rewrites the
stored message, so the flag survives. Remove: the removal clears the message;
`Removed` already makes it non-counting.
   - Reminders: `ReminderPost` (`channels/session.go:169`) gains
`NoUnread bool`, and `ReminderMessage` (`session.go:238`) copies it onto the
`Message`. Both backends already call `ReminderMessage`, so neither backend file
changes for this.
   - `channelstest` gains `testReadsNoUnread` (with `edit` and `action
change` rows), a row in the `testReadsCount` table, and `testReminderNoUnread`
in `channelstest/reminders.go`.
3. **Service (`internal/realtime`).** `service.go:450` and `system.go:62`
set `NoUnread: mt.NoUnread`. `reminderStep` (`reminders.go:246`) sets `NoUnread`
on the `ReminderPost` it returns, from `ct.MessageTypes[r.PostType]` with a
nil-safe lookup; the post type is checked to exist at load
(`realtimecfg/reminders.go:226`). Session notices are written by the store and
are non-counting already. The renderer does not copy the field, so no wire type,
golden or OpenAPI document changes.
4. **Data-entry check.**
   - `dataentryconfig.ChannelSchema` keeps four methods.
`MessageProperties(channelType, messageType) []ChannelProperty` is replaced by
`MessageType(channelType, messageType string) (MessageFacts, bool)`, with
`MessageFacts{Properties []ChannelProperty; CountsUnread bool}`; false for an
unknown type. `channelBlockErrors` (`channels.go:339`) makes one lookup per
message type and reads both facts from it.
   - A sibling `channelBlockWarnings` returns the AC 8 warning for
`display: hidden` when `CountsUnread` is true. `ValidateChannels` appends it to
`warnings`.
   - `appbuild.realtimeChannelSchema.MessageType` reads
`ct.MessageTypes[mt]` and answers its properties as today and `CountsUnread:
!mt.NoUnread`.
   - Every implementation changes with the interface: the fake in
`dataentryconfig/channels_test.go`, `roomSchema` in
`dataentry/realtime_wiring_test.go:38`, and `noChannelTypes` in
`projectsetup/validate_test.go:315`.
5. **SPA.**
   - `InboxView.vue`: `link(c, seq?)` appends `&seq=<seq>` for mention
and reply rows.
   - `ChannelsSection.vue`: one watch on `[query.channel, query.seq]`.
When `channel` changed, it runs today's `applyLink` path. It then sets a
`pendingReveal` ref `{key, seq}` when `seq` parses as a positive integer
(`/^[1-9]\d*$/`), else null. When only `seq` changed to a valid value, it sets
`pendingReveal` for the linked key and sets `selected` to that key directly
(never through the toggling `onSelect`); the list is not re-fetched. A `seq`
that became absent (our own `router.replace`) changes nothing.
   - `ChannelsSection.vue` passes `:reveal-seq` to `OpenChannel` only when
`pendingReveal?.key === open.key`. On `@revealed` it clears `pendingReveal` and
calls `router.replace` with the query minus `seq`. A change of `selected` away
from `pendingReveal.key` clears it too.
   - `rela-components` `RlChannel.vue`: add the existing `jumpTo` to
`defineExpose`. It already runs `reveal`, sets `jumping` and emits `jump`, so
link paging is silent for screen readers in the same way as a reply quote's.
   - `OpenChannel.vue`: prop `revealSeq?: number` and emit `revealed`. A
watch on `[revealSeq, first page loaded, channelView]` waits until all three are
set, awaits `nextTick` so `RlChannel`'s mount scroll has run, calls
`channelView.value.jumpTo(seq)` once, and emits `revealed`. A null ref means
"not ready", so the watch waits rather than paging. The existing
`@jump="onJump"` handles the older pages, and `onHeld` finds the mounted ref.
`OpenChannel.vue` (602 lines, over the warn-only limit) grows by a prop, an emit
and one watch; `useChannelPaging.ts` does not change.
   - `reveal` already keeps a revealed hidden row for the mount and
expands a collapsed run.
6. **Docs.** See Documentation Planning.

**Alternatives considered:**

- *The server reads `data-entry.yaml`.* Rejected by the ticket, and
rightly: presentation would then drive counts, and the CLI and MCP, which never
load `data-entry.yaml`, would disagree with the web server.
- *Derive `display` from `unread`, or the reverse.* Rejected. They answer
different questions. A `status` type a person should see in full but not be
alerted by is `unread: false` with no `display`. A `progress` type folded into a
line may still be worth a badge. A cross-check warning covers the one
combination that is almost always a mistake (Q2).
- *`quiet: true` in `realtime.yaml`.* Rejected for the reason TKT-03DMXH
rejected `audience: agents`: it names intent, not effect. A reader would have to
guess whether a quiet type also hides mentions, skips mail notifications or
folds in the web app. A refused-key hint points `quiet:` to `unread: false`.
- *`counts_unread: false`.* Clear, but no other key in the file is
verb-plus-noun; `notify: false` is the precedent for an opt-out on the same
level.
- *`display:` in `realtime.yaml` (one key for both files).* Rejected in
TKT-03DMXH Q1, and it would still not say anything about counts.
- *Count at read time from the type (no stored flag).* The SQL backends
have no `type` column, so the unread statement would filter on `doc->>'type'`
per row. That leaves the partial index, so a count would scan every message
above the marker instead of at most 100 index entries, against
`collection-reads.md`. `Reads` also takes keys of many channel types in one
statement, so each would need its own list of types. A `type` column plus
migration would fix the index but not the per-type list. The stored flag keeps
the statement and the index as they are.
- *Skip mention and reply rows for a non-counting type.* Rejected (Q4).
- *A seq on every inbox row.* "Unread channels" has no single target
message, and "Waiting" points at a state. Out of scope.

**Dependencies:** `internal/realtimecfg`, `internal/channels` (with
`sqlchannels`, `filechannels`, `channelstest`), `internal/realtime`,
`internal/dataentryconfig`, `internal/appbuild`. No change to
`internal/configedit`, `internal/apiwire`, `internal/mcp`, OpenAPI or the SQL
schemas. Frontend: no new packages; `rela-components` exposes one existing
method (`RlChannel.jumpTo`).

**Files to modify:**

- `internal/realtimecfg/raw.go`, `compiled.go`, `validate.go`, `keys.go`.
- `internal/channels/channels.go`, `reads.go`, `session.go`,
`sqlchannels/sql.go`, `filechannels/ops.go`, `filechannels/tail.go`,
`channelstest/reads.go`, `channelstest/reminders.go`.
- `internal/realtime/service.go`, `system.go`, `reminders.go`.
- `internal/dataentryconfig/channels.go`,
`internal/appbuild/realtime_channels.go`.
- `frontend/src/views/InboxView.vue`,
`frontend/src/components/entity/channels/ChannelsSection.vue`,
`OpenChannel.vue`;
`frontend/packages/rela-components/src/components/channel/RlChannel.vue`.
- Tests: `internal/realtimecfg/unread_test.go` (new),
`docs_example_test.go`, `internal/channels/reads_test.go`, `reminders_test.go`,
`filechannels/reads_test.go`, `internal/realtime/unread_test.go` (new),
`reads_test.go`, `inbox_mentions_test.go`,
`internal/dataentry/inbox_summary_test.go`, `live_inbox_test.go`,
`realtime_wiring_test.go` (`roomSchema`),
`internal/projectsetup/validate_test.go` (`noChannelTypes`),
`internal/dataentryconfig/channels_test.go`,
`internal/appbuild/realtime_channels_test.go`, `internal/cli/validate_test.go`,
`frontend/src/views/InboxView.test.ts`, `ChannelsSection.deeplink.test.ts`,
`frontend/src/composables/useMarkRead.test.ts`.
- Docs: `docs-project/entities/guides/GUIDE-realtime.md`,
`GUIDE-data-entry.md`; `docs/realtime.md` and `docs/data-entry.md` regenerate
through `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`unread:` in `realtime.yaml` (operator).* A boolean, decoded as `*bool`;
any other shape is refused by the key checker with its path. Refused on an
ephemeral type. Checked at load, on reload (a failed reload keeps the last good
config) and by `rela validate`. Configure cannot write `realtime.yaml`, so no
allowlist entry is needed and `TestAllowlist_ClassifiesEveryKey` is unaffected
(it covers `schema.yaml` and `data-entry.yaml` only).
- *`?seq=` in the SPA URL (anyone who can craft a link).* Parsed as a
positive integer, else ignored. It only chooses which already-loaded message to
scroll to. Every page it loads goes through the same gated `GET .../messages`
the channel uses.
- *`NoUnread` on a stored message.* Set only by the service, from the
compiled type. Not accepted from any request body.

**Security-Sensitive Operations:**

- *No new oracle.* The flag is decided from the config and the message
type, never from the reader. Every count is still taken after the gate, per
principal, as in TKT-2B2AIA. A reader learns from a lower count only which types
the config marks, and the config is not a secret (CLAUDE.md). Hidden channels
still change nothing (`TestInbox_UnreadHiddenChangesNothing` stays green). The
live inbox frame rule is unchanged: a replay of a change the reader cannot see
still sends no frame, because the hash is taken over the gated answer.
- *No read path is narrowed.* A non-counting message is readable by
everyone who may read the channel, through every client. `unread: false` is not
access control; the docs say so.
- *The message link* reveals only a message the reader already holds or
can page to. A seq the reader may not read cannot be loaded, so nothing shows,
and no error says whether it exists.
- *Errors.* Load errors name the config path and the reason. Both are
config, not secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestParse_Unread`, `TestCheckReload_UnreadFlip`, `TestDocsExample_Unread` |
| 2 | `TestPost_NoUnreadType`, `TestTriggerPost_NoUnreadType`, `TestReminderStep_NoUnreadType`, `TestRender_NoUnreadNotOnWire`, `TestCounts` rows, `TestReminderMessage_CopiesNoUnread` |
| 3 | `channelstest` `testReadsNoUnread` (with `edit` and `action change` rows) and `testReminderNoUnread` on four backends |
| 4 | `TestReads_TailCost` new case; existing SQL query-plan tests and inbox budget tests unchanged |
| 5 | `TestInbox_UnreadSkipsNoUnread`, `TestInboxSummary_NoUnreadType` |
| 6 | `TestInbox_MentionInNoUnreadType`, `TestInbox_ReplyInNoUnreadType` |
| 7 | `TestLiveInbox_NoUnreadPostSendsNoFrame`, `TestLiveInbox_NoUnreadPostMovesLastSeq`, `TestLiveInbox_NoUnreadPostSummaryNoFrame` |
| 8 | `TestValidateChannels_HiddenCountsUnread`, `TestRealtimeChannelSchema_MessageType`, `validate_test.go` case |
| 9 | `InboxView.test.ts` "mention row links to its seq" |
| 10 | `ChannelsSection.deeplink.test.ts` nine cases |
| 11 | `TestDocsExample_Unread`, reviewed |
| 12 | `useMarkRead.test.ts` "marks a hidden newest message" |

Integration: `testReadsNoUnread` and `testReminderNoUnread` run through the
conformance harness on every backend, including pgchannels on a schema-pinned
DSN. `testReminderNoUnread` drives a reminder through the store's session step,
so it covers `ReminderMessage` as each backend calls it, not the trigger path.
`TestInboxSummary_NoUnreadType` drives the HTTP handler over the real fs wiring
with a `realtime.yaml` based on the PoC (`progress`, `claim`, `resume` with
`unread: false`). The deep-link cases mount `ChannelsSection` with mocked reads
and a real router, through `OpenChannel`, `useChannelPaging` and the real
`RlChannel`, with a `channels:` block that hides `claim` and collapses
`progress`. One case has the target on the first page, so the reveal runs right
after the mount and must win over the mount's scroll to the bottom.

**Edge cases:**

- The key flips to `false` while 20 `progress` messages sit above Alice's
marker: they keep counting until she reads them; new ones do not count. Flipping
back to `true` does not make stored ones count. Documented.
- A channel with only non-counting messages above the marker: `Unread` 0,
not in `unread_channels`, but it sorts in the first group of the read window
(`LastSeq > Seq`), as a channel with only session notices does today. The window
cut at 500 can thus keep it over an older read channel. Accepted; documented in
the code comment of `SortReadWindow`.
- 150 non-counting then 3 counting above the marker: 3 (AC 3).
- 150 counting then 50 non-counting: 100, the cap, as today.
- An edit of a non-counting message: stays non-counting, and the SQL
`UPDATE` writes `counts = 0` again.
- An action change (pending, done, clear) on a non-counting message: the
SQL `UPDATE` recomputes `counts` from the round-tripped `doc` and writes 0
again; the file backend's action patch never touches counting.
- A reminder of a non-counting post type: stored with `NoUnread` on every
backend; a reminder of a counting type counts as today.
- Two processes on different `realtime.yaml` versions during a reload
window (web server, MCP stdio, CLI) can store two posts of one type with
different flags. This follows from deciding at append time; the guide says so.
- A removal of a non-counting message: non-counting through `Removed`.
- A person posts a non-counting type: their own marker moves past it as
for any own post; it never counted for them anyway.
- A reply of a non-counting type to Bob's message: Bob's reply row is
written, the channel's count for Bob stays 0.
- A message link to a seq that has been removed: the removed placeholder
is revealed, as a reply quote would.
- A message link while `hasNewer` is set (the reader scrolled far back
before): the target is held or paged to; "Jump to latest" is unchanged.
- A message link to a seq more than ten pages back: nothing is revealed,
the channel opens at the newest page, no error. Out of scope (Scope).
- A second mention in the same channel (only `seq` changes): the new
target is revealed; the channel is not toggled or re-fetched.
- The same mention clicked twice: the first reveal dropped `seq` from the
URL, so the second click changes the route and reveals again.
- The reader picks another channel before or after the reveal: that
channel gets no `reveal-seq`; the pending reveal is cleared.
- The reader closes and reopens the linked channel: no second jump.
- An older page fails to load while paging for a link: the channel's
usual load error shows, as for Load older.
- The target is on the first page and `RlChannel` mounts in the same
tick: the watch waits for the ref and one tick, so the reveal is not lost and
the mount scroll does not undo it.
- A message link to a channel that is not in the list (deleted): the
existing `?channel=` path finds nothing; `seq` is never used.
- `?seq=` without `?channel=`: ignored.
- Realtime off: `ValidateChannels` returns the existing "realtime is off"
warning and no hidden-counts warning.
- `display: hidden` on a type `realtime.yaml` does not declare: the
existing error, no warning.

**Negative Tests:**

- `unread: "no"` (quoted), `unread: 1`, `unread: [false]`: refused at
load with the path. Unquoted `unread: no` is a YAML boolean and loads as
`false`; a table row pins that so nobody changes it by accident.
- `unread: false` and `unread: true` on an ephemeral type: refused.
- `quiet: true`, `display: hidden` on a message type in `realtime.yaml`:
refused with the hint.
- A request body that carries `no_unread` on a post: refused as an
unknown field by the existing strict decode (no new code; a row in
`TestPost_NoUnreadType` pins it).
- `?seq=0`, `?seq=-3`, `?seq=abc`, `?seq=1.5`: ignored, no error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Config change is not retroactive.* An operator who adds `unread:
false` expects old badges to drop. Mitigated by the docs ("applies from the next
post; open the channel once to clear older ones") and by the fact that opening
the channel clears them. The read-time alternative costs the index range
(Approach).
- *File backend scan over long non-counting runs.* A channel where an
agent posted 2 000 `progress` notes since Alice last looked makes her unread
scan read 2 000 record heads, where the SQL backends read none. This is the same
order as a run of session notices today, and the scan still stops at the marker.
Pinned by the AC 4 case. The file backend is the small-project tier; a "last
counting seq" in `reads.json` could cut it later if it shows up.
- *Two keys to keep in step.* An operator may hide a type in
`data-entry.yaml` and forget `realtime.yaml`. Mitigated by the AC 8 warning and
one docs paragraph that shows both together.
- *Downgrade.* An older binary ignores `no_unread` in stored messages
(`encoding/json` ignores unknown fields; no backend decodes strictly), so it
counts them again, and refuses `unread:` in `realtime.yaml` as an unknown key.
Both are what a downgrade past a new key does elsewhere.
- *Live inbox frame on a non-counting post into an already unread
channel* (AC 7). The row's `last_seq` moves. Accepted: "Mark read" must mark up
to the true last message, and the frame is one refetch.
- *Plimsoll.* `MessageType` gains one exported field (12 to 13 of 20);
`Message`, `NewMessage` and `ReminderPost` stay below 20. `ChannelSchema` stays
at four methods: `MessageType` replaces `MessageProperties`.
- *A missed write path.* The design review found the reminder path, which
never reaches `Append`. Mitigated by setting the flag on all three paths and by
`testReminderNoUnread`, which goes through the store's session step on every
backend.
- *Deep-link state in the SPA.* A pending reveal that outlives its link
could jump in the wrong channel or jump again. Mitigated by tying it to the
linked key, using it once and dropping `seq` from the URL, each pinned by a
deep-link case.

**Effort:** m. About 40 lines of Go config and validation, 25 in the store
across two backend families and the reminder message, 10 in the service across
three write paths, 50 for the data-entry warning and the `MessageType` change
over four implementations, 70 of SPA code (pending reveal, URL cleanup, one
exposed library method), and most of the work in tests on four backends and nine
deep-link cases. The review findings add work within m. One PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: `schema.yaml` is unchanged)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change; `rela validate` prints the new warning through its existing path)
- [x] `GUIDE-realtime.md`:
  - message type key table (line 223 area): a row for `unread`, "Default
`true`. `unread: false` keeps the type's messages out of every unread count and
out of the inbox's unread channels. Mentions and replies still reach the inbox;
use `notify: false` on a property for that. Not allowed on a signal type." With
a PoC example (`progress`, `claim`, `resume`), pinned by
`TestDocsExample_Unread`.
  - "Which messages count" table: a row "A message of a type with
`unread: false` | No", and a sentence that the key applies to messages posted
after the change. One more sentence: each process decides from its own loaded
config, so during a reload the web server, an MCP process and the CLI can
briefly store posts of one type with different flags.
  - "In the web app" paragraph (line 1968): one sentence that `unread:
false` belongs beside `display` for agent-only types.
- [x] `GUIDE-data-entry.md`, "Quiet message types": replace the
known-limit paragraph (line 1471) with: pair `display` with `unread: false` in
`realtime.yaml` so the badge agrees with what you see; the validator warns when
a hidden type still counts; an inbox mention or reply opens the channel at that
message and shows it even when its type is hidden or collapsed. A short note on
the `?seq=` link under "Inbox": the link reveals the message once and then drops
`seq` from the address.
- [x] `docs/realtime.md`, `docs/data-entry.md`: regenerated from the
guides by `just docs`, not edited by hand.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows `notify: false` and the `counts` column)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Decisions

- **Q1. The key.** `channel_types.<ct>.message_types.<mt>.unread: false`.
It mirrors `notify: false`: an opt-out boolean, default true, named after its
effect, refused where it means nothing. Rejected: `quiet:` (intent, not effect),
`counts_unread:` (no precedent), `display:` (a presentation key in a structural
file).
- **Q2. Relation to `display:`.** Independent: neither is derived from
the other, and the server never reads `data-entry.yaml`. When they disagree,
each does its own job. `display: hidden` with counting is a warning, not an
error, because a project may stage the two files. `display: collapsed` with
counting gets no warning: the fold line is a visible row, so the badge points at
something. `unread: false` without `display` is a valid choice (seen in full, no
alert).
- **Q3. When the flag is decided.** At append time, stored on the message.
A key change applies from the next post. Rejected: read-time from the type
(leaves the partial index; see Alternatives).
- **Q4. Mention and reply rows.** Unchanged: a non-counting type still
writes them. A mention is addressed to a person; dropping it would hide
something that person must see. `notify: false` per property already gives an
operator that choice for mentions. The SPA message link makes the row usable
when the type is hidden.
- **Q5. The SPA reveal.** Mention and reply rows link with `&seq=`; the
entity page reveals the message through the reply-quote path, bounded at ten
older pages. A load-around page for older targets is a follow-up if it is
needed. The reveal runs through `RlChannel.jumpTo`, so link paging is as quiet
for screen readers as a reply quote's. It runs once per link, only in the linked
channel, and then drops `seq` from the URL. A bad or never-held seq shows
nothing; a failed page load shows the channel's usual error, because that
failure is about the channel, not the link.
- **Q6. Wire.** `unread` is not served in `Describe`, MCP, OpenAPI or
`_config`; `no_unread` is not on any message. No golden changes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

Thirteen findings: one critical, two significant, seven minor, three nits. All
are addressed in the plan; none is declined.

- *Critical: reminder write path.* Reminders build their message through
`ReminderPost` and `channels.ReminderMessage`, not `Append`. `ReminderPost`
gains `NoUnread`, `ReminderMessage` copies it, and `reminderStep` sets it from
the post type. New tests: `TestReminderStep_NoUnreadType`,
`TestReminderMessage_CopiesNoUnread` and `channelstest` `testReminderNoUnread`
on four backends. The old `TestSystemPost_NoUnreadType` is renamed
`TestTriggerPost_NoUnreadType` because it tests the trigger path (AC 2, AC 3,
Approach 2 and 3).
- *Significant: seq-only change.* `ChannelsSection` watches
`[query.channel, query.seq]`. A change of only `seq` reveals the new target
without toggling or re-fetching the channel (AC 10).
- *Significant: reveal seq scope and lifetime.* The pending reveal is
`{key, seq}`, passed only to the `OpenChannel` of that key, used once, and
cleared on `revealed`, on a switch to another channel, and by dropping `seq`
from the URL with `router.replace` (AC 10).
- *Minor.* Link paging goes through an exposed `RlChannel.jumpTo`, so
pages are not announced (one-line library change). The reveal waits for the
`RlChannel` ref and one tick, and treats a missing ref as not ready; a
first-page case is added. AC 10 now says a failed page load shows the usual
error. The two other `ChannelSchema` implementations are listed.
`TestLiveInbox_NoUnreadPostSummaryNoFrame` pins the badge subscription. AC 12
pins that opening a channel marks a hidden newest message. `testReadsNoUnread`
gains an `action change` row.
- *Nits.* `MessageProperties` becomes `MessageType`, returning
properties and `CountsUnread`, so `ChannelSchema` stays at four methods. The
negative table says the refused `"no"` is quoted and pins unquoted `no` as
`false`. The guide notes that processes may disagree on the flag during a
reload.
