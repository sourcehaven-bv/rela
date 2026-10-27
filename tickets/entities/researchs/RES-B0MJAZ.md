---
id: RES-B0MJAZ
type: research
title: 'Per-principal inbox across channels: read markers, mentions, waiting on you'
summary: 'Per-principal inbox: markers and mention rows in channels.Store (all backends); code-span mentions of person ids and job identities, gated at read; exact capped unread counts for opened channels; derived waiting entries via the gated current-user graph query (no awaits: key); GET /_inbox, MCP list_inbox with watch and mark_read, live inbox topic; six slices, waiting-first for the demo.'
status: done
---

# Draft research: per-principal inbox across channels

Status: draft for user review. Builds on RES-1UCUBA (channels, process locality,
the multi-instance seam) and RES-O8I8S7 (sessions, `waiting_for`). Code
citations are against `origin/develop` at b8ca16ac2 (2026-10-27, sessions 4/6
merged).

Terms used in this document:

- **Principal key**: the string an inbox belongs to. It is
`QueryIdentity.ID()`: the user entity id when `acl.yaml` declares
`user_entity_type`, else the raw identity, such as `jeroen` or
`integration:triage-bot`.
- **Read marker**: per (channel, principal), the highest message `seq` the
principal has read.
- **Pushed entry**: a record written when a message addresses a principal (a
mention or a reply). It is stored.
- **Derived entry**: an entry computed at read time, such as "this session
waits on you". It is not stored.
- **Gate**: the read check of a channel for one reader: the anchor entity is
readable (row gate), the channel type's `read:` role admits the reader, and the
channel belongs to the live entity (incarnation).

## Problem

Today a principal sees channels one entity at a time (`GET
/api/v1/{plural}/{id}/_channels`, `Service.ListAnchor`,
`internal/realtime/service.go:671`). Nothing answers "what waits on me across
all channels". rela has no principal mentions (`internal/mentions` finds entity
ids only), no read state and no unread counts.

Two consumers need this:

1. **The agent-runner demo** (Claude Agent SDK). Agents run as `system:` or
`integration:` job identities, or as users. An agent must list everything that
waits on it (mentions, replies, sessions where `waiting_on_me` is true), mark
items read, and long-poll for new items through MCP (`list_inbox` with `watch`).
2. **People in the SPA**: unread badges per channel, a sidebar badge, and an
inbox page.

The questions:

1. Where do read markers and pushed entries live on each backend, and how do
they follow rename, tombstone, purge, removal and edit?
2. What is a principal mention, how is it parsed, and who may mention whom?
3. How does the inbox gate many channels in one request without per-row
lookups, and without counts that leak hidden rows?
4. How are derived "waiting on you" entries found without scanning every
channel? Is a separate `awaits:` key still needed?
5. What is the live, MCP and SPA surface, and what changes with several
server processes?

## Context

### What exists

