---
id: PLAN-GMEUX4
type: planning-checklist
title: 'Planning: Inbox 3: principal mentions and replies'
started: "2026-10-27"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slices 1 and 2 (TKT-ZZ4PNX, TKT-2B2AIA) answer "which sessions wait
on me" and "which of my channels have new messages". Nobody can address a person
or an agent in a message yet: `internal/mentions` finds entity ids only, and a
reply tells its parent's author nothing. RES-B0MJAZ slice 3 adds principal
mentions and replies: a code span names a person or a job identity, the store
keeps one row per addressed principal beside the message, and the inbox lists
the unread ones after the gate. The decisions of 2026-10-27 fix the frame: rows
live inside `channels.Store`; a mention is a code span with a person entity id
or a job identity; anyone who may post may mention, and entries are gated at
read time; a typed `ref` property to the user entity type counts as a mention by
default; no mail in this slice. The composer `@` picker is split into a
follow-up (see the follow-up file); this slice renders mentions as chips.

Terms used below:
- **Person mapping**: the ACL maps a login to a person entity. It is on when
`acl.yaml` sets both `user_entity_type` and `principal_property`
(`acl.Policy.PrincipalPropertyLookupEnabled`, the same test as
`appbuild.pilePersonType`). Only then does the web app stamp `principal.User`
with the person entity id.
- **Mention user type**: the user entity type when person mapping is on,
else empty. `realtimecfg.Compiled.MentionUserType()` is the one predicate that
picks the mention form everywhere (classification, render, docs).
- **Principal key**: the string an inbox belongs to. It is `principal.User`
as `channels.Reader.User` records it: the person entity id in the web app and
over remote MCP when person mapping matched; the raw login over stdio, over the
CLI, and in the web app when mapping is off or did not match; the job identity
for a job (`integration:triage-bot`). The organization is not part of it.
- **Target**: a principal key that one message addresses, stored with its
form (`user`, `job` or `login`).
- **Addressed row**: one stored row per (channel, seq, target, kind). Kind is
`mention` or `reply`.
- **Mention entry** / **reply entry**: an inbox entry built from addressed
rows, one per (channel, seq), after the gate.
- **Candidate span**: a code span whose whole text could name a principal:
an entity id, a job identity, or `@` followed by a login.
- **Read**: an addressed row is read for an inbox reader when that reader's
marker on the channel (slice 2, keyed by user and organization) is at or above
its seq. No marker means seq 0.

**Scope:**
- In: `mentions.Spans`, the shared code-span parser; `All` is rebuilt on it.
Its result changes only for a span that holds a line break, which is no longer a
span (AC 1).
- In: classification and resolution of targets in `internal/realtime` on post
and edit, in the one ref batch the post already makes; the cap of 10 targets per
message (`422 too_many_mentions`).
- In: `notify: false` on a ref property in `realtime.yaml`.
- In: store: `channels.Mention`, `NewMessage.Mentions`, `Message.Mentions`,
a new `Edit` argument, reply rows computed by the store, `ListAddressed`, the
per-target cap, on every backend (filechannels on disk and on MemFS,
sqlitechannels, pgchannels through sqlchannels), pinned by `channelstest`.
- In: schema: `channel_mentions` with two indexes in sqlite `schemaSQL` and
rung v28 to v29, and postgres migration 0036; `mentions.json` per channel
directory on the file tier.
- In: inbox kinds `mention` and `reply`; `mentions` and `mention_count` on
the first page; wake keys; REST, MCP `list_inbox`, CLI `rela channel inbox`.
- In: `mentions` on `MessageView` (REST message pages, post and edit answers,
MCP `read_channel`, live frames through the same views).
- In: SPA: person, job and login mention chips in rendered messages; i18n en
and nl for any new text.
- In: MCP watch wake for the target's own `list_inbox` watch by exact
principal key; MCP tool descriptions (`post_message`, `read_channel`,
`list_inbox`).
- In: docs (`docs/realtime.md`, `docs/mcp-server.md`, `docs/cli-reference.md`,
`docs/acl-security.md`, `docs/data-entry.md`, docs-project `GUIDE-realtime`).
- In: budget tests (post, message page, inbox), explain tests, and ACL tests
that hidden rows change nothing.
- Out: the composer `@` picker and the `mentions` block on
`GET /api/v1/_schema/channels` (follow-up a). People type the span by hand until
then; agents and the CLI need no picker.
- Out: re-stamping the stdio and CLI principal to the person entity id
(follow-up b). This slice documents the limitation.
- Out: moving rows and markers on a person rename or delete (the slice 2
forget-reader follow-up, TKT-6V8K8I, gets one line for it).
- Out: mail and desktop notifications (TKT-Z3PS9U); the SPA inbox page,
sidebar badge, `/_inbox` live shape and `i:` topics (TKT-9YRRM5); live frames
across processes (TKT-YK30D1).
- Out: mentions written by the trigger engine, timers and reminders. Only a
person's or agent's post or edit through `Service` writes mention rows.
- Out: wildcard mentions (`@all`, `@channel`), group or role mentions,
"mentions me" highlighting, a per-entry "done" state, snooze, per-entry read
flags.
- Out: a policy that restricts who may mention job identities (research
option c3); a later ticket.
- Out: backfill of mentions in messages stored before the upgrade. They were
written under no mention semantics; they never notify.
- Out: resolving `@login` to a person entity when person mapping is on.

**Example:**

With person mapping on, a colleague posts in the `breach-response` channel of
`RISK-2`:

```text
`USR-7` can you check the log? `integration:triage-bot` please summarise.
```

The post resolves `USR-7` through the poster's gate as a user entity, and stores
two addressed rows (target `USR-7`, target `integration:triage-bot`). USR-7's
inbox, first page:

```json
GET /api/v1/_inbox
"mentions": [
  { "kind": "mention",
    "channel": { "anchor": "RISK-2", "entity_type": "risico", "anchor_title": "Server room",
                 "type": "breach-response", "name": "7m2c…", "title": "Breach 13", "state": "open" },
    "seq": 15, "at": "2026-10-27T10:04:00Z", "message_type": "note",
    "author": { "user": "USR-3" },
    "links": { "entity": "/api/v1/risicos/RISK-2",
               "channel": "/api/v1/risicos/RISK-2/_channels/breach-response/7m2c…",
               "message": "/api/v1/risicos/RISK-2/_channels/breach-response/7m2c…/messages/15" } }
],
"mention_count": 1
```

The message as a reader who may read USR-7 sees it:

```json
{ "seq": 15, "type": "note", "props": { "body": "`USR-7` can you check …" },
  "mentions": [
    { "text": "USR-7", "kind": "user", "ref": { "id": "USR-7", "type": "person", "title": "Ann de Vries" } },
    { "text": "integration:triage-bot", "kind": "job" } ] }
```

A reader who may not read USR-7 gets only the job entry; the span renders as
plain code.

**Acceptance Criteria:**

