---
id: PLAN-OD6OXJ
type: planning-checklist
title: 'Planning: Inbox 1: inbox read with waiting entries'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Sessions 4/6 tell one reader, one channel at a time, whether a
session waits on them (`waiting_on_me`, `internal/realtime/waiting.go`). Nothing
answers "which sessions wait on me, across every channel". The agent-runner demo
needs exactly that: an agent (a `system:` or `integration:` job identity, or a
user) long-polls one MCP tool with `watch` and acts on each session that waits
on it. RES-B0MJAZ slice 1 ("f2") decides the design: find the entities whose
role property names the caller with one gated graph query per configured pair,
fetch their channels in one batch, then reuse `waitingView` per channel. Creator
roles need a queryable creator column.

Terms used below:
- **Principal key**: `QueryIdentity.ID()`, the user entity id when the
transport resolved one, else the raw identity (`jeroen`,
`integration:triage-bot`).
- **Waiting pair**: one (entity type, role property, anchor mode) that some
`input_required` state's `waiting_for` reads through a `from: entity.<prop>`
role. Computed from config only.
- **Holder query**: the gated graph query "entities of type T whose `prop`
names me" for one waiting pair.
- **Entry**: one inbox row: a stored, open channel whose current state is
`input_required` and whose waiting roles the caller holds, exactly when
`waiting_on_me` would be true on a `read_session` of that channel.
- **Window**: the gated, sorted entry list one request computes before it
pages. Every count, cap and `truncated` flag is taken on it.

**Scope:**
- In: `realtimecfg.Compiled.Waiting()`: the waiting pairs, the creator
waiting types and, per channel type, the stored session states that can wait
(with `""` when the initial state waits). Load-time, config only.
- In: `channels.Store.ListWaiting(ctx, WaitingQuery)` on every backend
(filechannels on disk and on MemFS, sqlitechannels, pgchannels through
sqlchannels), pinned by `channelstest`.
- In: column `created_by_user` on `channels` for sqlite (rung v26 to v27)
and postgres (migration 0034, guarded per row), written on insert, backfilled
from the `created_by` JSON, with a partial index. Nothing on the file tier.
- In: a gated, redacting holder lookup (`visibility.HolderLookup`, a
separate small type built next to the readers in `scriptReads`, see Approach 4),
wired only into appbuild's `realtime.HolderFinder`. The reader family does not
grow.
- In: a batched anchor read (`AnchorReader.Anchors`) and `AnchorView.Title`.
- In: `Access.Prime(ctx, ids)` so permission read roles batch their
relation walk (`acl.PrimeLocalRoles`).
- In: `Service.Inbox(ctx, InboxRequest) (InboxView, error)`.
- In: `GET /api/v1/_inbox` with `limit` and `cursor`; wire types, golden
field set, OpenAPI path.
- In: MCP `list_inbox`, added to `watchOrder`, woken by any channel change,
store events and the channel poll.
- In: `rela channel inbox` (text and `--json`), because it reuses the
existing channel service binding and costs one file.
- In: docs: `docs/realtime.md`, `docs/mcp-server.md`,
`docs/cli-reference.md`, one paragraph in `docs/acl-security.md`.
- In: a budget test (equal store calls at 10 and 50 entries, half the
anchors hidden) and an ACL test that hidden rows change nothing.
- Out: read markers, unread counts, `mark_read` (TKT-2B2AIA).
- Out: mentions and replies (TKT-6RXAC0). The entry has `kind: waiting`
so those slices add kinds without a wire break.
- Out: SPA inbox page, sidebar badge, live shape and `i:` topics
(TKT-9YRRM5). `/api/v1/_inbox` is not subscribable on the live stream in this
slice; the TypeScript mirror is not added.
- Out: mail and desktop notifications (TKT-Z3PS9U). Cross-process live
(TKT-YK30D1); the existing 2 s / 5 s watch poll covers other processes.
- Out: an `awaits:` key (decision: `waiting_for` covers it).
- Out: a new way for job identities to hold roles. They hold a named role
through a string property that holds the job identity, or the creator role of a
channel they opened. This is documented, not built.

**Example (docs example and testdata):**

```yaml
# realtime.yaml (excerpt). owner is a string property of risico.
channel_types:
  breach-response:
    attach_to: [risico]
    cardinality: many
    roles:
      lead:   { from: entity.owner, label: the incident lead }
      opener: { creator: true, label: whoever opened it }
    session:
      initial: detected
      states:
        detected:  { waiting_for: [lead], on: { assessed: { to: assessed, roles: [lead] } } }
        assessed:  { waiting_for: [opener], on: { notified: { to: notified, roles: [opener] } } }
        notified:  { final: true }
```

`GET /api/v1/_inbox` for the holder of `owner` on RISK-1:

```json
{
  "notice": "…untrusted…",
  "entries": [
    {
      "kind": "waiting",
      "channel": { "anchor": "RISK-1", "entity_type": "risico", "anchor_title": "Laptop lost",
                   "type": "breach-response", "name": "0k3x…", "title": "Breach 12", "state": "open" },
      "session": { "state": "detected", "label": "detected", "status": "input_required",
                   "waiting_for": [{ "role": "lead", "label": "the incident lead", "holders": ["USR-7"] }],
                   "generation": 0, "entered_at": "2026-10-27T09:00:00Z" },
      "roles": ["lead"],
      "since": "2026-10-27T09:00:00Z",
      "transitions": ["assessed"],
      "links": { "entity": "/api/v1/risicos/RISK-1",
                 "channel": "/api/v1/risicos/RISK-1/_channels/breach-response/0k3x…" }
    }
  ],
  "count": 1,
  "truncated": false,
  "degraded": false,
  "next_cursor": ""
}
```

MCP answers the same object without `links` (the agent addresses a channel by
`channel.anchor`, `channel.type`, `channel.name`, which are `read_session`'s
arguments).

**Acceptance Criteria:**