| Area | What exists | Where |
| --- | --- | --- |
| Channel store | `channels.Store`: gapless `seq` per channel, `rev`, CAS on lifecycle, actions and session, audit trail, `Rekey` on rename, tombstones, purge. `Channel` carries `LastSeq` and `Incarnation`. No per-reader state. | `internal/channels/store.go:26-283`, `channels.go:332-350` (Message), `417-433` (Channel), `439` (`CheckIncarnation`) |
| Backends | `filechannels`: a directory per channel with `meta.yaml` and JSONL segments, file lock; cross-channel queries (`ListByLifecycle`, `Keys`) walk every channel directory. `sqlitechannels`/`pgchannels` over shared `sqlchannels`: `channels` table with `session_state`, index `(type, state, session_next_due_at)`. | `filechannels/file.go:1-20`, `ops.go:1058`, `internal/sqlitedb/sqlitedb.go:677-702,749-750`, `pgstore/migrations/0026,0031-0033` |
| Message removal and edit | Removal erases content from storage; edit replaces props, keeps no history. | `channels.go:21-35` |
| Sessions, waiting | Per state: `status` and `waiting_for` (4/6). `waiting_for` may name only `from: entity.<prop>` or `creator: true` roles, never permission, attached or principal-type roles. `waiting_on_me` is computed per reader; the code already notes "an inbox computes the same answer from the same facts, batched per page". | `docs/realtime.md:620-689`, `internal/realtime/waiting.go:41-75,106-118` |
| Role match | A property role matches through `is_current_user`/`has_current_user` against `QueryIdentity.ID()`; a creator role through `sameAuthor` (User and OrgID). | `internal/realtime/authz.go:272-328,363-366` |
| Identity | `QueryIdentity` exists because `principal.User` differs per transport (the SPA rewrites it to the user entity id; CLI, MCP and scheduler keep the raw name). Job identities: `system:` and `integration:` (`ValidJobIdentity`), refused from the wire (`IsReserved`). | `internal/predicatefns/currentuser.go:78-118`, `internal/principal/principal.go:588,600,635,737` |
| Gated reads | `AnchorReader` reads one anchor per call (a one-element header batch). `RefResolver` resolves many ids in one batch. `acl.PrimeLocalRoles` batches the relation-conferred role walk for a page; `HoldsPermissionForEntity` is otherwise per entity. | `internal/realtime/access.go:51-92`, `internal/appbuild/realtime_service.go:285-330`, `internal/acl/localroleprime.go:56`, `resolver.go:263` |
| Entity mentions | `internal/mentions`: a code span whose whole text is a valid entity id. One parser for rendering (forward) and the backlink index (backward), capped at 1000 targets. pgstore stores targets on the row, written in the same statement as the body, so rename, delete and purge carry them for free. | `internal/mentions/mentions.go:1-12,37-75`, `pgstore/migrations/0024_mention_targets.sql:1-18` |
| Per-principal state | Piles: KV document per tier (`kvpiles`) or tenant tables (`pgpiles`); only the owner reads; foreign pushes are write-only, silent and bounded; system identities have no piles. Leases: same split, plus a `lease_counters.version` row that watchers poll across processes. | `internal/piles/piles.go:14-29`, `service.go:296-340`, `pgstore/migrations/0023_piles.sql`, `0025_leases.sql:47-56`, `internal/lease/wake.go` |
| Live delivery | `realtime.Local`: process-local subscriptions hold a set of changed channel keys (max 1024, then resync). The SPA live hub replays subscribed GETs as the subscriber and hashes the result; topics (`f:`, `a:`, `c:`) restrict a round to subscriptions a change can move, and a topic is learned from the route's own 200 reply, never from the client. | `internal/realtime/local.go:86-180`, `internal/dataentry/live.go:28-52`, `live_topics.go:11-58` |
| MCP watch | Replays read tools as the caller and compares digests. Watchable: `show_entity`, `list_entities`, `list_relations`, `read_channel`, `read_session`. Channel wake: in-process subscription plus a 2 s poll (5 s on remote MCP). Limits 32 watches per server, 4 per identity, 50 s per call. | `internal/mcp/tools_watch.go:28-72`, `docs/realtime.md:1670-1683,1727-1740` |
| Background work | `jobs.Queue`: external side effects (mail) go on it; ephemeral on fs and desktop, durable in the tenant schema on postgres. Scheduled mail already renders under the recipient's principal (child job per recipient). | `.claude/rules/jobs.md`, `internal/appbuild/scheduled_mail.go:19-42`, `internal/scheduler/foreach.go:15-26` |
| Desktop | `desktopnotify`: notifications and Dock badge from entity predicates in `desktop.yaml`, diffed against a seen-set in `state.KV`. | `internal/desktopnotify/doc.go` |
| Mockups | Storybook "Mockups/Realtime" already shows unread counts per channel and a sidebar "Needs you: 2 waiting for you" item. | main checkout, untracked: `frontend/packages/rela-components/src/fixtures/realtime.ts:731-772`, `RlRealtimeEntityChannels.vue:59` |