1. `mentions.Spans(body)` returns the distinct texts of the complete, plain
code spans of body in document order, skipping link text and code blocks, with
the GFM extension set the renderer uses. A code span that holds a soft or hard
line break is not returned, because the server and the SPA would read it
differently. `All` returns what it returns today, built on `Spans`, except that
it drops such a span too. Test: `TestSpans` table in `internal/mentions` (order
kept, duplicates once, span in link text skipped, fenced and indented blocks
skipped, span in a table cell found, a span holding emphasis markup is not
plain, "`USR-\n7`" refused, empty body nil), and the existing `TestAll` rows
plus one line-break row.
2. Classification (`realtime.classifySpans`) depends on the mention user
type, not on `user_entity_type` alone. With person mapping on, a span that
passes `entity.ValidateID` is a user candidate. With it off, no such span is,
and a span `@x` is a login target `x` when x is 1 to `channels.MaxTextBytes`
bytes of NFC UTF-8 with no whitespace, control character, Unicode format
character (category Cf, such as U+200B and bidi marks), backtick or NUL, is not
`principal.Unknown`, and is not `principal.IsReserved`. With person mapping on,
an `@x` span is plain code. A span that passes `principal.ValidJobIdentity` is a
job target in every deployment (rela's own identities such as `system:realtime`
fail it). Only the first 64 candidate spans in document order are considered.
The three forms cannot collide: an entity id holds neither `:` nor `@`. Login
matching is exact and case-sensitive, as `acl.yaml` assignments are. Test:
`TestClassifySpans` table (each form with mapping on and off; `user_entity_type`
set without `principal_property` uses the login form; `@system:x` and
`@integration:x` are not logins; `@unknown`; `@jeroen​` and a non-NFC login
refused; 70 id spans keep 64; `integration:Bad Name` refused).
3. Post resolution: user candidates are resolved in the same single
`RefResolver.Resolve` batch as the message's ref properties, through the
poster's gate. A user target is a candidate whose resolved type is the mention
user type, stored as the resolved `ref.ID`, not the span text. A ref property
value of that type is a user target unless the property says `notify: false`.
Job and login targets are stored as written. The poster's own key is dropped.
More than `channels.MaxMentions` (10) distinct targets is 422
`too_many_mentions`; nothing is stored and the rate token is spent, as for any
invalid message. The poster's answer does not depend on what they cannot see,
stated as two equalities: a hidden user, a missing entity and a non-user entity
give equal answers (no chip), and a readable user who can read the channel and
one who cannot give equal answers (a chip). Tests: `TestPost_MentionTargets`
(store fake records `NewMessage.Mentions`; a lower-case span `usr-7` that the
resolver matches stores `USR-7`), `TestPost_MentionAnswerIsUniform` (the two
equalities, comparing JSON apart from seq, rev and time),
`TestPost_TooManyMentions` (11 readable users: 422; 10: stored; 11 spans of
which one is a hidden user: stored, because the hidden one is no target),
`TestPost_RefPropMention` (a `handoff.to` ref to the user type is a target; with
`notify: false` it is not; a ref to another type is not).
4. `Append` with `NewMessage.Mentions` writes one addressed row per target
(kind `mention`) and stores the targets with their form on `Message.Mentions`,
in the same store step as the message. An append whose `ReplyTo` names a parent
writes one reply row for the parent author's user when `channels.ReplyTarget`
holds: the parent's `TriggeredBy` is empty, its author user is not empty, not
`principal.Unknown`, not a rela identity, and differs from the poster's user.
The comparison is on the user only, because rows name a user only: a reply by
the same user name from another organization writes no row. A failed append
writes no row. Invalid targets (empty, over `MaxTextBytes`, not UTF-8, NUL,
leading or trailing space, unknown form) are `ErrInvalidMessage`; more than
`channels.MaxMentions` distinct targets is `ErrTooLarge`. Test:
`testAppendAddressed` in `channelstest` (rows seen through `ListAddressed`;
reply to own message: no row; reply to a trigger post or a reminder: no row;
parent author `unknown` or empty: no row; same user, other org: no row; reply
that also mentions the parent author: a mention row and a reply row; stale
incarnation, closed channel with `RequireOpen`, bad reply target: no row;
invalid targets refused).
5. `Edit` takes the full new target set. It replaces the message's mention
rows: rows of kept targets keep their time, rows of added targets get the edit
time, rows of dropped targets go; an empty set drops them all. The reply row
stays. It replaces `Message.Mentions`. `Remove` deletes every row of the
message, the reply row included, and clears `Message.Mentions`. Removing a
parent keeps the reply rows of its replies. Test: `testEditRemoveAddressed` in
`channelstest` (includes "edit that removes the only mention", which pins the
empty-set SQL).
6. A service edit keeps a target the editor cannot read when its span or
ref value is still in the new props: the new set is the editor's own resolution
plus (old targets that are still candidates). The 10-target check runs on that
final set in `Service.Edit`, so the store never sees more than 10 and never
answers `ErrTooLarge` to an edit. The 422 may then count a target the editor can
no longer see; this is accepted, because the editor wrote that id while they
could see it. An edit never moves a session and never writes a reply row. Tests:
`TestEdit_KeepsUnreadableMention` (author loses read access to USR-7, edits
another word: the row stays), `TestEdit_DropsRemovedMention`,
`TestEdit_AddedMentionWakesTarget`, `TestEdit_TooManyWithKeptHidden` (9 visible
plus 2 kept hidden: 422; 8 plus 2: stored).
7. Lifecycle and caps. `Delete` and `Purge` erase a channel's addressed
rows; `Rekey`, `Tombstone` and `Untombstone` carry them. A target keeps at most
`channels.MaxAddressedPerTarget` (200) rows in one channel. When a new row would
go over, the store frees one slot in the same step, in this order: the oldest
row of that target in the channel that is read under every marker the target
user holds there (at least one marker must exist); else the oldest unread row of
that target whose message has the same author user as the new row's message;
else the new row is not written (logged with key, seq and target, no content).
One poster can therefore never erase another poster's unread mention. The file
tier also caps a channel's `mentions.json` at `MaxAddressedPerFile` (5000) rows
with the same order across targets. Tests: `testListAddressed`; a `Raw` check on
the SQL backends that no `channel_mentions` row survives `Delete` or `Purge`;
`testAddressedTargetCap` (200 unread rows from A, then a post by B: B's row is
skipped, A's rows stay; target marks read up to seq 50: B's next row evicts the
oldest read row; 200 unread rows from A, A posts again: A's own oldest unread
row goes); `TestAddressed_FileCap` in filechannels.
8. `ListAddressed(r, limit)` returns the addressed rows whose target is
`r.User`, on live channels, with a seq above r's marker on that channel, taken
from the target's newest `channels.MaxAddressedScan` (2000) rows, newest first
(time descending, then key, then seq), at most limit (1 to
`channels.MaxAddressedWindow`, 500, else `ErrInvalidMessage`). Each row carries
the full `Channel`, seq, time, kind, the message's author and type, and for a
reply the parent seq. Test: `testListAddressed` in `channelstest` (order; a
marker at the seq hides the row, a marker below keeps it; the same user in
another organization has its own marker; a tombstoned channel is absent and
comes back after `Untombstone`; a `Rekey` keeps the rows under the new id;
another target's rows absent; 2001 read rows newer than one unread row: the
unread row is outside the scan and absent, as documented).
9. Upgrade and fresh databases: a fresh sqlite database and a fresh postgres
schema have `channel_mentions` and both indexes; a sqlite database at v28 and a
postgres schema at 35 gain them. Running it again changes nothing. Old message
docs read with no `Mentions`. Tests: `TestMigrate_v28ToV29ChannelMentions` in
`internal/sqlitedb`, `TestMigration0036_ChannelMentions` in pgstore
(schema-pinned DSN), `status_test.go` pinned at 36, the ladder well-formedness
tests, and `TestDBImportFS_SQLiteTablesAreAccounted` with `channel_mentions`
listed.
10. Inbox: `kinds` accepts `waiting`, `unread`, `mention` and `reply`; the
default is all four. When `mention` or `reply` is asked and there is no cursor,
the answer carries `mentions` (unread entries of the asked kinds, newest first,
at most `MaxInboxMentions` = 50) and `mention_count` (a pointer: the entries in
the gated window, at most 500). One entry per (channel, seq): `kind` is
`mention` when a mention row exists, else `reply`. A cursor without `waiting` in
kinds is 400 `invalid_cursor`. The `inbox_too_large` detail names
`kinds=unread,mention,reply`. Tests: `TestInbox_Mentions`, `TestInbox_Kinds`
(extended rows: `kinds=mention` makes no holder query, no `ListWaiting` and no
`ListReads`; `kinds=reply` lists replies only), `TestInboxHandler_Kinds`
(`kinds=mention,reply` accepted, `kinds=x` 400, cursor with `kinds=mention` 400,
`mention_count: 0` present).
11. Gate before count, per principal: a row whose anchor the caller cannot
read, whose `read:` role the caller fails (named, permission and creator roles),
whose anchor face the caller cannot read, or whose channel type the
configuration no longer declares, yields no entry and does not change
`mention_count`. The inbox answer is byte-identical to the answer for a project
where those rows do not exist. Tests: `TestInbox_MentionHiddenChangesNothing`
(realtime, fakes) and `TestInboxMention_ACL` (dataentry, real `acl.yaml`,
fsstore, filechannels).
12. Read state: an entry leaves the inbox once the target's marker on the
channel reaches its seq (`mark_read`, `PUT …/_read`, or the target's own later
post there). A mention creates no marker. Removing the message removes the
entry. Test: `TestInbox_MentionReadByMarker` (dataentry: post mentioning B; B's
inbox lists it; B marks read up to seq; gone; a new mention at a higher seq
lists again; the author removes it; gone).
13. Cost. A post makes at most one `Resolve` call, whether it names 1 or 10
mentions, and no extra store call besides the `Append`. A message page makes one
`Resolve` for its refs and mentions together. The inbox makes one
`ListAddressed` call beside slice 2's calls, with the one shared anchor batch
and role prime, and decodes no message document for it. Each count is the same
at 10 and 50 rows. Tests: `TestChannelsBudget_PostMentions`,
`TestChannelsBudget_MessagesMentions` (50 messages, each with two user
mentions), `TestInboxBudget_10vs50` (extended: 10 and 50 mention rows, half on
hidden anchors, a relation-conferred `permission:` read role). The SQL plans are
pinned: the inner scan of `ListAddressed` reads `channel_mentions_target_idx` in
index order with no sort below its LIMIT (sqlite: no `USE TEMP B-TREE` for the
inner ORDER BY; pg: an Index Scan under the Limit with no Sort node), and the
cap statement reads `channel_mentions_cap_idx`. Tests:
`addressed_explain_test.go` in sqlitechannels (modernc driver) and
`channel_mentions_explain_test.go` in pgstore.
14. Rendering: `MessageView.Mentions` lists, for each stored target of a
message that is not removed, by its stored form: a readable user as `{text,
kind: "user", ref: {id, type, title}}`; a job target as `{text, kind: "job"}`; a
login target as `{text: "@<login>", kind: "login"}`. `text` is the span text the
SPA matches. A user the reader cannot read is absent: no id, no kind. A removed
message has none. Titles come from redacted properties. Render trusts the stored
form; it does not re-derive it from current config, so a later change of person
mapping does not reinterpret old messages. Tests: `TestRender_Mentions` (two
readers, one who cannot read USR-7; an old login target `jeroen` stays a login
after mapping is switched on), golden wire file, `wireFields.test.ts`.
15. SPA rendering: `renderMessageMarkdown` turns a code span whose whole text
is a `user` mention's text into the entity link refs already get, with the
server's title, and a `job` or `login` mention's span into a mention chip built
with DOM APIs (`textContent`, no `innerHTML` from data). Spans in links and code
blocks stay as they are; a span with no matching mention stays code. Tests:
`markdown.test.ts` (rows for each kind; a title holding markup renders as text;
a hidden user stays code), `channelView.test.ts`, `RlChannelMessage.stories.ts`
story.
16. MCP: `list_inbox` lists `mentions` and `mention_count` and takes the new
kinds; `post_message` and `read_channel` describe the syntax and the `mentions`
field. A `watch` on `list_inbox` wakes when a post or edit in this process
addresses the caller's principal key, also for a channel outside `WakeKeys`; it
does not wake for a mention of someone else, nor when the watch's kinds hold
neither `mention` nor `reply`. `WakeKeys` also holds the gated mention entries'
channel keys. Only subscriptions that opted in with `SubscribeAddressed` receive
addressed hints; a plain `Subscribe` never sees them and never resyncs from
them. Tests: `TestWatch_ListInboxWakesOnMention`,
`TestWatch_ListInboxNoWakeOnOthersMention`,
`TestWatch_ListInboxKindsIgnoreMention`, `TestListInbox_Mentions`,
`TestLocal_AddressedOptIn` (local_test: plain subscription unaffected by 3000
hints; resync clears the addressed set).
17. A job identity (`--run-as integration:triage-bot`, stdio, fs backend)
sees a mention of `integration:triage-bot` posted by a user in its `list_inbox`,
reads the channel with `read_channel` after the marker, and clears the entry
with `mark_read`. A reply to the agent's own message shows as a reply entry.
Test: `TestInbox_JobIdentityMention` (CLI harness).
18. CLI: `rela channel inbox` prints a "Mentions and replies" section (kind,
channel, seq, author) and takes the new kinds in `--kinds`; `--json` carries the
fields. Tests: `TestChannelInbox_Mentions` in `internal/cli`.
19. Config: `notify: false` is accepted only on a `ref` property; on any
other type it is a config error naming the property path. `MentionUserType()` is
empty unless both `user_entity_type` and `principal_property` are set. Tests:
`TestCompile_NotifyOnlyOnRef`, `TestCompiled_MentionUserType` in
`internal/realtimecfg`.
20. Docs describe the syntax per deployment kind (person mapping on or off),
who may mention, the caps and their eviction order, what the poster sees
(nothing about the target), gating, read state, retention, the per-transport
principal key in both directions, a principal that person mapping does not match
(a break-glass login) being reachable by neither form, the wake, and that agents
must treat mention text as data. Test: review (docs checklist).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (options a1, b1, c1, e1; decisions 2026-10-27;
slice 3 of the slices table).