1. `Compiled.Waiting()` lists one waiting pair per (attach type, property,
anchor mode) that an `input_required` state's waiting role reads, the channel
types with a creator waiting role, and per channel type the stored states that
can wait, with `""` exactly when the initial state is `input_required`. A config
without sessions, or without `input_required` states, has none. Test:
`TestCompiledWaiting` table in `internal/realtimecfg` (named role, creator role,
two types sharing a property, face-anchored type, initial-waits row, final-only
row).
2. `ListWaiting` returns the live (not tombstoned), open channels whose type
is in `q.States` and whose stored session state is listed for that type, that
are either on one of `q.Anchors` (exact id and face; `""` is the family) or of a
type in `q.CreatorTypes` and created by `q.CreatedBy` (exact `Author.User`).
Each channel appears once, even when both branches match it. Closed, archived,
tombstoned, other-state, other-type and other-face channels are left out. An
empty `q.States` or an invalid type is `ErrInvalidKey` (as `CheckDueQuery`);
neither anchors nor creator, or more than `MaxWaitingAnchors` anchors, is
`ErrInvalidKey` too. More than `channels.MaxWaitingRows` matches after merging
and deduplication is `ErrTooManyRows`. Order: anchor id, face, type, name. Test:
`testListWaiting` in `channelstest`, run by every backend's existing conformance
entry point, with rows for: a channel matched by both branches (one result), 1
200 anchors (crosses the SQL chunk size), a merged result of `MaxWaitingRows+1`
spread over chunks (lowered in the test), and a creator channel of a type not in
`CreatorTypes` (absent).
3. A channel created through `CreateMany` or the first `Append` of a
cardinality-one channel stores `created_by_user` = `Author.User`, and a `Rekey`
and an untombstone keep it. Test: rows in `testListWaiting` (create, rename the
anchor, query by creator; tombstone then untombstone).
4. Upgrade: a sqlite database at v26 and a postgres schema at 33 with
channels gain the column, filled from `created_by.user`. A row whose
`created_by` is not valid JSON or holds `\u0000` keeps `''` and the migration
succeeds. Running the rung again changes nothing. Tests:
`TestMigrate_v26ToV27ChannelCreator` in `internal/sqlitedb` (seed a v26 file,
open, query), `TestMigration0034_ChannelCreatorBackfill` in pgstore
(schema-pinned DSN; seeds one valid row, one row whose `created_by` holds the
JSON escape `\u0000`, and one row with invalid JSON; the migration succeeds and
the last two keep `''`), and the ladder well-formedness test.
5. The holder lookup returns only refs the caller may read, judged on the
redacted row: row gate, face allowlist and world as a list read; props are ANDed
on every verdict branch, including AllowAll (RR-GQWRLD); each streamed row is
redacted and the property test is applied again to the redacted row, so a row
whose role property `visible:` hides from the caller is neither returned nor
counted toward the limit. The no-gate variant returns the same refs without
redaction. The refusing variant returns `ErrReaderUnavailable`. A zero
`ReadQueryResult` (nil query with no verdict) is an error, not a fallback.
Without pushdown (no policy, or a store without `GraphQueryer`) it returns the
same refs. Results are in (id, face) order; at `limit` it reports `more`. Tests:
`TestHolderLookup_GatedLikeList` (policy, no-gate and refusing variants; a
row-hidden match is absent; parity with `ListEntities` plus a Go filter, on
every backend), `TestHolderLookup_HiddenPropertyNotCounted` (limit 2, three rows
whose hidden property names the caller and two visible ones: `more` is false),
`TestHolderLookup_ZeroResultErrors` in `internal/visibility`.
6. A user who holds the named waiting role on RISK-1 sees one entry for each
open, stored channel in a waiting state there; `read_session` on each answers
`waiting_on_me: true`, and on every channel the inbox leaves out it answers
false. Test: `TestInbox_MatchesWaitingOnMe` in `internal/realtime`: a table of
channels (holder, not holder, closed, final, stranded, `working` state, unstored
cardinality-one, list property holding the caller, a list value on a property
the schema now declares scalar, creator role by the caller, creator role by
another user in the same org, creator in another org, a family-anchored type on
a faced entity type whose second-ranked face names the caller and whose first
does not, a face-anchored channel on an unreadable face of a readable id)
compares the inbox key set with the keys where
`Session(...).Session.WaitingOnMe` is true. For every entry it also compares
`Transitions` with `Session(...).Session.Transitions`; two channels on one
anchor, created by different users, with a creator role among the transition
roles, pin that the answers are not shared between channels.
7. A creator waiting role matches through `created_by_user` and then
`sameAuthor` (user and org). A channel created through the CLI as `jeroen` does
not match the data-entry caller `USR-7`, as today. Test: rows in AC 6.
8. Gate before fold: an entry on an entity the caller cannot read, on a
channel whose `read:` role the caller fails, or whose role property `visible:`
hides from the caller, is absent, and `count`, `truncated` and `next_cursor` are
byte-identical to the answer for a project where those rows do not exist. Test:
`TestInbox_HiddenRowsChangeNothing` (seed visible set V; seed V plus hidden rows
of each kind; compare the two JSON answers, at a limit that pages and at the
anchor bound, lowered in the test; hidden kinds include a role property
`visible:` hides from the caller).
9. `truncated` is set only when the deduplicated list of gated, redacted
holder anchors over all pairs exceeds `MaxWaitingAnchors`; the anchors kept are
the first in (id, face) order, so the same project gives the same subset on
every call. Never from a pre-gate count. Test: `TestInbox_TruncatedIsPostGate`
(bound lowered in the test; bound+1 readable holders set it; bound readable plus
many row-hidden and property-hidden holders do not; two calls give identical
answers).
10. Entries are ordered by `since` ascending (oldest wait first), then
anchor id, face, type, name. `limit` is 1 to 200 (default 50). `next_cursor`
pages the window without gaps or repeats, and a cursor from another principal or
for a now-hidden row positions only (it serves nothing hidden). A malformed
cursor is a 400 `invalid_cursor`. The cursor time is UTC RFC3339Nano at
microsecond precision and is normalized with `channels.NormTime` on decode.
Test: `TestInbox_Paging` (75 entries, limit 20: four pages, union equals the
window; entries with equal `since` and with `since` values 1 µs apart straddle
page boundaries; a forged cursor naming a hidden anchor returns the same page as
the nearest visible key).
11. Cost: one inbox request costs one holder query per waiting pair, one
`ListWaiting`, one anchor header batch and one local-role prime, whatever the
number of entries. Test: `TestInboxBudget_10vs50` in `internal/dataentry`
(storetest.Counting plus countingChannels; equal at 10 and 50 entries). The N
entries sit on N distinct anchors spread over at least two waiting pairs, plus
creator-branch channels; half the anchors are hidden; the channel `read:` role
is a `permission:` role conferred by a relation, so a per-anchor permission walk
or a per-pair loop would change the count. `ListEntity`, `Head` and per-row
`ResolveHeaders` calls stay at zero.
12. A degraded configuration answers 200 with `entries: []`,
`degraded: true`, and makes no store call. Realtime off answers 404 on REST and
the off text on MCP and CLI. A caller with no usable identity is 403
`no_principal`. More than `MaxWaitingRows` candidate channels is 503
`inbox_too_large` on REST and a tool error with the same code on MCP and CLI.
Test: `TestInbox_DegradedAndOff`, `TestInboxHandler_Errors` (off, no principal,
bad limit, bad cursor, method not allowed, row ceiling as 503 `inbox_too_large`,
other store fault as 500 without detail).
13. `GET /api/v1/_inbox` returns the wire shape above, documented in the
OpenAPI document; the golden wire field file lists `Inbox`, `InboxEntry`,
`InboxChannel` and `InboxSession`. Test:
`TestOpenAPI_EveryOperationReachesAHandler` (existing), `TestChannelWireGolden`
(updated), `TestInboxHandler_Shape`.
14. MCP `list_inbox` returns the same object without `links`, starts with
the untrusted notice, and is watchable: a `watch` on it wakes when a session
enters or leaves a waiting state for the caller (post in this process, a timer
move, a handoff by entity edit, a post in another process within the poll), and
not when an unrelated message is posted in a channel that stays waiting. The
digest is per caller. A watch holds at most one `list_inbox` target. On the poll
path a `list_inbox` target replays at most every 15 s, and a replay that did not
finish doubles that interval (up to the watch deadline) instead of rerunning at
the next tick. Tests: `TestListInbox_Answer`, `TestWatch_ListInboxWakes`
(in-process post, entity edit through the store, poll path with the subscription
off), `TestWatch_ListInboxNoWakeOnChat`, `TestWatch_ListInboxDigestPerCaller`,
`TestWatch_ListInboxOneTarget` (a second target is a tool error),
`TestWatch_ListInboxSlowReplayDoesNotSpin` (a fake inbox that takes longer than
`watchReplayTimeout` is called at most once per backoff interval).
15. `rela channel inbox` prints one line per entry, such as
`RISK-1 breach-response/0k3x… detected, input_required since
2026-10-27T09:00:00Z, waiting for the incident lead (you)`, and `--json` prints
the service answer. Test: `TestChannelInbox_Text`, `TestChannelInbox_JSON` in
`internal/cli`.
16. A job identity (`--run-as integration:triage-bot`) sees a session whose
string property `owner` holds `integration:triage-bot`, and one it created on a
type with a creator waiting role. Test: `TestInbox_JobIdentity` (stdio wiring
through the CLI test harness).
17. Docs describe the inbox, the entry, ordering, paging, caps, how each
transport's identity matches holders, and the agent loop. Test: review (docs
checklist).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (decisions 2026-10-27: slice 1 first, f2, gate the
whole candidate set, no `awaits:` key).

