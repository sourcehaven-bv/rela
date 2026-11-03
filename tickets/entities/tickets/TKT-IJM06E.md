---
id: TKT-IJM06E
type: ticket
title: 'Sessions 4/7: counting guards'
kind: enhancement
priority: low
effort: s
started: "2026-10-27"
status: backlog
---

## Description

Counting guards for quorum votes: a session counts distinct authors of a message
type and a transition guard reads the count (research use case 3, approval with
quorum and deadline).

Deferred on 2026-10-27: nice to have, but tricky to get right (atomic tally with
the post, vote edits and withdrawals, voter write grants). The draft plan below
was never design-reviewed.

The menu filter `session_state:` moved to TKT-TP0SXK.

Design: RES-O8I8S7. Part of FEAT-GWF8YH phase 4.

## Draft plan (2026-10-27, not reviewed)

### Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Research use case 3 ("approval with quorum and deadline") needs a
transition that fires when enough distinct people voted. Today a guard reads
only the posted message and `session.state`, `generation`, `entered_at` and
`reminders` (`internal/realtimecfg/session.go:625-651`). Nothing counts what
other people posted. Decision 1 of RES-O8I8S7 chose a `count()` facility for
guards; slice 7 in section (h) adds it with a menu filter `session_state:` for
data-entry `channels:` entries.

Two facts from the code shape the design:

1. The decider runs under the store's lock and must be pure: "no I/O and
no locks" (`internal/channels/session.go:272-275`). A guard that scans the
message log at evaluation time breaks that rule.
2. A count read before the append races. Two votes that arrive together
both see two earlier votes, and neither sees the third. The quorum is reached
and nothing moves. So the tally must change in the same append that stores the
vote, under the lock.

The research already names the place for it: "a small counters map" on the
channel row (option b1).

The menu half has a gap. `data-entry.yaml` has no `channels:` navigation entry
today. The `channels:` block is message presentation only
(`internal/dataentryconfig/channels.go:11-20`). `NavigationEntry` has no channel
kind (`internal/dataentryconfig/config.go:1541-1600`). There is no endpoint that
lists channels across anchors: `ListAnchor` is per entity
(`internal/realtime/service.go:671`), and so is MCP `list_channels`. So "a
`session_state:` filter on `channels:` entries" means building the entry kind, a
store query and a gated cross-anchor list first. See open question Q6.

Terms used below:
- **Count**: one `session.counts.<name>` entry. Its value is a number of
distinct authors.
- **Span**: the stretch of time in which a count is active. It starts when
the session enters one of the count's `in:` states from a state outside them, or
when the channel is created in such a state. It ends when the session moves to a
state outside `in:`.
- **Ballot types**: the count's `type` plus its `cleared_by` types.
- **Author key**: `(Author.OrgID, Author.User)`, the identity `sameAuthor`
compares (`internal/realtime/authz.go:363`). `Tool` and `PrincipalType` are not
part of it.
- **Counted**: an author is counted when their latest message of a ballot
type in the span is of type `type` and passes the count's `when:`.
- **Tally**: the stored record of one count: its span start, a fingerprint
of its definition, and per counted author the seq of the counted message.

**Scope:**
- In (part A, counting guards):
  - `session.counts` config in `internal/realtimecfg`: `label`, `type`,
`when`, `cleared_by`, `in`. Load checks and refused keys.
  - Guards read `session.counts.<name>` as an integer, declared only for
counts active in the transition's state.
  - A helpful refusal for an inline `count(...)` in a guard.
  - The tally on the channel row in all three channel backends:
`channels.Session.Counts`, `SessionSchedule.Counts`, the pg migration 0034, the
sqlite rung 26 to 27, `filechannels` meta, and conformance tests in
`channelstest`.
  - Tally update in the post's session step, carried across moves inside
the span, by the same schedule path as reminder records.
  - Output pre-check for a count-guarded transition: it runs whether or
not the guard holds at pre-check time.
  - Author edit and delete of a counted message refused with 409
`counted_message`.
  - `SessionView.counts`, the schema descriptor, apiwire, OpenAPI, TS
mirror, CLI rendering, SPA stepper, MCP `read_session` (through `SessionView`).
  - Docs, `testdata/valid.yaml` and the docs example.
- In (part B, menu filter; see Q6, recommended as its own ticket):
  - A navigation entry `channels: <channel type>` inside a group, with an
optional `session_state: [states]` filter.
  - `GET /api/v1/_channels?type=&session_state=` over a new store query
`ListBySessionState`, row-gated in batches.
  - SPA sidebar links.