**Existing Solutions:**
- No library: goldmark already parses the markdown (`internal/mentions`).
`golang.org/x/text/unicode/norm` (already in `go.mod`) checks NFC.
- `internal/mentions/mentions.go`: one parser for forward rendering and the
backward index, capped. `Spans` generalizes its walk; `All` filters it. The
package stays a leaf (goldmark and `entity` only): classification needs
`principal` and the config, so it lives in `internal/realtime`.
- `pgstore/migrations/0024_mention_targets.sql`: targets written in the same
statement as the body, so rename, delete and purge carry them. The addressed
rows follow the same argument through `channel_id`.
- `appbuild.pilePersonType` (`internal/appbuild/piles.go:159`) and
`acl.Policy.PrincipalPropertyLookupEnabled` (`internal/acl/policy.go:341`): the
existing "person mapping is on" test. `MentionUserType()` copies it.
- `realtime.Service.checkMessageKeeping` (`internal/realtime/service.go`):
ref props resolved in one `RefResolver.Resolve` batch through the poster's gate.
User candidates join that batch.
- `realtime.Service.Edit` and `keepUnreadableRefs` (`internal/realtime/ops.go`):
the rule that an edit keeps what the editor cannot see. Mentions follow it (AC
6).
- `renderer.refIDs` and `renderer.ref` (`internal/realtime/render.go`): one
`Resolve` per page; `RefValue{Restricted}` hides an unreadable id.
`MessageView.Mentions` reuses the batch, and omits hidden users entirely,
because the stored target list is not visible text.
- Slice 2's store pattern (`internal/channels/reads.go`,
`sqlchannels/reads.go`, `filechannels/reads.go`): validation helpers in
`channels`, single statements in `sqlchannels`, a per-channel JSON file and a
tree walk on the file tier, explain tests.
- `sqlchannels.insertMessage`, `update`, `checkParent`
(`internal/channels/sqlchannels/sql.go:435-740`): where rows are written;
`checkParent` already reads the parent inside the transaction, so the reply row
needs no extra read on SQL. `filechannels.replyParent` does the same on the file
tier (`ops.go:211`). The SQL `doc` struct (`sql.go:160`) is separate from
`channels.Message` and gains `Mentions`.
- Slice 1 and 2 inbox (`internal/realtime/inbox.go`): `inboxRun.prime` (one
anchor batch, one role prime), `anchorOf`, `roleAdmits`, `InboxView` marshalling
by kind, `unread_channels` as a first-page list.
- `Local.SubscribeReads` / `markFor` / `TakeReads`
(`internal/realtime/local.go`): an opt-in tagged wake. `SubscribeAddressed`
copies it, including the resync clear in `Take` (`local.go:171`).
- `watchChannel.matches`, `matchesKey`, `learnInbox`
(`internal/mcp/tools_watch.go:160-210`).
- SPA: `renderMessageMarkdown` and `linkMessageRefs`
(`frontend/src/utils/markdown.ts:400-450`) already turn ref code spans into
links with DOM APIs.
- Identity: `QueryIdentity` and `queryIdentityFor`
(`internal/predicatefns/currentuser.go`,
`internal/appbuild/nextaction_matchers.go:162`), `access.Opener` re-stamping
(`resolveIdentity`), `principal.ValidJobIdentity`, `IsReserved`,
`IsRelaIdentity`.
- Prior art (from the research): Zulip and Matrix (mention a stable id, not
a display name; Matrix's explicit mentions avoid spoofing); Zulip warns the
sender when the target cannot read, which in rela would be an oracle, so rela
does not; GitHub notifications (a reason per entry, hidden when access is lost);
Slack (mention badges counted apart from unread).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Parser** (`internal/mentions/mentions.go`):
   ```go
   // Spans returns the distinct texts of the complete, plain code spans of
   // body, in document order. A span in link text, or one that holds a line
   // break, is skipped; code blocks are never spans. It is the one walk All
   // and principal mentions share.
   //
   // Nil: returned when body has no spans.
   func Spans(body string) []string
   ```
`codeSpanText` reports a span that has more than one text segment or a line
break as not plain. `All` becomes `Spans` filtered by `entity.ValidateID`,
sorted. The stores keep calling `Targets` and `Index`.

2. **Store types** (`internal/channels/addressed.go`, new):
   ```go
   const (
       MaxMentions           = 10   // targets per message; realtime uses this one constant
       MaxAddressedPerTarget = 200  // rows one target keeps in one channel
       MaxAddressedPerFile   = 5000 // rows one channel's mentions.json keeps (file tier)
       MaxAddressedScan      = 2000 // newest rows of a target ListAddressed reads
       MaxAddressedWindow    = 500  // most rows one ListAddressed returns
   )
   type MentionForm string // "user", "job", "login"
   type Mention struct {
       Target string      `json:"target"`
       Form   MentionForm `json:"form"`
       Span   string      `json:"span,omitempty"` // span text as written, when it differs from Target
   }
   type AddressKind string
   const (
       AddressMention AddressKind = "mention"
       AddressReply   AddressKind = "reply"
   )
   // Addressed is one addressed row as ListAddressed returns it.
   type Addressed struct {
       Channel Channel
       Seq     int64
       At      time.Time
       Kind    AddressKind
       Author  Author // the message's author
       Type    string // the message's type
       ReplyTo int64  // the parent seq, for a reply
   }
   func CheckMentions(ms []Mention) ([]Mention, error) // dedupe by target, sort, validate
   func ReplyTarget(parent Message, poster Author) (string, bool)
   func CheckListAddressed(r Reader, limit int) error
   func SortAddressed([]Addressed)
   ```
`NewMessage.Mentions []Mention`; `Message.Mentions []Mention`
(`json:"mentions,omitempty"`), set by the store from `NewMessage`, replaced by
`Edit`, cleared by `Remove`. `PrepareMessage` runs `CheckMentions`.
`ReplyTarget` returns false for an empty user, `principal.Unknown`, a rela
identity, a parent with `TriggeredBy`, or the poster's own user.
`realtimelimits` gets no copy of the cap: `realtime` uses `channels.MaxMentions`
(channels does not import realtimelimits, and realtimelimits imports nothing
from rela).

`Message.ReplyTarget string` (`json:"-"`) carries the reply row's target back
from `Append` so that `Post` can wake it without reading the parent. Doc
comment: "Zero: always, except on the message `Append` returns, where it names
the reply row's target, if one was written. Never stored." Changing `Append`'s
return type instead would touch every store and fake for one field.

3. **Store methods** (`internal/channels/store.go`):
   ```go
   Edit(ctx context.Context, key Key, seq int64, props map[string]any, mentions []Mention, at time.Time) (Message, error)
   ListAddressed(ctx context.Context, r Reader, limit int) ([]Addressed, error)
   ```
`Append` keeps its signature. The plimsoll directives on `sqlchannels.Store` and
`filechannels.Store` go from 33 to 34, with the history line, as a
required-interface exception.

4. **sqlchannels** (`internal/channels/sqlchannels/addressed.go`, new;
edits in `sql.go`, `retention.go`):
   - `doc` gains `Mentions []channels.Mention` so the targets reach the
database and come back on read.
   - `insertAddressed(ctx, q, chanID, msg, rows)`: one multi-row `INSERT INTO
channel_mentions (channel_id, seq, target, kind, at, author_user, author_tool,
author_org, author_type, msg_type, reply_to) VALUES …` per message, after the
cap step for each target. `Append` calls it after `insertMessage` with the
mention targets and, when `checkParent` returns the parent and `ReplyTarget`
holds, the reply row. `checkParent` returns the parent message it already reads.
The author and type columns never change: an edit keeps author and type, and a
removal deletes the rows.
   - `Edit` (`update`'s change func): with an empty new set, `DELETE FROM
channel_mentions WHERE channel_id = ? AND seq = ? AND kind = 'mention'`;
otherwise the same with `AND target NOT IN (…)`, then the cap step and `INSERT …
ON CONFLICT DO NOTHING` for the new set at the edit time, so kept rows keep
their time.
   - `Remove`: `DELETE FROM channel_mentions WHERE channel_id = ? AND seq = ?`.
   - Cap step, per (channel, target) about to gain a row:
`SELECT seq, kind, author_user FROM channel_mentions WHERE channel_id = ? AND
target = ? ORDER BY seq` (at most 200 entries of `channel_mentions_cap_idx`).
Below 200: nothing. At 200: read the target user's markers on the channel
(`channel_reads` primary key prefix); delete the oldest row with seq at or below
the lowest marker, else the oldest row whose `author_user` equals the new row's,
else skip the new row.
   - `ListAddressed`:
     ```sql
     SELECT chanCols, a.seq, a.at, a.kind, a.author_user, a.author_tool,
            a.author_org, a.author_type, a.msg_type, a.reply_to
       FROM (SELECT * FROM channel_mentions
              WHERE target = ? ORDER BY at DESC, channel_id DESC, seq DESC LIMIT 2000) a
       JOIN channels c ON c.id = a.channel_id AND c.tomb = ''
       LEFT JOIN channel_reads r ON r.channel_id = a.channel_id
            AND r.reader_user = ? AND r.reader_org = ?
      WHERE r.seq IS NULL OR r.seq < a.seq
      ORDER BY a.at DESC, c.anchor_id, c.anchor_face, c.type, c.name, a.seq
      LIMIT ?
     ```
The inner scan reads at most 2000 entries of `channel_mentions_target_idx
(target, at DESC, channel_id DESC, seq DESC)` in index order. There is no join
to `channel_messages` and no document decode: rows are small and fixed-width.
- `Delete` and `Purge` (`retention.go` erase) delete `channel_mentions`
rows before `channel_messages`, with `ON DELETE CASCADE` on `channel_id` as a
backstop. `Rekey`, `Tombstone` and `Untombstone` change only `channels` columns,
so rows travel.
- `Raw` dumps `channel_mentions` rows for the conformance comparison.

5. **Schema.**
   - sqlite: `schemaVersion` 29. `channelMentionsDDL` goes into `schemaSQL`
(fresh databases, as `channelReadsDDL` does) and into rung v28 to v29
(`addChannelMentions`). No rung adds its columns, so the indexes can sit in
`schemaSQL` too.
     ```sql
     CREATE TABLE IF NOT EXISTS channel_mentions (
       channel_id  INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
       seq         INTEGER NOT NULL,
       target      TEXT    NOT NULL,
       kind        TEXT    NOT NULL CHECK (kind IN ('mention', 'reply')),
       at          INTEGER NOT NULL,
       author_user TEXT    NOT NULL,
       author_tool TEXT    NOT NULL DEFAULT '',
       author_org  TEXT    NOT NULL DEFAULT '',
       author_type TEXT    NOT NULL DEFAULT '',
       msg_type    TEXT    NOT NULL,
       reply_to    INTEGER NOT NULL DEFAULT 0,
       PRIMARY KEY (channel_id, seq, target, kind)
     ) STRICT;
     CREATE INDEX IF NOT EXISTS channel_mentions_target_idx
       ON channel_mentions(target, at DESC, channel_id DESC, seq DESC);
     CREATE INDEX IF NOT EXISTS channel_mentions_cap_idx
       ON channel_mentions(channel_id, target, seq);
     ```
No backfill.
- postgres: `0036_channel_mentions.sql`: the same table with `BIGINT`
columns and `TEXT COLLATE "C"` for target and author columns, both indexes.
Header comment in the style of 0035, including: the `REFERENCES channels(id)`
constraint takes a SHARE ROW EXCLUSIVE lock on `channels` for the length of the
CREATE; it is brief because the new table is empty.
- `internal/store/pgstore/status_test.go` pins 36.
- `internal/cli/db_importfs_sqlite_test.go` lists `channel_mentions` with
the TKT-DD8B6I "not copied yet" reason.
- The `datamigration` rule does not apply: these are schema rungs. Other
in-flight branches that add a rung renumber on rebase; both ladder tests catch a
clash.

6. **filechannels** (`internal/channels/filechannels/addressed.go`, new;
edits in `ops.go`):
   - `mentions.json` beside `meta.yaml` and `reads.json`: `{"v":1,
"rows":[{"seq","target","kind","at","author":{…},"type","reply_to"}]}` sorted by
seq, written atomically (temporary file, rename) through the rooted FS. The two
caps of AC 7 apply, with markers from `reads.json` in the same directory. A
damaged file reads as no rows with a warning; the next write replaces it.
   - Order inside the lock, so the index never points at a message that does
not address the target: rows that go are written out before the log record (edit
drops, removal), rows that come after it (append, edit adds). `mutate` has no
hook after its rewrite, so `Append` and `Edit` get their own post-write step
inside the lock. A failed write before the log record fails the operation; a
failed write after it is logged as a warning (key, seq, no content) and
swallowed, as slice 2 does for `reads.json`, because the message is already
durable.
   - Self-healing: every row write for a message rebuilds that message's rows
from `Message.Mentions` plus its reply row (kept if present, else recomputed
from the parent read under the same lock). So a lost write is repaired by the
next edit of that message, instead of being treated as "kept". A crash loses a
notification, never invents one.
   - `ListAddressed`: one tree walk as `ListReads`: per channel directory,
read `mentions.json` (a missing file is one failed open); for a directory with
rows for the target, read `reads.json` and `meta.yaml`; keep rows above the
marker. Collect the target's rows, keep the newest 2000 before the marker filter
(the SQL scan rule), sort, cut. An inbox call on the file tier therefore walks
the tree twice (`ListReads` and `ListAddressed`); this is accepted and added to
the fs scale note, because merging them would put an inbox-shaped method on the
store interface.
   - Delete, purge, rekey, tombstone and untombstone move or erase the
directory, so rows follow with no new code. The MemFS store is the same code.

7. **Config** (`internal/realtimecfg`):
   - `Property.Notify *bool` (`yaml:"notify,omitempty"`); compile refuses it on
a property that is not `ref` (AC 19).
   - `Compiled.MentionUserType() string`: the policy's `UserEntityType` when
`PrincipalPropertyLookupEnabled()`, else `""`, taken at compile time.
Classification, render and docs use only this predicate.
   - `configedit` keeps `notify` locked by default (no allowlist change).

8. **realtime service** (`internal/realtime/mentions.go`, new; edits in
`service.go`, `ops.go`, `render.go`, `inbox.go`, `local.go`, `errors.go`;
arch-lint lets `realtime` depend on `mentions`):
   - `classifySpans(spans []string, userType string) (users []string, others
[]channels.Mention)` (AC 2), on the markdown properties of the message type only
(strings and enums are never parsed).
   - `checkMessageKeeping` adds the user candidates to the ref ids of its one
`Resolve` call and returns `targets []channels.Mention`: resolved candidates of
the mention user type as `{ref.ID, user}`, ref values of that type without
`notify: false`, job and login targets, minus the caller's `principal.User`,
deduped and sorted. Over `channels.MaxMentions` is `tooManyMentions()` (422,
`TypeTooManyMentions`). The ref validity check is unchanged: a span that does
not resolve is not an error, it is just not a target.
   - `Post` sets `NewMessage.Mentions`. After the append,
`s.local.markAddressed(targets ∪ {m.ReplyTarget}, c.key)`.
   - `Edit` computes `targets = resolved ∪ (m.Mentions ∩ candidates(new
props))` (AC 6), checks the cap on that set, calls `store.Edit(…, targets, …)`,
and marks `targets \ m.Mentions` addressed.
   - `renderer`: `refIDs` also collects the targets of form `user` of
`Message.Mentions`. `message` fills `Mentions []MentionView` (AC 14) from the
stored form:
     ```go
     type MentionView struct {
         Text string    `json:"text"`
         Kind string    `json:"kind"` // user, job, login
         Ref  *RefValue `json:"ref,omitempty"`
     }
     ```
`Text` is `Mention.Span` when set, else the target, so the SPA matches the span
as written while the row holds the canonical id. A user target whose `Resolve`
answer is missing or not of the user type is left out.
- **Inbox** (`inbox.go`):
  - Kinds `InboxKindMention = "mention"`, `InboxKindReply = "reply"`;
`inboxKinds` returns a small struct of four booleans.
  - `InboxView.Mentions []InboxMention` (`json:"mentions,omitzero"`),
`MentionCount *int` (`json:"mention_count,omitempty"`), set whenever a mention
kind is asked on the first page.
  - `InboxMention{Kind string; Channel InboxChannel; Seq int64; At
time.Time; MessageType string; Author AuthorView; ReplyTo int64}`.
  - Steps on the first page: `ListAddressed(reader,
MaxAddressedWindow)`; drop rows of undeclared types and of kinds not asked; add
their channels to the one `prime` call (it takes a third candidate list); per
row `anchorOf` and `roleAdmits` as `unread` does, with the role caches reset per
channel; group by (channel, seq), mention before reply; `MentionCount` = groups;
keep the first 50; add the gated channel keys to `WakeKeys`.
  - Nothing is cached; every call answers for its own principal.
- **Wake** (`local.go`): `Local.SubscribeAddressed()` returns a
subscription with `allAddressed` set, as `SubscribeReads` sets `allReaders`.
`markAddressed(targets, key)` adds the key to `changed` for every subscription,
as `mark` does, and the targets to `addressed` only on opted-in subscriptions,
bounded by the same `max` (overflow is a resync). `Take`'s resync branch clears
`addressed` too. `TakeAddressed() []string` returns and empties the set. The
live hub does not opt in.

9. **REST** (`internal/dataentry`, `internal/apiwire/v1`):
   - `inbox_handler.go`: parse the new kinds; map `Mentions` and
`MentionCount` with links (`entity`, `channel`, `message`); `v1.InboxMention`.
   - `channels_wire.go`: `v1.Message.Mentions []MessageMention`.
   - `channels_errors.go`: `too_many_mentions` maps to 422.
   - Golden wire file and the TS mirror list the new types and fields.
   - `internal/openapi/channels.go`: inbox fields and kinds enum, the
message `mentions`, the 422 code.

10. **MCP** (`internal/mcp`):
    - `tools_inbox.go`: kinds argument accepts the four kinds; description
of `mentions`: "An entry points at a message that mentions you or replies to
yours. Read it with read_channel (after = your marker), act, then mark_read.
Entries carry no message text."
    - `tools_channels.go`: `post_message` description: "To mention a person,
put their user entity id in backticks (`USR-7`) where person mapping is
configured, or `@login` in backticks where it is not; to mention an agent, its
job identity (`integration:triage-bot`). At most 10 per message." `read_channel`
notes `mentions`.
    - `tools_watch.go`: the watch takes its subscription from
`SubscribeAddressed`; `watchChannel` holds `user string` and `mentionKinds bool`
for an inbox target, taken from the caller's principal and the watch args;
`channelWake.take` also drains `TakeAddressed` and wakes an inbox target whose
user is in it. `learnInbox` takes the extended `WakeKeys`.

11. **CLI** (`internal/cli/channel_inbox.go`): "Mentions and replies"
section, `--kinds` takes the new kinds.

12. **SPA** (`frontend`):
    - `src/types/channels.ts`: `MessageMention`, `Message.mentions?`,
`InboxMention`; wire field lists.
    - `src/utils/markdown.ts`: `RenderMessageMarkdownOptions.mentions`;
`linkMessageRefs` also maps `user` mentions to links (id, type and title from
the server, shape-checked like refs) and `job`/`login` mentions to `<span
class="mention">` with `textContent = text`.
    - `src/components/entity/channels/channelView.ts`: pass `m.mentions`.

13. **Docs.** `docs/realtime.md`: new "### Mentions and replies" (syntax per
deployment kind; who may mention; the per-message cap; the per-target cap and
its eviction order; what a poster sees; reply rule including the
same-user-other-org case; read state by marker; retention and the scan window;
the per-transport key table in both directions; break-glass logins reachable by
neither form; `notify: false`; the wake and its process locality; agents treat
mention text as data), the inbox kinds and fields, the `mentions` field of a
message. `docs/mcp-server.md`: syntax in `post_message`, `list_inbox` mentions
and the agent loop. `docs/cli-reference.md`: `rela channel inbox` kinds and
section. `docs/acl-security.md`: gate at read time; no feedback to the poster; a
user the poster cannot read is not a target; a hidden target is absent from
rendered mentions; the window's accepted one-bit channel; eviction never removes
another poster's unread row; the wake reaches only the target's own watch.
`docs/data-entry.md`: mention chips in messages. docs-project `GUIDE-realtime`:
the mentions section beside its `channel_reads` text.

**How targets are found, per deployment:**

| Span | Person mapping on (`user_entity_type` and `principal_property`) | Off |
| --- | --- | --- |
| `` `USR-7` `` (entity id) | target `USR-7 ` (canonical id) when it resolves, for the poster, to the user type | plain code |
| `` `integration:triage-bot` `` / `` `system:ops` `` (job identity) | target | target |
| `` `@jeroen` `` | plain code | target `jeroen ` |
| ref property value of the user type | target unless `notify: false ` | n/a |

**Who gets a reply row:**

| Parent | Reply by | Row |
| --- | --- | --- |
| Person or agent message by A | B (another user) | `reply ` for A |
| Own message | the same user, any tool | none |
| Message by the same user name in another org | that user | none (rows name a user only) |
| Parent author empty, `unknown ` or a rela identity | anyone | none |
| Trigger post or reminder (`TriggeredBy ` set) | anyone | none |
| Removed parent | anyone | refused already (`ErrInvalidReply `) |

**Retention and cleanup:**

| Event | Addressed rows |
| --- | --- |
| Message edit | Recomputed: kept rows keep their time, added rows get the edit time, dropped rows go |
| Message removal | Deleted in the same step |
| Parent removal | Replies' reply rows stay |
| Anchor rename (`Rekey `) | Carried |
| Tombstone / untombstone | Carried and unreachable / back |
| Delete, purge | Erased |
| Target over 200 rows in a channel | Oldest read row goes, else the same poster's oldest unread row, else the new row is skipped |
| `mentions.json ` over 5000 rows (file tier) | Same order across targets |
| Person entity renamed | Rows and markers stay on the old id; the open mention is lost to the person. Moved with markers by the forget-reader follow-up (TKT-6V8K8I) |
| Person entity deleted | Rows kept as strings, never shown to anyone else. A new entity reusing the id sees author and channel of entries only for channels it may read, never content. Same follow-up |

**Alternatives rejected:**
- Store every entity-id span as a target, with no lookup (the research's
"no lookup at post time"): ten ticket ids would use up the cap and push the
person out. The post already makes one `Resolve ` batch, so the lookup is free
in calls.
- Pick the form on `user_entity_type ` alone: without `principal_property `
every key is a login, so id mentions would notify no one, silently.
- Classify by id prefix of the user type: prefixes are optional and not
unique to a type.
- Resolve user candidates with an allow-all reader: it stores rows for
people the poster cannot see.
- Drop a target that cannot read the channel at post time (research c2):
needs another principal's ACL on the write path, and the read gate is needed
anyway.
- `@handle ` text (research b2) and a structured `mentions ` field (b3):
rejected by decision.
- Mixing mention entries into `entries ` with a new cursor: two orderings
(oldest wait first, newest mention first) do not page together.
- A per-channel cap that evicts the lowest seqs: any poster could erase
every unread mention in a channel.
- Joining `channel_messages ` in `ListAddressed `: carries up to 2000 full
documents through the sort; the six immutable columns are cheaper.
- Writing reply rows from the service: it would need the parent read before
the append (a race with removal). The store reads the parent inside the step
already.
- Rows for every thread participant on a reply: noisy. Later, with a follow
setting.
- An org column for the target: a mention names no organization, and
organizations are attribution only.
- Re-deriving the form on render: a later change of person mapping would
reinterpret old targets. The stored form is the post's decision.
- Warning the poster that a target cannot read the channel (Zulip): an
existence and access oracle.
- One combined fs walk for reads and mentions: needs an inbox-shaped store
method; the cost is noted instead.

**Dependencies:** `internal/mentions `, `internal/channels ` and its backends,
`internal/sqlitedb `, `internal/store/pgstore/migrations `,
`internal/realtimecfg `, `internal/realtime `, `internal/dataentry `,
`internal/apiwire/v1 `, `internal/openapi `, `internal/mcp `, `internal/cli
`, `frontend ` (SPA). One new arch-lint edge: `realtime ` may depend on
`mentions ` (a leaf). `golang.org/x/text/unicode/norm ` (already a
dependency).

**Files to modify:**
- `internal/mentions/mentions.go ` (+ test)
- `.go-arch-lint.yml ` (`realtime ` → `mentions `)
- `internal/channels/addressed.go ` (new, + test), `channels.go `
(`NewMessage.Mentions `, `Message.Mentions `, `Message.ReplyTarget `,
`PrepareMessage `), `store.go `
- `internal/channels/channelstest/channelstest.go ` (register; Edit call
sites), `addressed.go ` (new), `session.go ` (Edit call site)
- `internal/channels/sqlchannels/sql.go ` (`doc.Mentions `, Append, Edit,
Remove, `checkParent `), `retention.go `, `addressed.go ` (new), directive
- `internal/channels/filechannels/ops.go `, `addressed.go ` (new), directive,
tests (`TestAddressed_WriteOrder `, `TestAddressed_DamagedFile `,
`TestAddressed_Heals `, `TestAddressed_FileCap `)
- `internal/channels/sqlitechannels/addressed_explain_test.go ` (new)
- `internal/sqlitedb/sqlitedb.go ` (`schemaSQL `, `schemaVersion `),
`migrate.go ` (+ migrate test)
- `internal/store/pgstore/migrations/0036_channel_mentions.sql ` (new),
`status_test.go ` (36), `channel_mentions_explain_test.go `, migration test
- `internal/cli/db_importfs_sqlite_test.go `
- `internal/realtimecfg ` (`Notify `, compile check, `MentionUserType `, tests)
- `internal/realtime/mentions.go ` (new), `service.go `, `ops.go `,
`render.go `, `inbox.go `, `local.go `, `errors.go `, tests
(`mentions_test.go `, `inbox_test.go `, render tests, `local_test.go `,
`helpers_test.go ` fakes)
- `internal/dataentry/inbox_handler.go `, `channels_wire.go `,
`channels_errors.go `, `channels_budget_test.go `, `inbox_budget_test.go `,
`channels_wire_golden_test.go ` and testdata, new handler tests
- `internal/apiwire/v1/channels.go `
- `internal/openapi/channels.go ` (+ test)
- `internal/mcp/tools_inbox.go `, `tools_channels.go `, `tools_watch.go `,
tests
- `internal/cli/channel_inbox.go ` (+ test)
- `frontend/src/types/channels.ts `, `src/utils/markdown.ts `,
`src/components/entity/channels/channelView.ts `, tests;
`frontend/packages/rela-components/.../RlChannelMessage.stories.ts `
- `docs/realtime.md `, `docs/mcp-server.md `, `docs/cli-reference.md `,
`docs/acl-security.md `, `docs/data-entry.md `,
`docs-project/entities/guides/GUIDE-realtime.md `
- Test fakes of `channels.Store ` across packages (compile-driven:
`internal/realtime/helpers_test.go `, `internal/dataentry ` counting store,
`internal/mcp/channels_fixture_test.go `, appbuild tests)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Message markdown (post, edit; REST, MCP, CLI): parsed by goldmark; only
complete plain code spans without line breaks are candidates. Each form has an
allowlist grammar: `entity.ValidateID `, `principal.ValidJobIdentity `, and
the login rule of AC 2 (NFC, no Cf, no whitespace or control). At most 64
candidates are considered.
- Ref property values: already validated as ids and resolved through the
poster's gate; a target only when resolved to the mention user type, and stored
as the canonical id.
- Targets at the store: `CheckMentions ` (non-empty, at most `MaxTextBytes `,
UTF-8, no NUL, no surrounding space, known form, at most 10).
- `kinds ` (REST, MCP, CLI): allowlist of four names; anything else 400.
- `notify: ` in `realtime.yaml `: boolean, only on ref properties; config
error otherwise.
- `mentions.json ` on disk: parsed as data; a damaged file reads as empty.
- Rendering: the server sends id, type and title; the SPA shape-checks id and
type as it does for refs and builds the link and chips with DOM APIs. Login text
is NFC without format characters, so a chip cannot look like another login.

**Security-Sensitive Operations:**
- No oracle on post: the poster's answer depends only on what the poster can
see (AC 3's two equalities). A user the poster cannot read is simply not a
target. The `too_many_mentions ` count on a post is taken over targets the
poster can see. On an edit it may include a kept target the editor can no longer
see; that id was written by the editor while visible, so it reveals nothing new.
- Gate at read time, before count: mention entries go through the same
anchor batch (row gate, redaction, faces), role prime and `roleAdmits ` as
slice 1 and 2 entries; `mention_count ` is taken after the gate (CLAUDE.md
aggregate rule). AC 11 pins it.
- The window is cut before the gate, the accepted one-bit channel already
recorded for slice 2: hidden rows occupy scan slots. A caller with more than
2000 rows could infer that some hidden mention exists by watching a visible old
one drop out. It reveals no channel, content or author, needs someone to have
addressed the caller, and is recorded in `docs/acl-security.md `.
- Rendering: a mentioned user the reader cannot read is absent from
`mentions `, so the API never states that an id is a user entity to a reader
who cannot read it.
- Privacy of rows: `ListAddressed ` reads only rows targeting the caller's
own key, taken from the bound principal, never from the request.
- Wake: an addressed hint reaches only opted-in subscriptions, and wakes only
watches bound to the same principal key; the replay re-gates, and a watch
reports a change only when the gated answer changes, so a mention in a hidden
channel causes no visible event and no timing signal to anyone else.
- Prompt injection: anyone who may post can put text in front of an agent by
mentioning its job identity. Entries carry no message text; the text comes only
through `read_channel `, which keeps the `UntrustedNotice `. The docs say
agents treat it as data; a policy that restricts mentions of job identities is a
later ticket (research c3).
- Abuse: at most 10 targets per message, the post rate limit, 200 rows per
target per channel, 5000 rows per channel file, and a 2000-row scan per target
bound storage and inbox cost. Eviction removes read rows first and otherwise
only the same poster's own unread rows, so a flood by one poster cannot erase
another poster's unread mention. The cost of a full target is that further
mentions from other posters to that target in that channel are skipped until the
target reads; this is recorded in the docs.
- Write path: rows are channel-store state, outside the graph: no entity
audit record, version or search (comments rule).
- Personal data: a row records that one principal addressed another in a
channel. It is never shown to anyone but the target, and only while the target
may read the channel. It goes with the message or channel. The forget-reader
follow-up extends to targets, including rename; the docs list rows with the
other per-user data.
- Errors: store faults are logged with user, tool, channel key and seq, never
content or targets; clients get the generic channel failure text.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Package |
| --- | --- | --- |
| 1 | `TestSpans `, `TestAll ` | mentions |
| 2 | `TestClassifySpans ` | realtime |
| 3 | `TestPost_MentionTargets `, `TestPost_MentionAnswerIsUniform `, `TestPost_TooManyMentions `, `TestPost_RefPropMention ` | realtime |
| 4 | `testAppendAddressed ` | channels/channelstest (four backend runs) |
| 5 | `testEditRemoveAddressed ` | channels/channelstest |
| 6 | `TestEdit_KeepsUnreadableMention `, `TestEdit_DropsRemovedMention `, `TestEdit_AddedMentionWakesTarget `, `TestEdit_TooManyWithKeptHidden ` | realtime |
| 7 | `testListAddressed `, `testAddressedTargetCap `, `Raw ` check, `TestAddressed_WriteOrder `, `TestAddressed_Heals `, `TestAddressed_FileCap ` | channelstest, sqlchannels, filechannels |
| 8 | `testListAddressed ` | channelstest |
| 9 | `TestMigrate_v28ToV29ChannelMentions `, `TestMigration0036_ChannelMentions `, `status_test.go `, ladder tests, `TestDBImportFS_SQLiteTablesAreAccounted ` | sqlitedb, pgstore, cli |
| 10 | `TestInbox_Mentions `, `TestInbox_Kinds `, `TestInboxHandler_Kinds ` | realtime, dataentry |
| 11 | `TestInbox_MentionHiddenChangesNothing `, `TestInboxMention_ACL ` | realtime, dataentry |
| 12 | `TestInbox_MentionReadByMarker ` | dataentry |
| 13 | `TestChannelsBudget_PostMentions `, `TestChannelsBudget_MessagesMentions `, `TestInboxBudget_10vs50 `, `addressed_explain_test.go `, `channel_mentions_explain_test.go ` | dataentry, sqlitechannels, pgstore |
| 14 | `TestRender_Mentions `, golden wire, `wireFields.test.ts ` | realtime, dataentry, frontend |
| 15 | `markdown.test.ts `, `channelView.test.ts `, `RlChannelMessage.stories.ts ` | frontend |
| 16 | `TestWatch_ListInboxWakesOnMention `, `TestWatch_ListInboxNoWakeOnOthersMention `, `TestWatch_ListInboxKindsIgnoreMention `, `TestListInbox_Mentions `, `TestLocal_AddressedOptIn ` | mcp, realtime |
| 17 | `TestInbox_JobIdentityMention ` | cli (stdio wiring) |
| 18 | `TestChannelInbox_Mentions ` | cli |
| 19 | `TestCompile_NotifyOnlyOnRef `, `TestCompiled_MentionUserType ` | realtimecfg |

Integration: the dataentry tests run the real router, real `acl.yaml ` (with
person mapping on, and one variant with `user_entity_type ` but no
`principal_property `), fsstore and filechannels; the budget tests also run
under the `sqlite ` tag where the channel budget tests already do. The pg
conformance run covers `Append `, `Edit `, `Remove ` rows, the cap,
`ListAddressed ` and migration 0036 through the schema-pinned DSN. Optional
e2e: user A posts `` `USR-7` ``; B (USR-7) opens the inbox API and sees the
entry; B opens the channel (slice 2 marks it read); the entry is gone.

**Edge Cases:**
- A message that mentions the same person twice: one target, one row.
- Self-mention: dropped; no row.
- A reply that mentions the parent author: one mention row and one reply row;
one inbox entry, kind `mention `.
- A reply to one's own message from another tool or org: no row.
- A reply to a trigger post, a reminder, or an `unknown ` author: no row.
- Mention of a user who cannot read the channel: row stored; no entry; an
entry appears once access is granted.
- Mention of a user the poster cannot read: no row; the span renders as code
for the poster.
- Lower-case span on a case-insensitive store: stored as the canonical id.
- `user_entity_type ` without `principal_property `: login form; id spans are
plain code.
- A login that person mapping does not match (break-glass): reachable by
neither form while mapping is on (documented).
- Person mapping switched on later: old login targets still render as
logins (stored form).
- A person entity deleted after the mention: row kept; render omits it (the
resolve misses).
- An entity id that is a ticket: not a target, not counted against the cap.
- 11 spans where one is a hidden user: 10 targets, stored.
- 70 id spans: the first 64 are considered.
- `@login ` with person mapping on: plain code.
- An email login without mapping: `` `@ann@example.org` `` is target
`ann@example.org `.
- `` `@jeroen` `` followed by U+200B inside the span: refused.
- A span with formatting inside, or across a line break: not a target.
- Mention in a non-markdown property (string, enum): not parsed.
- Edit that removes the only mention: row gone (empty-set SQL), entry gone.
- Edit that adds a mention to a message the target has already read: row
written, entry hidden by the marker (documented).
- Edit by an author who lost access to a mentioned user: the mention stays
while its span stays; it counts toward 10.
- Ephemeral signal with a span: never stored; no row.
- Closed or archived channel: entries still listed (the channel may be read).
- Face-anchored channel: the face gate applies per row.
- Channel type removed from config: entries drop out.
- Two orgs, one user: one row; each org's marker decides read state; the cap
counts a row as read only when every marker is past it.
- Stdio user `jeroen ` and web user `USR-7 ` (same person): two keys; a
mention of `USR-7 ` is seen in the web app only, and a reply to a message
posted over stdio targets `jeroen ` and shows over stdio only (documented).
- 2001 rows newer than an unread one: outside the scan (documented).
- Target at 200 unread rows from other posters: new rows skipped and logged.
- Damaged `mentions.json `: read as empty with a warning; the next write
rewrites it.
- Failed `mentions.json ` write after the log record: post succeeds; the
notification is lost; logged; the message's next edit restores its rows.
- A pg or sqlite upgrade over channels with old messages: no rows; old
messages render without `mentions `.
- A watch whose kinds is `["waiting"] `: no wake on mentions.
- The live hub subscription under a flood of mentions: no addressed set, no
resync.

**Negative Tests:**
- Post with 11 readable user mentions: 422 `too_many_mentions `; nothing
stored.
- Edit whose final set (visible plus kept hidden) exceeds 10: 422, never
- Store `Append ` with an empty target, a NUL, 201 bytes, an unknown form, or
11 targets: `ErrInvalidMessage ` / `ErrTooLarge `.
- `ListAddressed ` with limit 0 or 501, or an invalid reader:
`ErrInvalidMessage `.
- `kinds=mentions ` (typo) or `kinds= `: 400. A cursor with `kinds=mention `:
400 `invalid_cursor `.
- `notify: false ` on a string property: config error.
- MCP `list_inbox ` with `kinds: ["bogus"] `: tool error.
- Store fault in `ListAddressed `: inbox 500 generic, no partial answer.
- `list_inbox ` through `UnavailableChannels `: the refusal text.
- A job identity in the wrong grammar (`integration:Bad `): not a target.
- A forged `mentions ` field in a post body: unknown field, 400 (props are
the only input; targets are never taken from the client).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Edit signature change.** `channels.Store.Edit ` gains an argument; every
fake and the conformance call sites follow. Mitigation: compile-driven; few call
sites (`internal/realtime/ops.go ` and `channelstest `).
- **Wrong deployment kind.** Picking the form on the wrong flag would make
mentions notify no one, silently. Mitigation: one predicate (`MentionUserType
`) shared with the piles test, and an integration variant without
`principal_property `.
- **fs cost.** Two tree walks per inbox call (`ListReads `, `ListAddressed `),
and each row write rewrites a file of at most 5000 rows under the project-wide
lock. Mitigation: the caps, the slice 1 watch pacing (15 s poll, 2 s wake gap),
and the fs scale note.
- **fs atomicity.** The file tier cannot write the log and the index in one
step. Mitigation: drop-before, add-after ordering and rebuild-on-write, so a
crash only loses a notification and the next edit heals it.
- **Full target skips new rows.** A target with 200 unread rows from other
posters in one channel stops receiving rows there until they read. Mitigation:
documented; it is the price of never erasing another poster's unread mention,
and 200 unread mentions in one channel is already an abuse signal.
- **Scan window hides old unread entries** for a principal with more than
2000 newer rows. Mitigation: documented as a fixed window, never a completeness
claim; the count is the gated window.
- **Per-transport keys.** A person who uses both stdio and the web app has
two keys. Mitigation: documented in both directions; follow-up b; agents use job
identities, which are the same everywhere.
- **Prompt injection of agents** through mentions. Mitigation: pointers only
in the inbox, `UntrustedNotice ` on `read_channel `, docs, and the c3 ticket
later.
- **Behaviour change for the demo agent.** The default `list_inbox ` now
carries `mentions `. Mitigation: docs; tell the agent-runner session when this
merges (memory note).
- **Rung clash.** Other branches may add sqlite v29 or pg 0036. Mitigation:
renumber on rebase; the ladder tests catch it.
- **Query plans.** Pinned by explain tests on the modernc driver and on pg
(AC 13), including index order without a sort.

**Effort:** L, with the picker split off: two store methods and three row writes
on three backends with two schema rungs, the parser and classifier, the inbox
merge, rendering, REST, MCP and CLI.

**Design decisions (taken as recommended on 2026-10-27, revised by the design
review):**
1. A person-id span counts only when it resolves, through the poster's gate,
to the mention user type, and is stored as the canonical id.
2. The id form needs person mapping (`user_entity_type ` and
`principal_property `); otherwise the `@login ` form applies.
3. Mentions are a separate first-page inbox list (`mentions `,
`mention_count `), unread only, no cursor.
4. More than 10 targets is a 422, also on edit counting kept hidden targets.
5. A reply notifies the parent's author only, compared on user.
6. The composer `@ ` picker and its schema block are a follow-up; chips stay.
7. Rows name a user only; read state uses the target's own (user, org)
marker.
8. An edit-added mention on an already read message stays hidden.
9. Self-mentions write no row; trigger, timer and reminder posts write no
mention rows in this slice.
10. Any identity in the `ValidJobIdentity ` grammar is a job target; a policy
limiting who may mention agents is a later ticket.
11. Caps are per target per channel (plus a file-tier total), evicting read
rows first, then the same poster's own rows, else skipping the new row.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change; `notify: ` is a realtime.yaml key documented in docs/realtime.md)
- [x] docs/cli-reference.md - `rela channel inbox ` mention section and kinds
- [x] docs/data-entry.md - mention chips in messages
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the gate-before-fold rule and the slice 1 and 2 inbox pattern as written)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] docs/realtime.md - "Mentions and replies" section, `notify: `, inbox kinds and fields, message `mentions `, caps, retention, identity table, wake, agents and untrusted text
- [x] docs/mcp-server.md - mention syntax in `post_message `, `list_inbox ` mentions, agent loop
- [x] docs/acl-security.md - gate at read time, no feedback to the poster, hidden users absent from `mentions `, the scan window's one-bit channel, eviction rule, wake scope
- [x] docs-project GUIDE-realtime - mentions section beside `channel_reads `

## Design Review

- [x] Run `/design-review ` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 22 findings in `.ignored/dr-6rxac0.tsv ` (1
critical, 4 significant, 11 minor, 6 nit), all folded into this plan;
resolutions per finding in `.ignored/dr-6rxac0-res.txt `.