**Existing Solutions:**
- No library: this is a gated query over rela's own stores.
- `waitingView` / `waitingOf` (`internal/realtime/waiting.go:44-118`): the
per-channel rule. The inbox calls it unchanged, per channel, with facts built
from the batch, so the inbox and `read_session` cannot disagree (AC 6 pins it).
- `Service.ListAnchor` / `call.listType` (`internal/realtime/service.go:671-760`):
the pattern of one anchor read plus one channel list, with the call's `ct`,
`head` and `ops` reset per channel type. The inbox loop is the same with many
anchors.
- `roleAdmits`, `propertyNamesCaller`, `sameAuthor`
(`internal/realtime/authz.go:272-366`): role matching, reused as is.
- Current-user pushdown: `queryplan.ConditionPrefilters`
(`internal/queryplan/queryplan.go:376-398`) lowers `is_current_user` /
`has_current_user` to `store.PropPredicate{Op: PropEqual, Value: identity,
Scalar: !list}`. The holder query builds a similar predicate directly, but with
`Scalar: false` so it follows the stored value's shape, as `propertyNamesCaller`
does.
- ACL pushdown: `visibility.composeReadScope` / `listPushdown`
(`internal/visibility/pushdown.go:68-181`) and `dataentry.scopedHeaders`
(`internal/dataentry/scopedread.go:162-230`, the RR-GQWRLD and TKT-O7R2A1
lessons: extra props on every verdict branch, copy the ACL query before
stamping, carry world and `FaceIn`).
- Gated reader precedent for a new query: `MentionedBy` on all three
readers (`internal/visibility/mentions.go`,
`internal/appbuild/appbuild.go:585-595`).
- Batched anchor read: `anchorReader` / `refResolver`
(`internal/appbuild/realtime_service.go:281-360`) already call
`ResolveHeadersErr`, which takes many refs.
- `acl.PrimeLocalRoles` (`internal/acl/localroleprime.go:56`), used by
`internal/dataentry/affordances.go:315`.
- Channel budget harness: `countingChannels` and
`TestChannelsBudget_List10vs50` (`internal/dataentry/channels_budget_test.go`).
- Watch: `watchOrder`, `channelWake`, `watchChannel.matches`
(`internal/mcp/tools_watch.go:72,317-380`).
- Schema rungs: `addChannelColumns` (`internal/sqlitedb/migrate.go:266-290`),
pg `0031`/`0033` add-column migrations, `channelTimersDDL` (index made after the
ladder because it names a rung's column).
- Prior art from the research: GitHub notifications (reason per item,
polling-friendly), Linear's derived "My issues" view kept apart from the pushed
inbox.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config index** (`internal/realtimecfg`). Add `Compiled.Waiting()`
returning a `WaitingIndex` built once in `Compile`:
   ```go
   type WaitingPair struct {
       EntityType string     // an attach_to type
       Property   string     // the role's anchor property
       Face       bool       // the channel type anchors to a face
       Types      []string   // channel types that use this pair, sorted
   }
   type WaitingIndex struct {
       Pairs   []WaitingPair         // sorted by EntityType, Property, Face
       Creator []string              // channel types with a creator waiting role
       States  map[string][]string   // channel type -> stored states that can wait; "" for initial
   }
   ```
A state counts when `Status == StatusInputRequired`. Only bounded roles can wait
(4/6 load check), so every waiting role is either a named role or a creator
role.

2. **Store method** (`internal/channels`). Add to `Store`:
   ```go
   // WaitingQuery selects the channels an inbox looks at.
   type WaitingQuery struct {
       States       map[string][]string // channel type -> stored session states; "" is a session that never moved
       Anchors      []AnchorKey         // exact (id, face); face "" is a family-anchored channel
       CreatorTypes []string            // channel types the creator branch covers
       CreatedBy    string              // Author.User of the creator; "" matches none
   }
   type AnchorKey struct{ ID string; Face entity.Face }
   ListWaiting(ctx context.Context, q WaitingQuery) ([]Channel, error)
   ```
`CheckWaitingQuery` validates with `ErrInvalidKey`, as `CheckDueQuery` does:
types valid, at least one state, anchors or (creator and creator types), anchor
ids and faces valid, at most `MaxWaitingAnchors` = 20 000 anchors.
`MaxWaitingRows` = 20 000 distinct channels after merging; more is
`ErrTooManyRows`. Exact faces keep channels on a face the caller cannot read out
of the candidate set, so they do not count toward the ceiling. The result is a
superset; realtime decides each entry.
- **sqlchannels**: `SELECT chanCols FROM channels WHERE tomb = '' AND
state = 'open' AND ((type = ? AND session_state IN (…)) OR …) AND <branch>`. The
anchor branch is `(anchor_id, anchor_face) IN (VALUES (?, ?), …)` in chunks of
500 pairs; the creator branch is `created_by_user = ? AND type IN
(CreatorTypes)` and runs once. All statements run in the one call. Rows merge
into a map keyed by channel key, so a channel both branches return counts once;
the ceiling applies to the merged count (each statement also carries `LIMIT
MaxWaitingRows+1` so no statement reads unbounded). The merged rows sort with
`channels.SortChannels`. Damaged rows are skipped with the `DamageWarner`, as
`ListSessionsDue` does. Bump the plimsoll directive on `sqlchannels.Store` to 30
(required interface). Chunks are used on postgres too, not `= ANY($n)`:
sqlchannels shares one `?`-placeholder SQL text across both databases and its
`Dialect` has no array binding; chunking keeps one code path.
- **filechannels**: the anchor branch uses `visitEntity` per distinct
anchor id and keeps the listed faces; the creator branch uses
`visitTypes(CreatorTypes)` once and filters on `meta.CreatedBy.User`. Merge by
key as above. No lock, as `ListSessionsDue`. Bump its directive to 30. The MemFS
store is the same code.
- Conformance: `testListWaiting` in `channelstest` (AC 2, 3).

3. **Creator column.**
   - sqlite: `schemaVersion` 27; rung v26 to v27 adds `created_by_user TEXT
NOT NULL DEFAULT ''` through `addChannelColumns`, then `UPDATE channels SET
created_by_user = COALESCE(json_extract(created_by, '$.user'), '') WHERE
created_by_user = '' AND json_valid(created_by) AND instr(created_by, '\u0000')
= 0`. `channelsDDL` gains the column for fresh files. A new `channelCreatorDDL`
creates `channels_creator_idx ON channels(created_by_user, type) WHERE tomb = ''
AND created_by_user <> ''` after the ladder, like `channelTimersDDL`.
   - postgres: `0034_channel_created_by_user.sql`: `ALTER TABLE channels
ADD COLUMN IF NOT EXISTS created_by_user TEXT NOT NULL DEFAULT ''`; then a
backfill through a temporary plpgsql function (the pattern of
`0024_mention_targets.sql`), `rela_creator_user(t text)`, that returns `''` when
`strpos(t, '\u0000') > 0` (with `standard_conforming_strings` on, this matches
the six-character JSON escape that jsonb rejects; `LIKE` would treat the
backslash as its escape character) and otherwise `COALESCE(t::jsonb ->> 'user',
'')` inside a `BEGIN … EXCEPTION WHEN others THEN RETURN ''; END` block, so
invalid JSON keeps `''` on every supported postgres version (`pg_input_is_valid`
needs PG16). `UPDATE channels SET created_by_user =
rela_creator_user(created_by) WHERE created_by_user = ''`; `DROP FUNCTION`; the
same partial index. Header comment in the style of 0031/0033.
   - `sqlchannels.insertChannel` writes `created_by_user = nc.By.User`.
`Rekey`, tombstone and untombstone update other columns only, so the value
travels. `Raw` adds the column to the erasure dump.
   - File tier: no change. `meta.yaml` already has `created_by`.
   - A row the backfill skips keeps `''`: its creator gets no creator
entry. That fails closed (nobody else gets it either).
   - The `datamigration` rule does not apply: this is a schema rung, not a
`migrations/*.yaml` data migration.

4. **Holder lookup** (`internal/visibility`, wired in `internal/appbuild`).
`ScriptReader` and `UnrestrictedReader` already have 20 exported methods each
(the plimsoll cap), and only realtime needs this query, so the reader family
does not grow. Instead, a new small type in `internal/visibility/holders.go`:
   ```go
   // HolderLookup finds the rows of one type whose property names an
   // identity, through the same gate, redactor, store and world as the
   // script readers. The limit counts rows after the row gate AND after
   // field redaction: a row whose property visible: hides from the caller
   // is neither returned nor counted.
   type HolderLookup struct{ /* gate, redactor, store, world, mode */ }
   func NewHolderLookup(...) (*HolderLookup, error)   // rejects nil collaborators
   func NewUnrestrictedHolderLookup(st store.Store) (*HolderLookup, error)
   func NewRefusingHolderLookup() *HolderLookup         // every call: ErrReaderUnavailable
   func (h *HolderLookup) Matching(ctx context.Context, q HolderQuery) (refs []entity.Ref, more bool, err error)
   type HolderQuery struct {
       EntityType, Property, Identity string
       Faces store.FaceSelection
       Limit int
   }
   ```
`scriptReads` builds it next to the reader from the same inputs (NopACL gives
the unrestricted variant, a missing gate the refusing variant).
- Gated mode: `composeReadScope`. A zero `ReadQueryResult` (nil query
and no deny or allow verdict) is an error, as in `scopedHeaders`, not the
fallback `composeReadScope` would take. Deny returns none. Allow builds
`store.GraphQuery{EntityType, Props: [p], Faces, FaceIn: rqr.Faces}`; graph
copies the ACL query and appends `p`. Both use `OrderBy: id, face` and `Limit:
q.Limit+1`, paging with `Offset` while fewer than `q.Limit+1` rows survived, so
every backend returns a deterministic order and reads a bounded page
(graphquerynaive only orders when paged). Each row is redacted (`RedactRow`, as
`listPushdown`'s `redactingSeq`) and the property test is applied again to the
redacted row (`predicatefns` equality: the string, or an element of a list);
only survivors count. Without pushdown: `listGatedThenRanked` plus redaction,
then the same Go test, in the same order.
- `p` is `PropPredicate{Property, Op: PropEqual, Value: Identity,
Scalar: false}`. Non-scalar `PropEqual` is equality on a scalar value and
membership on a list value, which is `propertyNamesCaller`'s rule whatever the
schema now says about the property's shape. An empty identity returns none
(never a match on unset).
- Unrestricted mode: raw `GraphQuery` with `p`, same order and limit.
- Refusing mode: `ErrReaderUnavailable`, as `DenyReader`.
- appbuild `holderFinder` adapts it to a realtime consumer interface:
  ```go
  // HolderFinder finds the anchors whose role property names the caller,
  // through the gated read path. A ref the caller may not read is never
  // returned, so a cap on the result is taken after the gate.
  type HolderFinder interface {
      Holding(ctx context.Context, q HolderQuery) (refs []AnchorRef, more bool, err error)
  }
  type HolderQuery struct{ EntityType, Property, Identity string; Face bool; Limit int }
  type AnchorRef struct{ ID string; Face entity.Face }
  ```
It selects `store.AllFaces()` for a face-anchored pair and
`store.InWorld(default world)` for a family-anchored pair, as `scriptReads`
does, and maps refs to `AnchorRef` (face `""` for a family pair, matching the
channel's stored `anchor_face`).
- `realtime.Deps.Holders` is required; `New` rejects nil.

5. **Batched anchor read.** `AnchorReader` gains
`Anchors(ctx, addrs []entity.Address, props []string)
(map[entity.Address]AnchorView, error)`, one `ResolveHeadersErr` call; a miss is
absent. `AnchorView` gains `Title`, computed in `anchorReader` with
`meta.DisplayTitle` from the full redacted header (no extra read). `Anchor`
delegates to `Anchors` so the two share one face rule.

6. **Prime.** `Access` gains `Prime(ctx context.Context, anchorIDs
[]string) context.Context`. `requestAccess` calls `acl.PrimeLocalRoles`;
`openAccess` returns ctx. Inbox calls it once with every anchor id of the batch,
before any role check.

7. **Service.Inbox** (`internal/realtime/inbox.go`, new file):
   ```go
   type InboxRequest struct{ Limit int; Cursor string }
   type InboxView struct {
       Notice     string       `json:"notice"`
       Entries    []InboxEntry `json:"entries"`
       Count      int          `json:"count"`
       Truncated  bool         `json:"truncated"`
       Degraded   bool         `json:"degraded"`
       NextCursor string       `json:"next_cursor,omitempty"`
   }
   type InboxEntry struct {
       Kind        string       `json:"kind"` // "waiting"
       Channel     InboxChannel `json:"channel"`
       Session     InboxSession `json:"session"`
       Roles       []string     `json:"roles"` // waiting roles the caller holds, in st.WaitingFor() order
       Since       time.Time    `json:"since"`
       Transitions []string     `json:"transitions"`
   }
   type InboxChannel struct{ Anchor, EntityType, AnchorTitle, Type, Name, Title string; State channels.State }
   type InboxSession struct{ State, Label, Status string; WaitingFor []SessionWaitingView; Generation int64; EnteredAt time.Time }
   ```
Steps, one `begin` (one config snapshot, one Access, one clock):
1. Validate limit (1 to 200, default 50) and decode the cursor (400 on
error) before any read.
2. `requireCaller` (403 `no_principal`). Identity := `QueryIdentity()`;
invalid identity is also `no_principal`.
3. Degraded: answer `degraded: true`, no entries, no store call.
4. `w := snap.Config.Waiting()`. For each pair:
`Holders.Holding(…, Limit: MaxWaitingAnchors)`. Merge all refs into one set of
`AnchorKey`, deduplicated. When any pair reports `more`, or the merged set
exceeds `MaxWaitingAnchors`, keep the first `MaxWaitingAnchors` keys in (id,
face) order and set `truncated`. All counted refs passed the row gate and
redaction, so the flag is post-gate. There is no per-pair cap: a job identity
that owns every incident sees all of them up to the overall bound.
5. One `ListWaiting(States: w.States, Anchors, CreatorTypes: w.Creator,
CreatedBy: c.p.User when w.Creator is not empty)`. Skip the call when there are
no anchors and no creator types.
6. Candidate anchors: per channel, the address (`ID` for family types,
`ID@face` for face types). One `Anchors` call with the union of `roleProps` of
the involved types. Channels whose anchor is absent are dropped (row gate,
redaction, world, face).
7. `c.ctx = acc.Prime(c.ctx, anchorIDs)`.
8. Group by (anchor address, channel type); per group set `c.ct` and
`c.anchor` as `listType` does. Per channel set `c.head = &ch` and reset `c.ops =
opGates{}`: `heldRole` caches a creator answer in `c.ops.roles` whenever
`c.head` is set (session.go:274-287) and `mayPost` caches per message type in
`c.ops.post` (authz.go:433-443), so a per-group reset would carry one channel's
creator answer to the next. After the prime the reset costs no store call. Then
check `ct.Read` with `roleAdmits`; then `waitingView(state, ct.Roles,
waitingOf(c, &ch))`; keep the channel only when it answers `onMe`. `Roles`
lists, in `st.WaitingFor()` order, the waiting roles `waitingOf(...).held`
admits; `Transitions` is `c.fireable(c.postable())`. Creator matches also check
the org through `sameAuthor` here, because `created_by_user` carries no org.
9. Sort by (since, anchor id, face, type, name). `Count` = window size.
Page from the cursor key; `NextCursor` when more follow. The cursor is base64url
of `{v:1, since, id, face, type, name}`, with `since` in UTC RFC3339Nano at
microsecond precision (stored times are `channels.NormTime`); the decoded time
is passed through `NormTime` before comparison. It positions by comparison only,
so a cursor naming a hidden key reveals nothing. `ListWaiting`'s
`ErrTooManyRows` becomes `realtime.ErrInboxTooLarge`: REST 503 `inbox_too_large`
with a fixed title, MCP and CLI the same code in the error text; the server logs
user, tool and the candidate count. It is an accepted one-bit channel (see
Security). Nothing is cached; every call recomputes for its own principal.

10. **REST** (`internal/dataentry/inbox_handler.go`). `GET /api/v1/_inbox`
registered next to `_piles`. A consumer-side `inboxReader` interface
(`Inbox(ctx, realtime.InboxRequest) (realtime.InboxView, error)`) set by
`SetChannels`. Realtime off or channels not wired: 404 `not_found`, title
"Realtime is not enabled". Errors through the existing channel error map
(`channels_errors.go`) plus `invalid_cursor` 400 and `inbox_too_large` 503 (both
listed in the OpenAPI responses). Wire types in
`internal/apiwire/v1/channels.go` (`Inbox`, `InboxEntry`, `InboxChannel`,
`InboxSession`, reusing `ChannelSessionWaiting`); links built with the metamodel
plural. Times as RFC 3339 like the other channel wire times. OpenAPI path in
`internal/openapi/channels.go`, emitted only when realtime is on.

11. **MCP** (`internal/mcp/tools_inbox.go`). `InboxReader` consumer
interface added to `ChannelDeps` (optional field `Inbox`; nil registers no tool;
`UnavailableChannels` refuses it). Tool `list_inbox`, args `limit`, `cursor`.
Registered by `registerChannelTools` into `reads`; `watchOrder` gains
`toolListInbox`. In `parseTargets`, a `list_inbox` target gets
`watchChannel{all: true}`; `matches` returns true for every key, so any channel
change in the process replays it; store events already set `dirty`. A watch with
a second `list_inbox` target is a tool error (`parseTargets`), so one watch
replays one inbox. The channel poll (2 s, 5 s remote) would otherwise replay the
whole inbox at that rate even when nothing changed, so a `list_inbox` target
keeps its own poll interval: it replays on the poll at most every 15 s
(`inboxPollEvery`), and a replay that did not finish within `watchReplayTimeout`
doubles that interval for the rest of the call instead of being retried at the
next 500 ms tick. In-process wakes and store events still replay at once. A
cheap change token was considered: no backend has a channel-store generation
today (fs has none at all), and adding one is a store method plus a column on
three backends; the bounded interval gets the same protection without it. The
answer has no clock field (no due times, no reminders), so the digest moves only
when the window moves. Errors go through `channelHandler.failed`. The channel
instruction text gains: "To wait for sessions that wait on you, watch
list_inbox."

12. **CLI** (`internal/cli/channel_inbox.go`). `ChannelInboxCmd` under
`rela channel`, flags `--limit`, `--cursor`, `--json`. Adds `Inbox` to
`channelReader`. Errors through `channelFailed`.

13. **Docs.** `docs/realtime.md`: new "### Inbox" under Agents and a row
in the HTTP API table plus `invalid_cursor` and `inbox_too_large` in Errors; the
fs scale note (a creator waiting role makes each inbox call read the `meta.yaml`
of every channel of those types, and a watching agent repeats that at most every
15 s) next to the `ListSessionsDue` note. `docs/mcp-server.md`: `list_inbox`
row, watchable list (one `list_inbox` target per watch), agent loop.
`docs/cli-reference.md`: `rela channel inbox`. `docs/acl-security.md`: the inbox
gates and redacts before it counts; `truncated` is post-gate; the 20 000-row
ceiling is an accepted one-bit channel (Security below).

**Agent loop (docs):** call `list_inbox`; for each entry, read with
`read_session` or `read_channel` and post one of `transitions`; then `watch`
`{"tool":"list_inbox","args":{}}` with the cursor, up to 50 s per call, and
repeat. An entry leaves the inbox when the session moves, the role moves to
someone else, the channel closes, or access is lost.

**How identities hold waiting roles (docs, no code):**

| Caller | Named role (`from: entity.<prop>`) | Creator role |
| --- | --- | --- |
| Web app, remote MCP | The property holds the user entity id (`USR-7`), or the raw login without `user_entity_type` | Channels opened in the web app or remote MCP (`created_by.user` is the entity id) |
| Stdio `rela mcp`, `rela channel` as a person | The property holds the raw login (`jeroen`) | Channels opened over CLI or stdio |
| Job identity (`--run-as integration:triage-bot`) | A string or list property holds `integration:triage-bot`. A `ref` property to the user entity type cannot hold it | Channels the job opened (`create_channel`) |
| Any client on a read-only token | Never: waiting roles are held for update, and the client ceiling removes that capability, so the inbox is always empty (as `waiting_on_me` in `read_session`) | Never, same reason |

Remote MCP cannot carry a job identity (reserved names are refused from the
wire), so job agents use stdio. Stdio cannot run against sqlite while
rela-server runs, so the demo uses the fs or postgres backend.

**Alternatives rejected:**
- f1 channel-first (all waiting channels, then gate): cost grows with every
waiting session in the project, and a cap before the gate is an oracle.
- f3 stored entries at transition time: stale after a handoff by entity
edit; needs allow-all reads of holders.
- Ungated holder query plus `ResolveHeaders`: any cap on it is pre-gate.
- Full gated type scan plus a Go filter (no new visibility type): one
call, but it reads every row of the type; kept only as the no-pushdown fallback.
- A `MatchingRefs` method on the reader family: pushes `ScriptReader` and
`UnrestrictedReader` past the 20 exported-method cap and widens an interface
every reader consumer binds to.
- A per-pair cap of 2 000 holders: cuts a job identity's sessions by id
order for good, while ref-only rows are cheap.
- `json_extract`/`->>` on `created_by` at query time instead of a column:
no index, and pg's jsonb rejects some stored values.
- A `truncated` flag for the row ceiling: a pre-gate cap that would look
like a normal answer; a distinct error names it; see Security.
- Including the full `SessionView`: its timers, reminders and due times
would move the watch digest for changes the agent does not act on.

**Dependencies:** `internal/channels`, `internal/realtimecfg`,
`internal/realtime` (no new imports; stays within its arch-lint list),
`internal/visibility`, `internal/appbuild`, `internal/acl` (`PrimeLocalRoles`,
from appbuild only), `internal/dataentry`, `internal/apiwire/v1`,
`internal/openapi`, `internal/mcp`, `internal/cli`, `internal/sqlitedb`,
`internal/store/pgstore/migrations`.

**Files to modify:**
- `internal/realtimecfg/compiled.go` (+ `waiting_index.go`, test)
- `internal/channels/store.go`, `channels.go` (WaitingQuery, checks, limits)
- `internal/channels/channelstest/channelstest.go` (+ `waiting.go`)
- `internal/channels/sqlchannels/sql.go` (ListWaiting, insert, Raw, directive)
- `internal/channels/filechannels/ops.go`, `file.go` (ListWaiting, directive)
- `internal/sqlitedb/sqlitedb.go`, `migrate.go` (+ migrate test)
- `internal/store/pgstore/migrations/0034_channel_created_by_user.sql` (new)
- `internal/visibility/holders.go` (new; `HolderLookup`), `pushdown.go`
(shared helpers only) (+ `holders_test.go`)
- `internal/appbuild/appbuild.go` (`scriptReads` also returns the
`HolderLookup`; the reader family interface is unchanged), `realtime_service.go`
(holderFinder, Anchors, Title, Prime)
- `internal/realtime/access.go`, `service.go` (Deps), `inbox.go` (new),
`inbox_test.go` (new), `helpers_test.go` (fakes)
- `internal/dataentry/channels_handler.go` (SetChannels),
`inbox_handler.go` (new), `api_v1.go` (route), `channels_errors.go`,
`channels_wire.go`, `inbox_budget_test.go` (new), `channels_wire_golden_test.go`
and its testdata
- `internal/apiwire/v1/channels.go`
- `internal/openapi/channels.go` (+ test)
- `internal/mcp/tools_channels.go`, `tools_watch.go`, `tools_inbox.go` (new)
- `internal/cli/channel.go`, `channel_inbox.go` (new), wiring that builds
the stdio and CLI channel deps
- `docs/realtime.md`, `docs/mcp-server.md`, `docs/cli-reference.md`,
`docs/acl-security.md`
- Test fakes of `channels.Store` and `realtime.AnchorReader`/`Access`
across packages (compile-driven).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `limit` (REST query, MCP arg, CLI flag): integer 1 to 200; anything else
400 `invalid_request` / tool error, before any read.
- `cursor`: base64url JSON with `v == 1`, an RFC 3339 time, a valid entity
id, a valid face, a valid channel type and name; anything else 400
`invalid_cursor`. A valid cursor only positions the scan.
- Identity: from the bound principal and `QueryIdentity`, never from the
request. Empty, `unknown` or invalid identity is 403 `no_principal`, so an empty
identity can never match unset properties (`ErrNoCurrentUser` rationale).
- Config (`realtime.yaml`): already validated at load; the index reads
compiled values only.
- Stored data: holders, titles and labels are entity data; the answer
carries the `UntrustedNotice`; entries carry no message content.

**Security-Sensitive Operations:**
- Row gate and redaction: the holder query is gated by the composed ACL
read scope (row, face allowlist, world) on every verdict branch, and each row is
redacted and re-tested before it is returned or counted, so the overall anchor
bound and `truncated` never count a row whose role property is hidden from the
caller. The anchor batch gates and redacts again through `ResolveHeadersErr`;
`waitingView` re-checks the property on the redacted view, so a
`visible:`-hidden property matches nobody in the inbox exactly as in
`read_session`.
- Channel read role: `roleAdmits(ct.Read, read)` per channel, with the
creator source checked on the head, before an entry is kept.
- Aggregates gate before they fold (CLAUDE.md): `count`, `truncated` and
`next_cursor` are computed on the gated window. The anchor bound counts gated,
redacted refs only. AC 8 and 9 pin it.
- Row ceiling, an accepted one-bit channel: `ListWaiting` fails above
20 000 distinct candidate channels, answered as 503 `inbox_too_large`. The count
is taken before three gates, so it can include rows the caller cannot see:
channels whose `read:` role the caller fails, on anchors the caller can read;
channels the caller cannot read on an anchor they hold; and channels created by
the same user name in another org (`created_by_user` has no org; `sameAuthor`
checks it later). The candidate set is narrowed first: anchors are exact (id,
face) pairs, so channels on a face the caller cannot read are not counted, and
the creator branch covers only `CreatorTypes`. What leaks is one bit, "at least
20 000 such rows exist", never whether a given entity exists and never a
property value, so it is accepted like `MentionedBy`'s `Truncated` note. The
same bit can end an MCP `watch` on a hidden write (the answer turns into the
error), which is an exception to the watch rule that hidden writes cannot end a
wait. A user who may create channels can trigger it on purpose (200 per anchor
per type), which only denies their own and co-holders' inbox. A distinct,
documented code keeps the fault diagnosable; a generic 500 would hide nothing,
because the failure itself is the bit. This is recorded in
`docs/acl-security.md`.
- Per principal: no cache. The MCP watch digest is keyed by `ShareKey`
and the target, as for every watch target.
- Job identities: no new way to assert one. `--run-as` validation and the
wire refusal of reserved names stay as they are.
- Errors: store faults are logged with user, tool and op, never message
content; the client gets the generic channel failure text.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Package |
| --- | --- | --- |
| 1 | `TestCompiledWaiting` | realtimecfg |
| 2, 3 | `testListWaiting` (all four backends through their conformance runs) | channels/channelstest |
| 4 | `TestMigrate_v26ToV27ChannelCreator`, `TestMigration0034_ChannelCreatorBackfill`, ladder test | sqlitedb, pgstore |
| 5 | `TestHolderLookup_GatedLikeList`, `TestHolderLookup_PropsOnAllowAll`, `TestHolderLookup_HiddenPropertyNotCounted`, `TestHolderLookup_ZeroResultErrors` | visibility |
| 6, 7 | `TestInbox_MatchesWaitingOnMe` | realtime |
| 8 | `TestInbox_HiddenRowsChangeNothing` | realtime (fakes) and dataentry (real ACL) |
| 9 | `TestInbox_TruncatedIsPostGate` | realtime |
| 10 | `TestInbox_Paging`, `TestInbox_CursorForged` | realtime |
| 11 | `TestInboxBudget_10vs50` | dataentry |
| 12 | `TestInbox_DegradedAndOff`, `TestInboxHandler_Errors` | realtime, dataentry |
| 13 | `TestInboxHandler_Shape`, `TestChannelWireGolden`, `TestOpenAPI_EveryOperationReachesAHandler` | dataentry, openapi |
| 14 | `TestListInbox_Answer`, `TestWatch_ListInboxWakes`, `TestWatch_ListInboxNoWakeOnChat`, `TestWatch_ListInboxDigestPerCaller`, `TestWatch_ListInboxOneTarget`, `TestWatch_ListInboxSlowReplayDoesNotSpin` | mcp |
| 15 | `TestChannelInbox_Text`, `TestChannelInbox_JSON` | cli |
| 16 | `TestInbox_JobIdentity` | cli (stdio wiring) |

Integration: the dataentry tests run the real router, real `acl.yaml`, fsstore
and filechannels; the budget test also runs on sqlite under the `sqlite` tag
where the channel budget tests already do. The pg conformance run covers
`ListWaiting` and migration 0034 through the schema-pinned DSN. Holder pushdown
parity: one table test runs the same seeded project through
`HolderLookup.Matching` on memstore, fsstore, sqlite and pg (where tagged) and
compares with the Go filter. Its rows include a list value on a property
declared scalar, and a faced entity type whose second-ranked face names the
caller and whose first does not (family-anchored pair).

**Edge cases:**
- Initial state waits and the session never moved: matched through `""`.
- Unstored cardinality-one channel whose initial state waits: no entry
(`waiting_on_me` needs a stored channel).
- Closed or archived channel in a waiting state: no entry.
- Final, `working`, `cancelled`, `failed` and stranded states: no entry.
- Config reload between requests that removes a state: stored rows in it
are stranded and drop out; `States` no longer lists them.
- Degraded config: `degraded: true`, empty, no reads.
- Property holds a list with the caller among others; a list with a
non-string element; an empty string; a property absent on the served face.
- Face-anchored type: role prop on the face; the family face of the same
id names someone else (and the reverse).
- Two channel types share one property on one entity type: one holder
query serves both.
- A user who is both holder and creator of the same channel: one entry,
`roles` lists both.
- Creator in another org with the same user name: no entry.
- Anchor renamed (`Rekey`) after the channel was created: still found.
- Anchor deleted (channels tombstoned) and the id reused: old channels are
tombstoned and never match.
- Anchor bound reached exactly (bound refs, no more): `truncated` false.
- Many pairs whose refs together exceed `MaxWaitingAnchors`: clamped in
realtime, `truncated` true, no `ErrInvalidKey` from the store.
- A channel the caller created on an anchor they hold: returned once.
- Two channels on one anchor created by different users, with a creator
transition role: each entry's `transitions` matches its own `read_session`.
- `since` values equal, or 1 µs apart, across a page boundary.
- A `list_inbox` replay slower than `watchReplayTimeout`: backs off.
- Zero entries: `entries: []`, `count: 0`, no cursor.
- `limit=1`, `limit=200`, `limit=201`, `limit=0`, `limit=-1`, `limit=abc`.
- Concurrent post during a watch: the subscription wakes; a missed wake is
caught by the poll.
- Cursor whose anchor has since become hidden: paging continues from the
next visible key.
- sqlite variable limit: 1 200 anchor ids in one `ListWaiting` (chunking).

**Negative Tests:**
- Bad `limit`, malformed `cursor` (not base64, wrong version, bad id): 400.
- No principal under a declarative policy: 403 `no_principal`.
- `unknown` principal: 403 `no_principal`.
- Realtime off: REST 404, MCP and CLI off text.
- POST/PUT on `/api/v1/_inbox`: 405 with `Allow: GET`.
- Store fault in `ListWaiting` or the holder query: 500 generic, nothing
partial served; logged without content.
- `ListWaiting` with empty `States`, invalid type, too many anchors, or
neither anchors nor creator: `ErrInvalidKey`.
- Row ceiling exceeded: 503 `inbox_too_large`, not a partial list.
- `HolderLookup` with a zero `ReadQueryResult`: error (never AllowAll).
- Refusing `HolderLookup`: `ErrReaderUnavailable`, answered 500.
- A second `list_inbox` target in one `watch`: tool error.
- pg migration over a `\u0000` row and an invalid-JSON row: succeeds.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Pushdown and Go rule disagree** (store equality stricter than
`is_current_user`): an entry would be missing. Mitigation: both are exact byte
equality; the pushed predicate is non-scalar so value shape decides, as in Go;
each row is re-tested in Go after redaction; the parity table test runs on every
backend.
- **fs cost**: with a creator waiting role, every inbox call walks every
channel directory of the creator types. Mitigation: the walk reads only
`meta.yaml` of `CreatorTypes`; without creator roles only the holder anchors'
directories are read; a watching agent replays on the poll at most every 15 s,
with backoff. Document the scale limit next to the existing `ListSessionsDue`
note; postgres and sqlite use the index.
- **Large holder sets** (a job identity owns every incident): no per-pair
cap; the overall bound is 20 000 anchors in (id, face) order, set in store with
`OrderBy` and `Limit` so the subset is stable between calls and the watch digest
does not flap. Past the bound `truncated` is set.
- **pg backfill** on a large channels table: one `UPDATE` through a
per-row function in the migration transaction. Channels tables are small today.
Rows the function cannot parse keep `''` and fail closed.
- **Identity per transport**: a holder written by the web app (`USR-7`)
does not match the same person over stdio (`jeroen`). This is today's rule for
every role; the docs table states it, and the demo uses job identities with
string properties.
- **Interface growth**: `channels.Store`, `AnchorReader` and `Access` each
gain a method; every fake must follow. The reader family does not grow.
Mitigation: compile-driven; fakes in `helpers_test.go` and `countingChannels`.
- **Watch load**: list_inbox replays on every channel change in the
process and on the poll at most every 15 s. One `list_inbox` target per watch, 4
watches per identity and 32 per server bound it; a slow replay backs off instead
of spinning; a replay costs the budget-pinned calls only.
- **Creator cache across channels**: transitions computed per channel
could reuse another channel's creator answer. Mitigation: `c.ops` is reset per
channel; AC 6 compares each entry with `read_session`.

**Effort:** L. The research estimated M; the store method on three backends with
two schema migrations, and the new gated visibility query, add a day each.

**Design decisions (taken as recommended):**
1. Order: oldest wait first.
2. Holder lookup: a new gated query in `internal/visibility`, as a
separate `HolderLookup` type after design review (not a reader method).
3. Row ceiling: fail at 20 000 candidate channels with 503
`inbox_too_large`, an accepted one-bit channel recorded in
`docs/acl-security.md`.
4. `rela channel inbox` is in this slice.
5. `transitions` are in each entry.
6. Realtime off on REST: 404.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] docs/cli-reference.md - `rela channel inbox`
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: SPA is TKT-9YRRM5)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the gantt rule as written)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] docs/realtime.md - Inbox section, HTTP API row, errors, identity table, agent loop
- [x] docs/mcp-server.md - `list_inbox`, watchable list, agent loop
- [x] docs/acl-security.md - inbox gating paragraph and the accepted `inbox_too_large` one-bit channel

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 16 findings (1 critical, 6 significant, 7 minor, 2
nit), all folded into this plan; resolutions per finding in
`.ignored/dr-zz4pnx-res.txt`. Two are adopted in part: finding 5 uses a bounded
poll interval with backoff instead of a change token, and finding 10 keeps
chunked SQL on postgres instead of `= ANY($n)`.