### Constraints that shape the design

- **Gate at read time.** An entry about a channel or entity the principal can
no longer read disappears without a trace (`CLAUDE.md`, row-level rule).
- **Aggregates gate before they fold.** Counts and `truncated` flags are
computed on the gated set; a pre-filter count is an existence oracle
(`CLAUDE.md`, gantt rule, `TestGantt_TruncatedIsPostFilter`). Results are per
principal and never cached across principals.
- **No per-row lookups on a list path**; pin cost with a budget test that is
equal at 10 and 50 rows (`.claude/rules/collection-reads.md`; existing pattern
`internal/dataentry/channels_budget_test.go:390`).
- **External side effects go on `jobs.Queue`**, never inline on a write path.
- **Runtime state does not use local files on postgres** (`storage.md`).
- **Channels stay outside the graph** (no entity type, audit, versioning or
search for channel data).

## Use cases

| # | Use case | Needs |
| --- | --- | --- |
| 1 | Agent waits for work: "anything for `integration:triage-bot`?" across all channels, acts, marks done, waits again | Derived waiting entries, mentions, mark read, `list_inbox` + `watch` |
| 2 | Human-in-the-loop: an agent session enters `input_required` waiting on the incident lead; the lead sees it in the sidebar badge and inbox | Derived waiting entries, live badge |
| 3 | Someone writes "`USR-7` can you check the log?" in a channel; USR-7 gets an inbox entry and, if configured, a mail | Mention parsing, pushed entry, mail job |
| 4 | Reply to my message in a thread | Pushed reply entry |
| 5 | "Which of my channels have new messages?" | Read markers, unread counts per channel |
| 6 | An entity becomes hidden from me (ACL change, delete) | Entries vanish; counts drop; no trace |
| 7 | Desktop badge shows "3 waiting" | Inbox count for the desktop user |

Use cases 1 and 2 drive the demo. Both need only derived entries plus an MCP
tool, which needs no new storage.

## Prior art, only what informs the design

| System | Lesson |
| --- | --- |
| **Slack** | One read marker per channel (`last_read`). Your own post moves your marker. Unread counts only for channels you are a member of; mention badges counted separately. |
| **Zulip** | Exact unread counts per stream and topic from per-message flags. Mention syntax embeds the user id (`@**Name\|id**`) so renames do not break it. Wildcard mentions (`@all`) are restricted by policy in large streams. When you mention someone who is not subscribed, Zulip warns the sender; in rela that warning would leak the target's access, so it must not exist. |
| **Matrix** | `m.fully_read` is a private per-room marker, separate from public read receipts. "Intentional mentions" (`m.mentions` with explicit user ids) replaced body parsing because display names in the body were spoofable and ambiguous. Unread and highlight counts are computed server-side per room. |
| **GitHub notifications** | One inbox across repositories. Each item has a `reason` (`mention`, `review_requested`, `assign`, `author`, `comment`). States: unread, read, done. Items from repositories you lose access to are hidden. The API is built for polling: `Last-Modified`/`If-Modified-Since` and `X-Poll-Interval`. |
| **Linear** | Separates the pushed **Inbox** (mentions, comments, assignments) from the derived **My issues** view (a query over assignee). Inbox items can be read, snoozed and deleted. |

What rela takes: one private marker per channel (Slack, Matrix); a mention names
a stable id, not a display name (Zulip, Matrix); a `reason` per entry (GitHub);
pushed and derived entries kept apart (Linear); a cheap "has anything changed"
check for pollers (GitHub). What rela leaves out for now: public read receipts,
wildcard mentions, snooze.

## Options

### (a) Where read markers and pushed entries live