- Out:
  - `waiting_for_me: true` on the menu entry (research (g) says "later
slice"), and the inbox.
  - Counting messages instead of authors, sums or weights. A count is
always distinct authors.
  - Counts that read the entity, or a `roles:` filter on a count. Limit
voters with the message type's `post:`.
  - Counts in timers or in output templates (`{{session.counts.x}}`).
  - Backfill: messages posted before a span started, or before the count
was declared, are never counted.
  - Un-counting on a moderator's removal. It is documented, as removal of
a message that moved the session is today.
  - General session variables (research question 6, option B).

**Example (the shape this slice accepts; also the docs example and a new
`testdata/valid.yaml` block):**

```yaml
channel_types:
  risk-approval:
    attach_to: [risico]
    cardinality: many
    roles:
      creator:  { creator: true, label: the requester }
      reviewer: { permission: risk.review, label: a reviewer }
    message_types:
      vote:
        properties:
          choice: { type: enum, values: [approve, reject], required: true }
          note:   { type: string }
        post: [reviewer]
      withdraw: { properties: {}, post: [reviewer] }
      cancel:   { properties: {}, post: [creator] }
    session:
      initial: voting
      run_as: job:risk-approvals
      counts:
        approvals:
          label: Approvals
          type: vote
          when: "message.choice == 'approve'"
          cleared_by: [withdraw]
          in: [voting]
        rejections:
          label: Rejections
          type: vote
          when: "message.choice == 'reject'"
          cleared_by: [withdraw]
          in: [voting]
      states:
        voting:
          label: Voting
          on:
            vote:   { to: approved, when: "session.counts.approvals >= 3" }
            cancel: { to: cancelled }
          after:
            deadline: { period: 72h, to: expired, label: Vote deadline }
        approved:
          label: Approved
          after:
            record:
              period: 1m
              to: recorded
              outputs:
                - set: { properties: { status: approved } }
        recorded:  { label: Recorded, final: true }
        expired:   { label: Expired, final: true, status: failed }
        cancelled: { label: Cancelled, final: true, status: cancelled }
```

A reviewer who votes `approve` and later `reject` stops counting for `approvals`
and starts counting for `rejections`. `withdraw` clears them from both. The
quorum move writes nothing itself. The `record` timer writes the decision a
minute later as `run_as`, so voters need no field write on `status` (see
approach 6). One message type fires one transition per state, so a reject quorum
needs its own message type; the docs say this.

**Acceptance Criteria:**

1. `session.counts` accepts at most 4 entries
(`realtimelimits.MaxSessionCounts`). Names match the state name grammar. More
entries, or a bad name, are refused at the entry key. Test: rows in
`TestSessionCounts_Parse` (`internal/realtimecfg`).
2. `type` is required. It must be a declared message type of the channel
type, persisted to the log (not ephemeral), and not the reserved `session`
notice type. Otherwise it is refused at `...counts.<n>.type` with a reason.
Test: rows in `TestSessionCounts_Parse`.
3. `cleared_by` entries must be declared log message types, different from
`type`, with no duplicates. An empty list is refused ("drop cleared_by"). Test:
rows in `TestSessionCounts_Parse`.
4. `in` is required, and names declared states that are not final, each
once. The reasons match the reminder `in:` checks
(`internal/realtimecfg/reminders.go:147-164`). Test: rows in
`TestSessionCounts_Parse`.
5. `when` compiles as a boolean over `message` typed for `type`, with the
standard library, and no `session` or `entity`. A `when` that reads `entity` is
refused with "a count's when reads only the message". A `when` that does not
compile is refused at `...counts.<n>.when`. Test: rows in
`TestSessionCounts_When`.
6. A guard reads `session.counts['<name>']` (bracket form for a hyphen)
as an integer. Only counts whose `in:` lists the transition's state are
declared. A guard that names a count not active in its state is refused at
`...on.<type>.when` with "count approvals does not run in state X (in: voting),
so it would always read 0". An undeclared name is refused as unknown. Test:
`TestSessionGuard_Counts` table.
7. A guard that calls `count(...)` is refused with "declare the count
under session.counts and read session.counts.<name>". Test: a row in
`TestSessionGuard_Counts`.
8. Refused keys answer with reasons: `session.counts.*.distinct` ("a count
is always of distinct authors"), `session.counts.*.roles` ("limit voters with
the message type's post"), `session.counts.*.since` ("a count runs over its
span; see in"), `session.states.*.counts` ("declare counts under
session.counts"). Test: rows in `TestParse_SessionInvalid`.
9. Counting rule. A post updates a count only when the channel is open,
the session's current state is in the count's `in:`, the message is of a ballot
type, and the author is not a system author (trigger posts, reminders, notices:
anything with `TriggeredBy` set or `author.system`). A message of `type` that
passes `when` sets the author's entry to its seq. A message of `type` that fails
`when`, or of a `cleared_by` type, removes the author's entry. The count is the
number of entries. Test: `TestTally_Rules` table in `internal/realtime` over the
pure tally function: first vote, second vote by the same author (one entry, new
seq), change from approve to reject, withdraw, a trigger post of `vote`
(ignored), a reminder post (ignored), a post while the channel is closed
(ignored), a post in a state outside `in:` (ignored).
10. The guard sees the tally after the current message. With two earlier
approvals, the third distinct approval moves `voting` to `approved` in the same
append. A second approval by an earlier voter does not. Test:
`TestSession_QuorumMoves` (memstore service harness).
11. Concurrency: three approvals posted at once by three authors, after
two earlier ones, move the session exactly once, and every vote is in the tally.
Run on every backend through the conformance harness, and under `-race` in the
service. Test: `TestSession_QuorumConcurrent` in `internal/realtime` and
`RunSessionCounts` in `channelstest`.
12. Span rule. The tally is carried on a move between two states that are
both in the count's `in:`, including a re-entry. It is dropped on a move to a
state outside `in:`, and a later return starts empty. A timer move follows the
same rule. A close and reopen keeps the tally. Test: `TestTally_Span` table
(move inside, re-entry, leave and return, timer move, close and reopen).
13. Definition change. A reload that changes a count's `type`, `when` or
`cleared_by` restarts that count: the stored tally's fingerprint no longer
matches, so it reads 0 and the next ballot post starts a new tally. A reload
that changes only `label` or `in:` keeps it. A reload that removes the count
drops its tally at the next write. Test: `TestTally_Reload`.
14. Cap: a tally holds at most 64 authors
(`channels.MaxCountAuthors`). A post by a 65th new author is stored and not
counted, existing authors still update, the view shows `capped: true`, and rela
logs a warning once per count per loaded configuration. Test: `TestTally_Cap`.
15. Store contract: `SessionSchedule.Counts` nil keeps the stored tallies,
empty stores none. `ApplySession` clears them on a move, and the schedule puts
back the carried ones. A tally over the cap, with an invalid name, a bad author
key or a seq below 1 is `ErrInvalidMessage` on write and a damaged row on read.
More than `MaxSessionCounts` records is refused. Test: `RunSessionCounts` in
`internal/channels/channelstest`, run by `filechannels`, `sqlitechannels` and
`pgchannels`.
16. Migrations: pg `0034_channel_session_counts.sql` adds `session_counts
TEXT NOT NULL DEFAULT ''`. sqlite `schemaVersion` goes to 27 with a rung that
adds the column, and `channelsDDL` declares it. A row written before reads no
tallies. Test: the existing migration tests for both backends gain the column;
`TestMigrate_ChannelSessionCounts` in `internal/sqlitedb`.
17. Output pre-check. For a transition whose guard reads `session.counts`
and which has outputs, `precheckOutputs` checks the outputs whenever the poster
passes the transition's poster rule in the current state, even when the guard
does not hold at pre-check time. It sets the gate to that state. So a voter
without the write grant is refused `403 session_output_denied` before anything
is stored, and a concurrent quorum vote is never blocked by the gate. A guard
that does not read counts keeps today's behaviour. Test:
`TestPrecheck_CountGuard` table (granted voter below quorum: accepted and
counted; ungranted voter below quorum: 403, nothing stored; concurrent quorum
vote with grant: moves).
18. Load warning: a count-guarded transition with outputs, where
`acl.AnyRoleGrants` finds no role that grants the writes, loads with one warning
at the output key: "every voter needs this grant, because any vote may reach the
quorum; or move the outputs to a timer that writes as run_as". Test:
`TestSessionCounts_OutputWarning`.
19. Author edit and delete of a counted message (its seq is in the current
tally under its author) answer 409 `counted_message`, "this vote is counted;
post a new vote, or a cleared_by message, to change it". A message of a ballot
type that is no longer counted (superseded, or the span ended) can be edited and
deleted as before. A moderator may remove a counted message; the tally keeps the
author. The check uses the head the operation already read. Test:
`TestOps_CountedMessage` table, with a read count that shows no extra store
read.
20. View: `SessionView.counts` lists the counts active in the current
state, sorted by name, with `name`, `label`, `count` and `capped`. It is empty
for a channel that is not stored, a final state and a stranded state. It is the
same for every reader. Test: `TestSessionView_Counts`.
21. Budget: `ListAnchor` over 20 session channels with counts makes the
same store reads as without counts. A post with two counts makes one `Append`
and no other store call beyond today's. Test: `TestListAnchor_CountsBudget` and
`TestPost_CountsBudget` with a counting store.
22. Descriptor: `SessionDescription.counts` gives `name`, `label`, `type`,
`cleared_by` and `in` per count. `when` is not served as code; the descriptor
says `filtered: true` when it is set. Test: `TestDescribe_SessionCounts` and the
OpenAPI schema test. (Config is not secret; this is about what the SPA needs,
not concealment.)
23. Wire and clients: `ChannelSession.counts` and the descriptor fields in
apiwire, OpenAPI and `frontend/src/types/channels.ts`. The CLI prints `approvals
2` after the state, `approvals 64+` when capped. The SPA stepper shows
"Approvals: 2" under the current state. MCP `read_session` carries the counts,
and a `watch` on it wakes on a vote. Test: `internal/openapi/channels_test.go`,
`wireFields.test.ts`, rows in `channel_render_test.go`, `RlChannelSession` unit
test and story `Quorum`, a row in `TestWatch_ReadSession`.
24. End to end: over memstore with the real `acl.Policy`, three reviewers
approve and the session moves to `approved`. One reviewer changes to reject
before the third approval, so the quorum needs a fourth. The `record` timer then
writes `status: approved` as `job:risk-approvals`, and the audit record names
it. Test: `TestSession_QuorumApproval` in `internal/appbuild`.
25. Docs and testdata: the example in `GUIDE-realtime.md` is the
`risk-approval` block of `testdata/valid.yaml` (`TestDocsExample_Realtime` gains
it). `TestParse_Golden` parses `valid.yaml` with no problems and no warnings.
`just docs` regenerates `docs/realtime.md` with no diff.

Part B (menu filter), only if Q6 keeps it in this slice:

26. `data-entry.yaml` accepts, inside a group, `channels: <channel type>`
with optional `session_state: [states]`, `state:` (default `open`), `limit:` and
`icon:`, and no `label:`. Load refuses an unknown channel type, `session_state`
on a type without a session, an unknown state, and `session_state` without
`channels`. Test: `TestValidateNavChannels` in `internal/dataentryconfig`.
27. `GET /api/v1/_channels?type=T&session_state=a,b&state=open&limit=N`
returns the channels the caller may read, newest activity first, each with
anchor id, type and title, channel name and title, and the session view. A
filter on the initial state also matches channels that never moved. Test:
`TestChannelsList` in `internal/dataentry`.
28. Row gate before the page: a channel whose anchor the caller may not
read, or whose read role the caller fails, is absent. The response for a caller
who may read none is an empty list, the same as for a type with no channels.
`next` is computed after the gate. Test: `TestChannelsList_ACL` and
`TestChannelsList_NextIsPostFilter`.
29. Budget: the endpoint makes the same number of store and ACL calls at
10 and 50 channels: one channel query per page, one batched anchor read, and one
batched permission check per permission read role. Test:
`TestChannelsList_Budget` with `storetest.Counting` and a counting access.
30. SPA: the sidebar group renders one link per channel ("anchor title:
channel title, state label"), each opening the entity page on that channel. It
refetches on the `refresh` live event. Test: unit test of the nav item and a
sidebar story.

### Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; the slice reuses the session step, reminder records and the predicate engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-o8i8s7.md`). Decision 1: flat
states, session timers with `in:`, and a `count()` guard. Option a2: "a small
`count:` facility for guards (distinct authors of a message type, optionally
filtered)". Option b1: "a small counters map" on the channel row. Question 6,
answer A: "a built-in `count()` guard over messages in the session". Section
(g): the menu filter.

**Existing Solutions:**
- Guard compile and evaluation: `validator.guard`
(`internal/realtimecfg/session.go:575-591`), `sessionGuardEnv`
(`session.go:625-651`), `guardHolds` and `sessionDecider`
(`internal/realtime/session.go:393-465`). The `readsEntity` probe
(`session.go:596-617`) is the pattern for the "count not active in this state"
reason (AC 6) and the `count(...)` reason (AC 7).
- Reminder counters are the closest precedent. Declared names go to the
guard env (`s.guardReminders`, `session.go:33-35`, compiled first at
`session.go:247-250`). The record lives on the row (`channels.ReminderRecord`,
`internal/channels/session.go:37-45`). It is carried across moves by
`carriedReminders` (`internal/realtime/reminders.go:114-130`) through
`stepSchedule` (`internal/realtime/timers.go:96-108`), pruned on reload by
`pruneReminders` (`reminders.go:133-145`), and bound by `reminderCounters`
(`reminders.go:167-179`). Counts copy each piece.
- Store write: `SessionSchedule` and `ApplySchedule`
(`internal/channels/session.go:96-158`) already store a record map that realtime
computes. `ApplySession` (`session.go:393-395`) resets the whole `Session`, so a
new field is cleared on a move for free.
- Backend columns: `session_reminders` in `sqlchannels`
(`internal/channels/sqlchannels/sql.go:218,245-268,280-310,518,526,802`),
`filechannels` meta (`internal/channels/filechannels/file.go:410-470,516`), pg
migration `0033_channel_session_reminders.sql`, sqlite `channelReminderColumns`
(`internal/sqlitedb/migrate.go:324-327`) and `schemaVersion = 26`
(`migrate.go:15`).
- Message seq: an append stores the message at `LastSeq + 1`
(`sqlchannels/sql.go:445`, `filechannels/ops.go:1239`), so the decider knows the
seq from the channel it is given. To confirm during implementation for every
backend.
- Edit lock precedent: `m.MovedSession` gives 409 `session_message`
(`internal/realtime/ops.go:219-220`, `session.go:508-511`).
- Output pre-check and gate: `precheckOutputs` runs the decider on the
pre-read head and checks outputs only if it moves
(`internal/realtime/outputs.go:295-338`); `outputGate.admits` then blocks a move
with outputs from any state that was not checked (`outputs.go:271-278`). This is
the race of AC 17.
- Field-level static dependencies: `Program.Attributes("session")` and
`UsesWhole` (`internal/predicate/program.go:33-69`) tell whether a guard reads
`session.counts`.
- Author identity: `sameAuthor` and `authorOf`
(`internal/realtime/authz.go:363-370`); `systemAuthor`
(`internal/realtime/sweep.go:90-92`).
- `acl.AnyRoleGrants` (4/6) for the AC 18 warning.
- Menu: `entities:` navigation entries (TKT-PEKL8L,
`internal/dataentryconfig/config.go:1567-1583`,
`docs/acl-security.md:1330-1339`) serve only the definition, and the SPA reads
rows from a gated list endpoint. Part B copies that. `roleAdmits` calls
`HoldsPermissionFor` per anchor (`internal/realtime/authz.go:276-277`), which is
fine per entity but a per-row call on a cross-anchor list
(`collection-reads.md`). `RefResolver.Resolve` is the batched gated read
(`access.go:81-86`).
- Other projects: Loomio and Decidim count one current position per
participant, and a later vote replaces the earlier one. GitHub required reviews
count the latest review per reviewer, and a dismissed review stops counting.
Both support latest-wins (Q2). XState and SCXML keep counters in `context`,
updated by actions on events, which is the stored-tally model rather than a
query at guard time.

### Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config (`internal/realtimecfg`).**
   - `raw.go`: `rawSession.Counts map[string]rawCount`, with
`rawCount{Label string; Type string; When string; ClearedBy []string
yaml:"cleared_by"; In []string}`.
   - New `counts.go` with an immutable `SessionCount{Name, Label, Type,
when *predicate.Program, clearedBy []string, in []string, inSet map[string]bool,
def string}`. Accessors return copies: `In()`, `ClearedBy()`, `Active(state)`,
`When()`, `Def()`. `def` is the fingerprint: the first 16 hex characters of the
SHA-256 of `type`, `when` source and sorted `cleared_by`.
   - `Session` gains `counts []SessionCount` (sorted by name),
`Counts()`, `Count(name)`, `CountsIn(state)`, and `guardCounts []string` for
every name that matches the grammar, valid or not, so a guard naming a broken
count reports no second error (as `guardReminders` does).
   - A `countCompiler` value type beside `reminderCompiler` runs before
the transitions (`session.go:247`). It implements AC 1 to 5. The `when` env is
`messageRecordType(props)` plus `predicatefns.Declare`. For an `entity` read it
reuses the `readsEntity` probe idea with its own reason.
   - `sessionGuardEnv` gains a `counts []string` argument: it declares
`session.counts` as a record of integers with those names. The transition
compiler passes the names of the counts active in the transition's state. On a
compile error, `v.guard` retries with every declared count. If that compiles, it
reports the AC 6 reason, naming the count by trying each in turn (at most 4). If
the error is `unknown function "count"`, it reports AC 7.
   - `keys.go`: the refused keys of AC 8, and `counts` in the known
session keys. `jsonschema.go`: the `counts` schema. `clone.go`: copy the counts.
`realtimelimits`: `MaxSessionCounts = 4`.
   - The AC 18 warning runs in `writeChecker` for transitions whose
`When` reads `session.counts` (`Attributes("session")` contains `counts`, or
`UsesWhole("session")`).
   - `Transition` gains an unexported `countGuard bool` set at compile,
read through `CountGuarded()`.

2. **Store (`internal/channels`).**

   ```go
   // CountRecord is one session count's tally in its span.
   type CountRecord struct {
       Since   time.Time      `json:"since"`
       Def     string         `json:"def"`
       Authors []CountedAuthor `json:"authors,omitempty"` // sorted by (Org, User)
   }
   // CountedAuthor is one counted author and the seq of their counted message.
   type CountedAuthor struct {
       User string `json:"user"`
       Org  string `json:"org,omitempty"`
       Seq  int64  `json:"seq"`
   }
   ```

   - `Session.Counts map[string]CountRecord`, `SessionSchedule.Counts`
with the reminder rules (nil keeps, empty stores none). `ApplySchedule`
validates through `CheckCounts`: at most `MaxSessionCounts` (4) records, valid
names, at most `MaxCountAuthors` (64) authors, non-empty `User`, `Seq >= 1`, no
duplicate author, `Def` at most 64 bytes. It normalizes `Since`.
   - A sorted slice, not a map, keeps the stored form stable for CAS
compares and for diffs of `meta.yaml`.
   - `sqlchannels`: column `session_counts` (JSON, '' for none) in the
select list, the scan, `decodeCounts` with the damaged-row path, and the three
update statements that write `session_reminders` today.
   - `filechannels`: `meta.SessionCounts`, written and read like
`SessionReminders`.
   - pg migration `0034_channel_session_counts.sql`; sqlite
`schemaVersion = 27`, a rung that adds the column, and the column in
`channelsDDL`. No index: nothing queries by tally.
   - `channelstest.RunSessionCounts` (AC 11, 15), called from `RunAll`.

3. **Tally (`internal/realtime/counts.go`, new).** All functions are
pure, so they are safe under the store's lock.

   ```go
   // ballot is what a post contributes to the counts, computed before the
   // append: the author key and, per count of the session, whether the
   // message is of its type and passes its when, or clears it.
   type ballot struct {
       author channels.Author
       system bool
       hits   map[string]ballotHit // count name -> hit
   }
   type ballotHit int // hitNone, hitCount, hitClear

   func newBallot(ctx, s *realtimecfg.Session, mt *realtimecfg.MessageType, fields map[string]predicate.Value, a channels.Author, system bool) ballot
   func tallyAfter(s *realtimecfg.Session, cur current, ch channels.Channel, b ballot, seq int64) map[string]channels.CountRecord
   func countValues(s *realtimecfg.Session, cur current, counts map[string]channels.CountRecord) predicate.Value
   func carriedCounts(s *realtimecfg.Session, from channels.Channel, counts map[string]channels.CountRecord, to string) map[string]channels.CountRecord
   func pruneCounts(s *realtimecfg.Session, rs map[string]channels.CountRecord) map[string]channels.CountRecord
   ```

   - `newBallot` evaluates each count's `when` once, outside the lock, on
the validated props. A `when` that fails to evaluate counts as a miss, and is
logged once per count per configuration, like a guard. A nil operand (an
optional property left out) is a miss, not a failure.
   - `tallyAfter` applies AC 9: only when `ch.State` is open and the
count is active in `cur.State`. A record whose `Def` differs from the count's is
treated as empty, with `Since = cur.EnteredAt`. A record that is absent starts
at `cur.EnteredAt`.
   - `countValues` binds `session.counts` for the counts active in the
current state, reading 0 for a stale `Def`.
   - The cap (AC 14): a new author over 64 is not added, and the record
is marked by `len == MaxCountAuthors`. The view's `capped` is `len(Authors) ==
MaxCountAuthors`.

4. **Decider and schedule.**
   - `newSessionPost` takes the ballot. `Service.Post` builds it from
`authorOf(c.p)` with `system: false`. The trigger post path builds it with
`system: true`, so trigger posts never count. Reminder posts use the timer step,
which has no ballot.
   - `sessionDecider`: after the open and transition checks, it computes
`counts := tallyAfter(s, cur, ch, b, ch.LastSeq+1)` and passes
`countValues(...)` to `guardHolds` as a new `session.counts` field. It computes
the tally even when the message type has no transition, because Due needs it.
The guard reads only active counts, so the record shape is fixed per state.
   - `withSessionDue` captures the ballot. `stepSchedule` gets the tally:
without a move, `Counts = pruneCounts(tallyAfter(from))`, set only when it
differs from the stored one (so a plain text post rewrites nothing extra). With
a move, `Counts = carriedCounts(tallyAfter(from), to)`: records of counts active
in both the left state and `to`. A timer step (no ballot) carries `from`'s
records the same way.
   - Lifecycle close and reopen do not touch counts. `resumeReminders`
is not involved.

5. **Evaluation semantics, settled.**
   - Scope: per span, not per channel and not per state visit. A re-entry
keeps the tally, so a re-entry cannot wipe votes. This matches the 4/6
deadline-reset concern and reminder records.
   - Which messages: log messages of ballot types posted in the span by
non-system authors while the channel is open. Ephemeral messages are refused at
load. Session notices and reminders are system messages.
   - Edited and retracted messages: the author changes a vote by posting
again. An author edit or delete of the counted message is refused (AC 19). A
moderator removal keeps the tally, as it keeps a move today. Retention purges do
not touch the tally, which lives on the row.
   - Distinct authors: the author key `(OrgID, User)`. One person over the
web and over MCP counts once only when both transports resolve the same `User`.
The web resolves through `principal_property`; CLI and MCP keep the raw
identifier (4/6 approach 6). The docs repeat that identity config must agree
across transports. Agents are ordinary principals and count as authors;
operators keep agents out of a vote with the message type's `post:` roles or
`client_baselines`.
   - Job identities: `run_as` writes graph outputs and never posts
messages, so it never counts. A CLI `--run-as` post counts as that identity,
which is a person-like principal by design.
   - Role changes after a vote: a vote is judged at post time, as a move
is. A voter who loses the role stays counted until they post again or the span
ends. Re-checking roles at guard time would be an ACL call per voter per post.
   - Filter engine: `internal/predicate`, compiled once at load over the
message record, as message `when:` is (`predicate.md`).

6. **Outputs (4/5) interaction.** AC 17 changes `precheckOutputs` for
count-guarded transitions: find `t := Transition(cur.State, mt)`. If
`t.CountGuarded()`, the poster passes `t`, and `t` has outputs, check the
outputs and set `gate.from = cur.State` whether or not the pre-check decider
moved. The decider under the lock then admits the move when the quorum is
reached by a concurrent vote. The cost is that every voter needs the write
grant. The docs give the pattern for a collective decision: the quorum
transition has no outputs, and a `period: 1m` timer out of the next state writes
as `run_as` (the example). We do not let a count-guarded transition write as
`run_as` directly: a guard such as `>= 1` would let any single poster launder a
job write, and the load cannot judge the threshold.

7. **4/6 interaction.**
   - `status` and `waiting_for` are unchanged. A voting state usually
waits for a permission role, so it stays `working`.
   - The can-answer check (4/6 AC 7) stays structural: a count-guarded
answer counts as an answer, as any guarded answer does.
   - The deadline-reset check (4/6 AC 8) is unaffected: re-entries keep
the tally.
   - `waiting_on_me` does not look at whether the reader already voted.
The docs note it.

8. **Timers and triggers.** Timers have no guards, so a timer cannot read
a count. A deadline is a timer out of the voting state; if the quorum came first
the session already left. A trigger post can fire a count-guarded transition
(its guard reads the tally as it is), but a trigger post is never counted.
`jobLoops` is unchanged.

9. **Read path and wire.**
   - `sessionView` adds `Counts []SessionCountView` for a stored channel
in a declared state that is not final: `{Name, Label, Count int64, Capped
bool}`. No extra read: the tally is on the head that every caller already holds.
Counts are channel data shown after the channel read gate, the same for every
reader.
   - `describe.go`: `SessionCountDescription{Name, Label, Type,
ClearedBy, In, Filtered}`.
   - apiwire `ChannelSession.Counts`, `ChannelSessionSchema.Counts`;
OpenAPI; TS types; `wireFields.test.ts`.
   - CLI `sessionText`; SPA `RlChannelSession` shows count chips under
the current state, `channelView.ts` maps them, new `useMessages` keys, story
`Quorum` from the `RlRealtimeSession.vue` mockup.
   - Edit and delete: `Service.Edit` and `Service.Delete` (author path)
check `countedSeq(c.head, m)` after the `MovedSession` check, using the stored
tally (no config needed: any record listing the author with this seq). New error
type `TypeCountedMessage` in `errors.go`.

10. **Cost on hot paths.**
    - Post: up to 4 `when` evaluations before the append, outside the
lock. Under the lock, the tally update is O(authors) (at most 64) per count, and
one guard evaluation as today. One `Append`, no extra store call (AC 21).
    - Performable transitions (`fireable`) and list reads do not evaluate
guards today (`internal/realtime/session.go:290-327`), and still do not.
Rendering counts reads the head the list already loaded.
    - Writes: a ballot post rewrites the `session_counts` column, or
`meta.yaml` on fs, at most 4 x 64 entries (about 10 KB). A non-ballot post
leaves it as it is.
    - Budget tests: AC 21.

11. **Part B: menu entry (if kept; see Q6).**
    - Config: `NavigationEntry.Channels string`, `SessionState []string`,
`ChannelState string`, `Limit int` (plimsoll pin +3; the format-mirror
exception). Only in a group, without `label`, like `entities:`. `ChannelSchema`
(consumer-side, already in `dataentryconfig`) gains `SessionStates(channelType)
(states []string, initial string, ok bool)`, supplied by the wiring site. Served
in `/api/v1/_config` with the rest of the tree; config is not secret.
    - Store: `ListBySessionState(ctx, typ string, states []string,
matchUnset bool, state State, after Cursor, limit int) ([]Channel, Cursor,
error)`, ordered by last activity. pg and sqlite get an index on `(tomb, type,
session_state, updated_at)`; fs scans the type directory under a scan cap.
Conformance in `channelstest`. The inbox ticket reuses it.
    - Service: `Service.ListType(ctx, typ, states, page)`. Per page of
candidates: one batched gated anchor read with the read role's property (a new
`AnchorReader.Anchors` method, or `RefResolver` with props), then the read role
per row in memory. A permission read role needs a batched
`HoldsPermissionForMany(ids, perm)` on `Access`. Fill the page by fetching
further candidate pages up to a scan cap; `next` comes from the last served row
(post-filter).
    - HTTP: `GET /api/v1/_channels` in `internal/dataentry`, with apiwire
and OpenAPI. A hidden anchor is absent, not an error.
    - SPA: a `channels` nav item that fetches the endpoint, like the
`entities:` item, and links to the entity page channel tab.
    - Budget: AC 29.

**Alternatives rejected:**
- An inline `count(type, distinct, filter)` function in the guard: it
needs a log scan at evaluation time. Under the lock that is I/O, which the store
contract forbids. Before the lock it races (problem fact 2). The SPA could not
show a count it does not know either.
- Counting messages instead of authors: one person could reach a quorum
alone.
- Any matching message ever counts (no latest-wins): a voter could not
change or withdraw a vote. Q2.
- Whole-channel scope: a second round after `revising` would inherit the
first round's votes.
- Resetting on re-entry: a poster who can re-enter would wipe the votes.
- Recounting on author edit or delete: needs a hook in the store's edit
and remove paths on all backends, and the seq of every ballot message, not just
counted ones. Refusing the edit is simpler and matches `session_message`.
- Writing count-guarded outputs as `run_as`: laundering through a low
threshold (approach 6).
- Storing an author set without seqs: then the edit check (AC 19) would
need a log read.
- A count `roles:` filter: duplicates `post:`, and role answers would be
per voter per post.

**Files to modify:**
- `internal/realtimecfg/raw.go`, `keys.go`, `session.go`, `counts.go`
(new), `clone.go`, `jsonschema.go`, `outputs.go` (warning), tests
(`counts_test.go`, `session_test.go`, `load_test.go`, `docs_example_test.go`),
`testdata/valid.yaml`, `testdata/acl.yaml` (grant `job:risk-approvals` update on
`risico.status`, and `risk.review`).
- `internal/realtimelimits/limits.go` (`MaxSessionCounts`).
- `internal/channels/session.go`, `channelstest/` (new
`RunSessionCounts`), `sqlchannels/sql.go`, `filechannels/file.go`.
- `internal/store/pgstore/migrations/0034_channel_session_counts.sql`
(new), `internal/sqlitedb/sqlitedb.go`, `migrate.go`, migration tests.
- `internal/realtime/counts.go` (new), `session.go`, `timers.go`,
`service.go` (`Post`), `system.go` or the trigger post path (system ballot),
`outputs.go` (`precheckOutputs`), `ops.go`, `errors.go`, `describe.go`, tests.
- `internal/appbuild` end-to-end test.
- `internal/apiwire/v1/channels.go`, `internal/openapi/channels.go`
(+ test), `internal/cli/channel_render.go` (+ test).
- `frontend/src/types/channels.ts`, `wireFields.test.ts`,
`frontend/src/components/entity/channels/channelView.ts` (+ test),
`frontend/packages/rela-components/src/components/channel/{types.ts,
RlChannelSession.vue, RlChannelSession.stories.ts}`, `useMessages.ts`.
- `docs-project/entities/guides/GUIDE-realtime.md` (then `just docs`).
- Part B only: `internal/dataentryconfig/config.go`, `channels.go`,
navigation validation, `internal/channels/store.go` and backends,
`internal/realtime/access.go`, `service.go`, `internal/dataentry` handler,
apiwire, OpenAPI, SPA sidebar, `GUIDE-data-entry.md`, `docs/acl-security.md`.

### Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` `session.counts`: operator config, checked at load
against declared message types and states (allowlists). `when` is a sandboxed
predicate with no I/O and fixed budgets. Config is not secret, so the descriptor
serves the count definitions.
- Message props: validated by the message type before `newBallot` sees
them. The filter reads only those props.
- Author: from the authenticated principal (`authorOf`), never from the
request body. Trigger and reminder posts are marked by the store (`TriggeredBy`
markers that only the store writes), so a poster cannot pose as a system author
to dodge or forge a count.
- Stored tally: validated on write and on read (`CheckCounts`); a bad row
reads as damaged, as reminder records do.
- Part B query params: `type` must be a configured channel type,
`session_state` declared states, `limit` capped. Unknown values are 400 with the
config key named; config is not secret.

**Security-Sensitive Operations:**
- ACL on the count: a count is an aggregate, so the "gate before fold"
rule was checked. Channel messages are not row-gated per reader: whoever passes
the channel read gate (which needs anchor read) sees every message and its
author. So the tally folds only data every reader of the channel can already
see, and the number is the same for every reader. Nothing is cached across
principals that differs by principal. The count is shown only after the read
gate, in the session view.
- A `when` over a `ref` prop compares raw ids, as a transition guard does
today. A reader who cannot read the ref'd entity sees the ref redacted but may
infer from the count whether a configured id was named. This is the same one-bit
channel 4/1 guards already have (a move is visible to every reader), not a new
class. Documented.
- Writes: the quorum move's outputs still write as the poster, after the
pre-check. AC 17 makes that pre-check stricter, never weaker. No allow-all is
inferred.
- Vote stuffing: one vote per author key. Many identities (agents) can
still vote; the operator limits voters with `post:` roles. Documented.
- Edit refusal (AC 19) stops an author from changing a counted message
after the guard or the tally judged it.
- Error answers: `counted_message` concerns the caller's own message in a
channel they already read; it reveals nothing new.
- Part B: a channel row reveals its anchor's existence, so the list
row-gates before paging and computes `next` after the gate (the gantt lesson in
CLAUDE.md). A permission read role is answered by a batched ACL call, not
skipped.

### Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named in the
criterion. `realtimecfg` tests are table-driven over `testdata`. The tally
functions get pure table tests. `internal/realtime` uses the service harness
with a counting store and access for budgets. `channelstest.RunSessionCounts`
runs on fs, sqlite and pg (pg through the schema-pinned DSN, `storage.md`).
`internal/appbuild` runs the quorum end to end over memstore with the real
policy and a capturing audit sink. MCP watch uses the shortened tick and poll of
4/6.

**Edge Cases (each has a test):**
- A count with no `when`: every message of `type` counts.
- A vote with an optional property the `when` reads left out: a miss,
which removes an earlier counted entry by the same author.
- The quorum vote's message type also appears in `cleared_by` of another
count: both counts update in one append.
- A session that never moved: the span starts at `created_at`, and the
first vote both creates the row's session and counts (unstored channel created
by its first post).
- A re-entry through another message type keeps the tally.
- A reload that renames a count: the old record is pruned at the next
write, and the row never exceeds `MaxSessionCounts`.
- A stranded state: no counting, records kept until the state returns.
- A degraded configuration: counting and guards run on the last valid
config, as moves do.
- The 64th and 65th authors (AC 14).
- Two votes by the same author at once: the higher seq wins.
- A post into a closed channel by a trigger with `when_closed: post`:
not counted, does not move.

**Negative Tests:** AC 1 to 8 (config refusals), AC 9 (ignored posts), AC 13
(stale definition reads 0), AC 15 (invalid records and damaged rows), AC 17
(ungranted voter refused before storing), AC 19 (counted message edit refused),
and for part B AC 26 and AC 28.

### Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Store contract change on three backends, plus migrations. Mitigation:
copy the reminder record path line for line; one conformance suite.
- Due computing the tally and the decider computing it again could drift.
Mitigation: one `tallyAfter` function, called by both with the same inputs; a
test compares the guard's view with the stored result.
- A count used with outputs forces write grants on every voter (AC 17),
which may surprise operators. Mitigation: the AC 18 warning and the documented
timer pattern.
- Row growth: up to 4 x 64 authors rewritten on each ballot post; on fs
that is a `meta.yaml` rewrite. Mitigation: the cap, and only ballot posts
rewrite it.
- The cap silently stops counting new authors. Mitigation: `capped` in
the view and CLI, a warning log, and the documented limit.
- Identity mismatch across transports can double-count one person.
Mitigation: documented, as for claims in 4/6.
- Part B is a new cross-anchor read surface with batched ACL work that
does not exist yet. Mitigation: split it (Q6).

**Effort:** part A m; part B m. Together l.

### Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. `GUIDE-realtime.md` (generates
`docs/realtime.md`): a "Counts" subsection under "Sessions" with the example,
the counting rule, latest-wins and `cleared_by`, spans and re-entry, what never
counts (system posts, closed channel, before the span), the cap, definition
changes, the edit refusal and moderator removal, identity across transports,
agents, and the timer pattern for collective outputs. The guard paragraph lists
`session.counts`. The load rules go in "Checks when the file loads". `counts`
and `capped` go in "HTTP API", and the 409 `counted_message` in the error table.
Part B, if kept: `GUIDE-data-entry.md` navigation section and
`docs/acl-security.md` (the channel list keeps the `entities:` property).

**Open design questions (recommended answer first):**

- **Q1. Declared counts, not an inline function.** Guards read
`session.counts.<name>`, declared under `session.counts`, instead of
`count(type, distinct, filter)` inline. Reason: an inline count needs a log
scan, which is I/O under the lock or a race before it. Recommended.
- **Q2. Latest message per author decides**, with `cleared_by` for a
withdraw type. The alternative is "any matching message ever counts" (simpler,
but no vote change). Recommended: latest-wins.
- **Q3. Scope is the span of `in:` states**, carried across re-entries,
reset on leaving. The alternatives are per state visit or the whole channel.
Recommended: span.
- **Q4. Author edit and delete of a counted message are refused (409);
a moderator removal keeps the vote.** The alternative is recounting on edit and
delete, which needs store hooks. Recommended: refuse.
- **Q5. Outputs on a count-guarded transition need the grant for every
voter**, with the timer-plus-`run_as` pattern for collective writes.
Recommended. The alternative (quorum outputs as `run_as`) opens a laundering
path.
- **Q6. The menu filter needs a feature that does not exist yet:** a
`channels:` navigation entry, a cross-anchor channel list with batched ACL, and
a store query by session state. Recommended: split part B into its own ticket
("Sessions 4/7b: channel menu entries"), sequenced with or after the inbox,
which needs the same store query. Ship part A as 4/7.
- **Q7. Cap of 64 distinct authors per count.** Higher caps grow every
ballot write. Recommended: 64, raise later if a real vote needs it.

### Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:**