| Option | Description | Assessment |
| --- | --- | --- |
| **a1 Inside `channels.Store`** | New store methods, pinned by `channelstest`. SQL: `channel_reads(channel_id FK, principal, seq, at)` and `channel_mentions(channel_id FK, seq, target, reason)` with an index on `(target, channel_id, seq)`, written in the same transaction as `Append`/`Edit`/`Remove`. fs: `reads.json` and `mentions.jsonl` inside each channel directory. | Rename (`Rekey` keeps `channel_id`; fs moves the directory), tombstone, purge and delete carry the data with no extra code, the argument migration 0024 makes for entity mentions. Removal and edit update mention rows atomically, so a removed message cannot keep pointing at a person. Travels with `rela.db` on sqlite. Cost: the fs backend has no cross-channel index, so "all my markers" walks channel directories, as `ListByLifecycle` already does. **Recommended.** |
| a2 Separate `inbox` package like piles and leases | `kvinbox` (one KV document per principal under a file lock) and `pginbox` (tenant tables). | Fits the "per-person state outside the graph" pattern and gives a cheap per-principal read on fs. But it must re-implement rename, tombstone, purge, removal and edit through hooks, and an entry write after `Append` is not atomic. Entries for an old incarnation of a reused id need an explicit check. More code, more ways to drift. |
| a3 Markers in `state.KV`, mentions in the store | Split by kind. | Two lifecycles to keep in step; no gain over a1. |

Cardinality and retention under a1:

- Markers: one row per (channel, principal) the principal has opened or
posted in. Bounded by activity. Deleted with the channel.
- Mention rows: at most `MaxMentionsPerMessage` (proposed 10) per message.
Removed with the message, rewritten on edit, purged with the channel or by
message retention.
- Per principal: the inbox reads at most the newest N mention rows (proposed
500) per request. Older rows stay in the store but drop out of the inbox. No
eviction write is needed, so a flood cannot erase anything.
- Incarnation: a row joins its channel, so an entry can never point at a new
entity's channel that reuses an old id (`CheckIncarnation`).

### (b) Mention syntax and resolution

| Option | Description | Assessment |
| --- | --- | --- |
| **b1 Code span with a principal key** | `` `USR-7` `` (an entity id whose type is `user_entity_type `) or `` `integration:triage-bot` `` (a job identity in the `ValidJobIdentity ` grammar, not one of rela's own). Extend `internal/mentions ` with one function that returns principal targets, so rendering and indexing share one parser. The composer offers an `@ ` picker that inserts the code span. | No new markdown syntax, no clash with e-mail addresses, stable under display-name changes, natural for agents. Needs no lookup at post time. A person mention also renders as an entity link through the existing `RefResolver ` batch, gated for each reader. **Recommended.** |
| b2 `@handle ` text | Resolve `@name ` against a configured person property. | Familiar, but needs a lookup per handle at post time through the poster's gate, is ambiguous, and breaks on rename. Telling the poster "unknown handle" is an existence oracle for hidden persons. |
| b3 Structured `mentions ` field on the post (Matrix style) | The client sends `mentions: [ids] ` beside the props. | Explicit, no parsing. But the body and the list can disagree, every client (CLI, MCP, scripts) must fill it, and the API grows. |

Typed refs: a message property `type: ref ` whose `to: ` is the
`user_entity_type ` (for example `handoff.to `) is an intentional mention too.
Proposed: such a property counts as a mention unless the property says `notify:
false `.

Replies: a reply creates a `reply ` entry for the parent message's author,
unless the reply's author is the same principal (`sameAuthor `).

Deployments without `user_entity_type `: the principal key is a raw login such
as `jeroen `, which is not a valid entity id. b1 then needs a second form, for
example `` `@jeroen` `` inside a code span. This is an open question.

Edits: an edit recomputes the targets. New targets get entries; dropped targets
lose them (Zulip behaviour). A removal drops all of them.

Untrusted content: targets are data extracted by rela's parser, capped per
message, and never interpreted. Rendering stays as today: the SPA shows a person
mention through the gated ref view, and a hidden person renders restricted. MCP
answers keep the `UntrustedNotice ` (`internal/realtime/usertext.go:11 `).

### (c) Who may mention whom, and when a target cannot read

| Option | Description | Assessment |
| --- | --- | --- |
| **c1 Anyone who may post may mention anyone; store, then gate at read** | The store keeps the row. The target sees it only while the gate admits them. The poster gets the same answer whether the target exists, can read, or not. | No oracle, no evaluation of another principal's ACL on the post path. Access changes are handled by the read gate anyway. Abuse is bounded by the per-message cap, the message type's `rate ` and the 500-row inbox window. Risk: a poster in a channel the target cannot read can push hidden rows that crowd the target's window. Mitigation: the window counts only rows that pass the gate, read in pages until full or a scan cap is hit (see (e)). **Recommended.** |
| c2 Drop at post time if the target cannot read | Evaluate the target's read access as the target. | Needs an `acl.Request ` for another principal on the post path, a per-target walk, and still needs the read gate later. Gains little. |
| c3 Restrict mentions by config | `mentions: {by: [roles]} ` per channel type, or a permission needed to mention job identities. | Useful against prompt injection of agents: anyone who can post can make an agent read text. The phase 3 rule says agents are ordinary principals limited through `acl.yaml `. Keep as a later option; ask the user. |

### (d) Unread counts

| Option | Description | Assessment |
| --- | --- | --- |
| d1 `LastSeq - marker ` | No message read. | Free, but counts your own messages (unless your post moves your marker), system notices and removed messages. |
| **d2 Exact count of other principals' live messages after the marker, capped at 100** | SQL: one grouped `COUNT ` over `channel_messages ` for the page's channels. fs: read the log tail after the marker, stop at 100. | Matches what the reader sees when opening the channel. Bounded cost. **Recommended**, with "99+" display. |

Which channels show a count: only channels with a marker (the principal has
opened or posted in them), the Slack membership idea. A channel never opened
shows no count, so the inbox does not list every readable channel as unread.
Being mentioned in a channel does not create a marker; the entry is enough.

Mark read: `marker = max(marker, seq) `. It never moves back, except by an
explicit "mark unread" (later). Posting moves the poster's marker to their own
message. A mention or reply entry counts as read when the marker passes its `seq
`. No per-entry read flag is needed. A "done" state per entry (GitHub) is left
for later.

### (e) Gating the inbox in one request

The inbox read for one principal, as one operation (`begin ` captures one
config snapshot and one `acl.Request `):

1. Collect candidate channels: the principal's markers, its newest mention
rows, and the derived waiting channels (see (f)). One store call per source.
2. Load the channel heads in one batch (new `channels.Store.Heads(keys) ` or
part of the source queries).
3. Read all anchors in one gated header batch (a batched `AnchorReader `, the
same `ResolveHeadersErr ` call the single reader already makes, with the role
properties of the involved channel types).
4. Prime relation-conferred roles for the anchor ids with
`acl.PrimeLocalRoles `, so a `permission: ` read role does not walk per row.
5. Apply `roleAdmits ` for each channel's `read: ` role, the incarnation check,
the open/closed filter and the removed-message filter.
6. Only then count, sort, page and set `truncated `.

| Option | Description | Assessment |
| --- | --- | --- |
| **e1 Gate the whole candidate set, then fold** | Steps 1 to 6 with fixed caps on each source (markers 500, mention rows 500, waiting 500). | Every count and flag is post-gate. Cost is one call per source plus one header batch, independent of rows. A budget test pins it at 10 and 50 channels with half the anchors hidden. **Recommended.** |
| e2 Count in SQL, gate only the page | Faster for large inboxes. | A pre-gate total is the oracle the gantt rule forbids. Rejected. |

Existence-oracle notes:

- A source cap must not leak. If the mention source hits its cap of 500 rows
before the gate, `truncated ` may say only "older entries not shown", and it
must be set the same way whether or not hidden rows exist. The simplest rule:
the inbox always shows a fixed window ("last 500 mentions") and never claims
completeness. The count shown is the count in the gated window.
- Hidden rows inside the window shrink the visible count but reveal nothing:
the reader cannot tell a hidden mention from no mention.
- The mark-read route answers 404 for a channel the caller cannot read, exactly
as the channel GET does, and stores nothing then.

### (f) Derived "waiting on you" entries

A session waits on a principal when its state is `input_required `, the channel
is stored and open, the config is not degraded, and the principal holds one of
the `waiting_for ` roles (`waiting.go:44-75 `). `waiting_for ` roles are only
`from: entity.<prop> ` or `creator: true ` (`docs/realtime.md:645-650 `),
which makes a targeted query possible.

| Option | Description | Assessment |
| --- | --- | --- |
| f1 Channel-first | List channels by `(type, state=open, session_state IN waiting states) ` (new index), then gate their anchors in one batch and test the role. | One channel query plus one batch, simple. But the work grows with all waiting sessions in the project, and any cap is applied before the gate, so a `truncated ` flag would be an oracle. |
| **f2 Anchor-first through the gated graph query** | For each (attach_to type, role property) pair that some waiting state uses, run one gated header query "entities of type T where `prop ` names me", pushed down as `is_current_user `/`has_current_user ` (the next-action pushdown, `predicatefns.CurrentUserPrefilterSpec `). Then one `channels.Store ` call: channels on those anchors with a waiting session state. Creator roles: one call for channels created by me in a waiting state (needs a `created_by_user ` column; today `created_by ` is JSON text). | The graph query is the row gate, so caps apply after it. Cost is one query per configured pair (bounded by config, not data) plus two channel calls. Uses the same redacted props as `waitingView `, so a `visible: `-hidden property matches nobody in both. **Recommended.** |
| f3 Materialize waiting entries at transition time | Write entries for raw holders when a session enters a waiting state. | Goes stale when the holder property is edited outside the channel (a handoff in the entity form), and needs an allow-all read of holders. Rejected. |

**Phase 3 collapses into phase 2.** 4/6 already ships the per-state `waiting_for
`, `status: input_required ` and the per-reader `waiting_on_me `. That is what
`awaits: ` was meant to express. No new key is needed. The inbox reuses
`waitingView ` with facts built per page. An optional per-type `inbox: false `
could keep noisy session types out; it is not needed for v1.

A derived entry has no read state. It leaves the inbox when the session moves
on, the role moves to someone else, or access is lost.

### (g) Live delivery per principal

| Option | Description | Assessment |
| --- | --- | --- |
| **g1 A live shape for `/api/v1/_inbox ` with an inbox topic** | The SPA subscribes to `/api/v1/_inbox/summary ` (badge) and `/api/v1/_inbox ` (page). The route records topic `i:<principal key> ` from its own 200 reply. Hints: a mention or reply row marks `i:<target> `; a marker write marks `i:<self> `; a session move marks `i:sessions `, which every inbox subscription listens to. Graph writes already start full rounds, which cover handoffs by entity edit. | Reuses the replay-and-hash model: a frame reveals no more than polling the GET. Hints are not data; the replay re-gates. Needs a second kind of change in `realtime.Local ` beside channel keys. **Recommended.** |
| g2 A push stream of entries | Server sends entry payloads. | A second delivery path with its own ACL story. Rejected. |

Process-local caveat: hints and subscriptions live in one process, as for
channels today (`docs/realtime.md:1660-1683 `). A mention posted through stdio
`rela mcp ` or another rela-server process reaches open browsers at the next
full round or refetch, not at once.

### (h) MCP surface

Proposed tools:

- `list_inbox `: args `kinds ` (subset of `waiting `, `mention `, `reply `,
`unread `; default all), `unread_only ` (default true), `limit `. Result:
`{notice, counts: {waiting, mentions, replies, unread_channels}, entries:
[{kind, reason, channel: {anchor, type, name, title, state}, seq, at, author,
session?: {state, status, waiting_for, transitions}}], unread_channels:
[{channel, unread, marker, last_seq}]} `. Entries carry pointers, not message
content: the agent reads content with `read_channel ` (`after ` = marker).
This keeps the answer small and the injection surface in one tool. A short
`preview ` is an option for the SPA, not for MCP.
- `mark_read `: args channel address and `seq ` (default: the channel's last
seq). Answers the new marker.
- `list_inbox ` joins `watchOrder ` (`tools_watch.go:72 `). Its wake sources: the
channel subscription (any channel change or inbox hint), store events, and the
existing 2 s / 5 s poll for other processes. The answer must not depend on the
clock (no "due in" fields), so the digest moves only when entries move, as
`read_session ` already does.

Agent loop: `list_inbox `, act on each entry (`read_session ` / `read_channel
`, post), `mark_read `, then `watch ` on `list_inbox ` with the cursor.

Job identities: the inbox key of `integration:triage-bot ` is that raw string.
Its inbox works without a user entity. The startup warning for identities
without an `acl.yaml ` assignment already exists and applies.

### (i) SPA surface

- Unread count per channel in the entity channel panel (`ListAnchor ` gains
`unread ` from one marker batch per anchor; extend
`TestChannelsBudget_List10vs50 `).
- Opening a channel moves the marker to the last message shown.
- Sidebar "Inbox" item with a badge from `/_inbox/summary ` (the mockup's
"Needs you").
- Inbox page with three sections: "Waiting on you", "Mentions and replies",
"Unread channels". Each row links to the channel at the message.
- Composer `@ ` picker for persons and job identities. The list of job
identities comes from `acl.yaml ` assignments; configuration is not a secret.

### (j) Notifications

| Option | Description | Assessment |
| --- | --- | --- |
| **j1 Mail digest job, rendered as the recipient** | A mention or reply enqueues `inbox/notify ` on `jobs.Queue ` with the recipient key and an idempotency key per recipient (one pending digest at a time). The job runs under the recipient's principal, as scheduled mail does, reads the gated inbox, and renders through `mailrender `. An empty gated result sends nothing. Opt-in per user (user settings). | No content reaches someone who cannot read it, because the read is the recipient's own gated read at send time. Follows the jobs and mail rules. **Recommended**, as a later slice. |
| j2 Content-free mail ("you have new items") | No render as recipient. | Safe and simple, but even an entity title would leak, so the mail says almost nothing. A valid smaller first step. |
| j3 Inline send on post | | Violates the jobs rule. Rejected. |

Desktop: the desktop app is one user in one process. The shell can read the
inbox summary for the Dock badge and diff entries for native notifications, as
`desktopnotify ` does for entities. No queue is needed for a local UI
notification.

### (k) Several server processes (phase 5)

- Storage: correct as is on postgres, because markers and mention rows are in
the tenant schema and every process reads them.
- Live frames: process-local, like channel frames today. The fix is the same
one channels need: a `pg_notify ` poke for channel changes on the existing
`rela_changed ` channel, turned into `Local.mark ` calls on each process.
- MCP watch: already polls every 2 s / 5 s. Each poll replays `list_inbox `.
An `inbox_version ` per principal (like `lease_counters.version `) could skip
a replay when nothing pushed changed, but derived entries move without it, so it
only helps for pushed entries. Not needed for v1.
- sqlite: one process, no issue.

## Recommendation

- Storage **a1**: markers and mention rows in `channels.Store `, all three
backends, conformance in `channelstest `.
- Mentions **b1**: code spans with a person entity id or a job identity, plus
typed refs to the user entity type; replies notify the parent's author; edits
recompute targets.
- Policy **c1**: anyone who may post may mention; store, then gate at read;
no feedback to the poster about the target.
- Counts **d2**: exact, capped at 100, only for channels with a marker.
- Gate **e1**: gate the whole candidate set in one batch, then count and page;
the inbox is a fixed window, never a completeness claim.
- Waiting **f2**: anchor-first through the gated graph query; no `awaits: ` key.
- Live **g1**, MCP **(h)**, SPA **(i)**, notifications **j1** later.

### Slices

Each slice ends shippable and documented in `docs/realtime.md `. The order puts
the demo's needs first; the ticket's suggested order (markers first) is the
alternative.

| # | Slice | Contents | Effort |
| --- | --- | --- | --- |
| 1 | Inbox read with waiting entries | `Service.Inbox ` (derived waiting only, f2), batched anchor reader, `GET /api/v1/_inbox `, MCP `list_inbox ` in `watchOrder `, budget test, docs. No new storage except the creator column. | M |
| 2 | Read markers and unread counts | Store methods (`SetRead `, `Reads `, grouped unread count) in three backends plus conformance; own post moves the marker; `unread ` in channel list and head; `PUT .../_channels/{type}[/{name}]/_read `; MCP `mark_read `; `unread_channels ` in the inbox; SPA counts in the channel panel. | L |
| 3 | Mentions and replies | Principal targets in `internal/mentions `; mention rows written in `Append `/`Edit `/`Remove `; caps; `mention `/`reply ` entries in the inbox; rendering of person mentions; composer `@ ` picker. | L |
| 4 | SPA inbox and live | `/_inbox/summary `, live shape and `i: ` topics, hints in `realtime.Local `, sidebar badge, inbox page, Storybook from the mockup. | M |
| 5 | Notifications | `inbox/notify ` mail digest as the recipient, user opt-in setting, desktop badge and notifications. | M |
| 6 | Multi-process live (phase 5) | `pg_notify ` poke for channel changes into `Local.mark `, shared with channel live frames. | M |

Risks:

- The fs backend has no cross-channel index. Markers and mentions walk
channel directories, like `ListByLifecycle `. Fine at fs and desktop scale, but
it needs a budget test on store calls and a stated scale limit.
- f2 depends on the current-user pushdown for every role property type used
by `waiting_for `. A property type the pushdown cannot lower falls back to the
Go filter path; it must stay one query, not per row.
- Mentions make any poster able to put text in front of an agent. The agent
must treat it as data; the operator limits what the agent may write. Option c3
is the stronger control if the user wants it.

## Open questions for the user

1. **Slice order.** (A) waiting entries and `list_inbox ` first, for the demo;
(B) markers and unread counts first, as the ticket suggests. Recommendation: A.
2. **Storage.** (A) markers and mention rows inside `channels.Store `; (B) a
separate inbox service like piles and leases. Recommendation: A.
3. **Mention syntax.** (A) code span with an entity id or job identity, plus an
`@ ` picker; (B) `@handle ` text; (C) a structured `mentions ` field. And for
deployments without `user_entity_type `: allow `` `@login` `` in a code span,
or no user mentions there? Recommendation: A, with `` `@login` ``.
4. **Who may mention.** (A) anyone who may post, gated at read; (B) a role or
permission is needed to mention job identities (agents). Recommendation: A now,
B as a later option.
5. **Unread scope and counting.** Counts only for channels the principal
opened or posted in, exact and capped at 100? Or also for channels where the
principal holds a role? Recommendation: opened or posted only.
6. **Typed refs.** Does a `ref ` property to the user entity type count as a
mention by default (with `notify: false ` to opt out)? Recommendation: yes.
7. **`awaits: ` key.** Drop it, since `waiting_for ` covers it? Recommendation:
drop.
8. **Mail.** (A) digest rendered as the recipient, opt-in per user; (B)
content-free mail; (C) no mail in this ticket. Recommendation: A in a later
slice.

## Decisions (2026-10-27)

The user took all recommendations: slice 1 (inbox read with waiting entries)
first; read markers and mention rows inside `channels.Store `; mentions as a
code-span id plus an `@ ` picker; anyone who may post may mention, gated at
read time; unread counts only for channels the principal opened or posted in,
capped at 100; mail as an opt-in digest rendered as the recipient, in a later
slice; no `awaits: ` key.
