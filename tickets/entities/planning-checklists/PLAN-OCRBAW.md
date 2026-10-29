---
id: PLAN-OCRBAW
type: planning-checklist
title: 'Planning: Agent activity indicator on channels and boards'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** The agent-runner PoC
(`rela-agent-runner-poc/rela-mission/project/realtime.yaml`) runs one `run`
channel per ticket. A person sees what needs their action in the inbox. They
cannot see which tickets an agent is busy on, unless they open each channel. Two
things are missing:

- Board and list reads carry no session state. A kanban card does not know
that its anchored session is in a state such as `planning` or `working`. The PoC
works around this by writing the ticket's `status` property from session
outputs. That is a copy of the session, kept in step by hand.
- An agent has no way to say "working on it" in a channel that a person sees
live. The PoC runner talks to rela through stdio `rela mcp --run-as
integration:runner`, a separate process from `rela-server`. Today:
  - Typing (`Service.Typing`, `internal/realtime/typing.go`) lives in a
process-local `typingStore`. MCP has no typing tool, and `read_channel` says
`typing_now` is empty over stdio.
  - Operator signals (`persist: ephemeral`) are refused over stdio
(`ChannelDeps.RefuseSignals`, set in `internal/cli/mcp_wiring.go`).
  - A post from stdio sends no live frame to `rela-server` (PoC README,
"Limits"; TKT-YK30D1).

So nothing ephemeral crosses processes today. What does cross is stored state:
the session row in the channel store, and graph writes, which `rela-server`
learns of through the fs watcher or the pg listener.

The session already says what "busy" means. Each state has a `status:`
(`realtimecfg.SessionStatus`, `internal/realtimecfg/waiting.go`): `working`,
`input_required`, `completed`, `cancelled` or `failed`. A non-final state
without `waiting_for:` defaults to `working`. In the PoC, `planning` and
`working` declare `status: working`. Agents already read this status through
`read_session`.

**Terms used below.**

- *Working state*: a session state whose status is `working`.
- *Busy channel*: an open, live channel whose current session state is a
working state, and which the caller may read. The current state of a session
that never moved is the initial state, entered when the channel was created
(`currentSession`, `internal/realtime/session.go`).
- *Busy type*: the channel type a board asks about, named by the card field
`busy:` in `data-entry.yaml`.
- *Busy entry*: one busy channel as a row carries it in `_busy`: channel
type, type label, name, state, state label and the time the session entered the
state.
- *Activity line*: a line in the channel's session strip that says the
session is working.

**Scope (in):**

- `realtimecfg.Session.WorkingStates()`: the stored state names that read
`working`, with `""` when the initial state is working.
- A batch read in `internal/realtime`: `BusyReader.Busy`, which answers the
busy channels of one busy type on a page of anchors. It uses one
`channels.Store.ListWaiting` call, one batch anchor read and one role prime,
whatever the number of rows. No store behavior changes on any backend; only the
`WaitingQuery` godoc is reworded for a second consumer.
- List API: `?busy=<channel type>` on `GET /api/v1/{plural}` adds `_busy` to
each row (an `apiwire` field in the `Activity` group).
- Live: `busy` is an accepted live list key, so the board's live
subscription keeps working. A full live round (a graph write) replays it, as
today. A channel change alone does not select it.
- `data-entry.yaml`: a kanban card field `busy: <channel type>`, checked
against `realtime.yaml` through `ChannelSchema` at server load, on reload, by
`rela validate` and by Configure, with or without a `channels:` block.
- SPA: the board asks for `busy` when a card field names it. The card shows
an animated busy indicator with the state label and how long the session has
been in the state, plus an accessible text. The channel session strip shows an
activity line while the status is `working`.
- MCP: `read_session` and `post_message` descriptions say that a working
state shows the anchor as busy in the web app, so an agent signals "working on
it" by moving the session into a working state.
- Docs: `docs/realtime.md`, `docs/data-entry.md`,
`docs/data-entry/api-reference.md`, including a session timer example that moves
a stalled run out of a working state.

**Scope (out):**

- Follow-up "busy board live refresh on session moves": selecting a busy
board subscription in a topic round when a channel of its busy type changes. It
needs the page's anchors recorded on the subscription, a no-fallback rule like
the inbox's and a syncing recheck, so that a change the subscriber cannot see
never frames its board. Until then the board refreshes live on graph writes
only.
- Follow-up "busy column on list rows": a `busy:` column kind for
`EntityList`. The server side ships here, because the list endpoint is shared.
See Q4.
- An ephemeral "working on it" signal from an agent: an MCP `typing` tool, an
activity signal over stdio, or `typing_now` over stdio. See "Cross-process
limits" and Q5.
- An MCP or CLI read of busy state over entity lists. Agents read
`read_session` per channel today.
- Automatic stall detection, such as a lease check. The docs show a session
timer as the pattern for now. See Q3.
- Showing which agent is busy. The session does not record a worker.
- A new session status (such as `idle`) or a per-state `busy:` key.
- Busy state on calendars, gantts, dashboards and views (`_views`).

**Cross-process limits.** A session move from stdio `rela mcp` is stored at
once, so the next board or channel read shows it. `rela-server` sends no live
frame for the move itself. When the move has graph outputs, as every PoC move
does, the graph write starts a full live round (fs watcher or pg listener), and
that round replays the board. So the PoC board updates live today. A move
without outputs shows on the next reload.

TKT-YK30D1 adds a pg_notify poke for channel changes, so it helps only on the
postgres backend. On the file and SQLite backends, which the PoC uses, a
cross-process move without graph outputs stays reload-only even after
TKT-YK30D1. Closing that gap there needs a stored activity entry with a TTL or a
watch on the fs channel directory by `rela-server`. No ephemeral signal crosses
processes on any backend today, so an MCP typing tool would do nothing for the
PoC runner. It is not added here.

**Acceptance Criteria:**

1. **Working states come from config.** For the PoC session,
`WorkingStates()` returns `["planning", "working"]`. A session whose initial
state is working also lists `""`. States with status `input_required`,
`completed`, `cancelled` or `failed` are never listed. A non-final state without
`waiting_for:` and without `status:` is listed, because it reads `working`.
Test: `internal/realtimecfg/session_test.go` `TestSession_WorkingStates` (table:
PoC shape, working initial, default status, all final).
2. **The batch read answers busy channels only.** `BusyReader.Busy(ctx,
"run", anchors)` returns, per anchor address, the busy entries of type `run`,
sorted by name. Each entry is built from the current session (`currentSession`),
so a session that never moved, with a working initial state, gives the initial
state's name and label and `since` equal to the channel's `CreatedAt`. Absent
from the answer: a channel in `asking`; a closed or archived channel in
`working`; a stranded session (stored state no longer declared); a session that
never moved when the initial state is not working; an anchor the caller may not
read; a channel whose `read:` role the caller does not hold (an
`entity.<property>` read role that does not name the caller). For a
face-anchored type the query uses the row's face; for a family type it uses face
`""`. An entry carries type, type label, name, state, state label and `since`
(the session's `EnteredAt`). Test: `internal/realtime/busy_test.go`
`TestBusy_States` (including a never-moved case that asserts state name, label
and `since == CreatedAt`), `TestBusy_ReadRole`, `TestBusy_Anchor` (face and
family), `TestBusy_ManyChannels` (cardinality many, two busy, sorted).
3. **The batch read never fails a list for config or identity reasons.**
Realtime off (`ErrRealtimeOff`), a request without a principal (the
`noPrincipal` denial from `begin`), a degraded config, an unknown channel type,
a type without a session, a type with no working state and a type that does not
attach to the anchor's type all return an empty answer and `ok=false`, never an
error, and log nothing. A store error is returned as an error. Test:
`TestBusy_NotServed` (table, with a no-principal case) in `busy_test.go`.
4. **The list carries `_busy`.** `GET /api/v1/tickets?busy=run` gives each
row `_busy`: `[]` when no channel of the row is busy, else the entries. Without
the parameter, or when the read returned `ok=false`, `_busy` is absent. A failed
busy read is logged and leaves `_busy` absent on every row; the list still
answers 200. The parameter is read with `queryGet`, as `comment_counts` is: the
first value counts, and an empty value is treated as absent. Test:
`internal/dataentry/busy_rows_test.go` `TestListBusy_Rows`,
`TestListBusy_NotRequested`, `TestListBusy_NotServed`,
`TestListBusy_StoreErrorKeepsList`.
5. **Busy state is gated like a channel read, per principal.** A row the
caller may not read never appears (the list row gate). A readable row whose
channel the caller may not read gets `[]`, the same answer as "not busy". The
answer is computed per request and never cached across principals. Test:
`TestListBusy_HiddenRow` in `busy_rows_test.go`, and
`TestListBusy_TwoPrincipals`: alice holds the `run` read role and bob does not,
both may read the ticket, and both GET the same URL; alice's row has one entry
and bob's row has `[]`. `TestLive_BusyListPerPrincipal` in `live_test.go`: alice
and bob subscribe to the same busy list URL, a graph write starts a full round,
and their replays have different hashes and frames.
6. **Constant cost per page.** A list of 10 rows and one of 50 rows, all
with a busy channel, make the same entity-store reads and the same channel-store
calls. `ListWaiting` runs exactly once per page, with the page's anchors. The
pages include a face-anchored type with rows on two faces and rows whose channel
read is denied, so a per-row fallback read would show in the counts. Test:
`internal/dataentry/channels_budget_test.go`
`TestChannelsBudget_ListBusy10vs50`, using `countingChannels` and
`storetest.Counting`.
7. **The live board keeps working.** `busy` is an accepted live list key. A
live list subscription with `busy=run` is replayed by a full round (a graph
write), and the replay carries `_busy`. A topic round started by a channel
change alone does not select it, as for any list today. Test: `livepath_test.go`
case for the key; `internal/dataentry/live_test.go` `TestLive_BusyListFullRound`
and `TestLive_BusyListIgnoresChannelRound`.
8. **The card field loads and is checked.** `card.fields: [{ busy: run }]` on
a ticket board loads. Refused, at `kanban "<id>": card.fields[i]`: `busy` with
`property`, `relation`, `comments` or `display`; a second `busy` field on one
card; `busy` on a calendar or gantt field (kanban only). Refused through
`ValidateChannels`: a channel type `realtime.yaml` does not declare; a type
without a session; a type that does not attach to the board's entity type. A
warning: a busy type whose initial state reads `working` by default (no explicit
`status:`), because every new channel of that type would show busy. With
realtime off, a `busy` field is a warning, as a `channels:` block is. All of
this holds without a `channels:` block, at server boot, on reload, in `rela
validate` and in Configure. Test: `internal/dataentryconfig/validate_test.go`
`TestValidateKanbanBusyField`; `channels_test.go` `TestValidateChannels_Busy`
(with no `channels:` block, and the initial-working warning); calendar and gantt
cases in `validate_calendars_test.go` and `validate_gantts_test.go`;
`internal/cli/validate_channels_test.go`: `rela validate` rejects `busy: nope`
in a project without a `channels:` block; `cmd/rela-server/configure_test.go`: a
save with `busy: nope` and no `channels:` block is refused;
`internal/dataentry/realtime_wiring_test.go`: `SetRealtime` with realtime off
logs the warning for a busy field.
9. **The card shows the indicator.** A board whose card has `busy: run`
sends `busy=run`. A card whose row has one `_busy` entry shows `RlBusyIndicator`
with the visible text of the state label and the time in the state ("Planning ·
3h"). Its accessible name is the type label, the state and the time ("Agent run:
Planning, for 3 hours"). The visible text is `aria-hidden`, so a screen reader
reads the name once. Two or more entries show the first label plus "+N", and the
accessible name lists every entry. A card with `[]` or no `_busy` shows nothing.
The indicator has no `role="status"` and no `aria-live`. Test:
`frontend/src/views/KanbanView.cardBusy.test.ts`: param sent; indicator shown
with the visible text; accessible name asserted (not the presence of two spans);
"+1" case with both entries in the name; nothing for `[]` and absent; no `role`
or `aria-live` attribute on the indicator. `RlBusyIndicator.test.ts`: the dots
and the static icon are both rendered with `aria-hidden="true"`, and the
component source (`?raw`) hides the dots and stops the animation inside `@media
(prefers-reduced-motion: reduce)`. `RlBusyIndicator.stories.ts` with default,
long label, several entries and reduced-motion stories.
10. **The channel shows an activity line.** In the normal status, a session
with status `working` shows "Working…" (en) / "Bezig…" (nl) with the state
label. The line renders inside the strip's existing `role="status"` element, and
the strip has no second element with `role="status"` or `aria-live`. It shows
nothing in the paused, final, stranded or loading status, and nothing for
`input_required`. Test: `frontend/src/components/entity/ChannelsPanel.test.ts`
cases "activity line while working" (the text is a descendant of the one
`[role="status"]` element, and the count of live regions is one), "no activity
line when paused" and "no activity line for input_required";
`RlChannelSession.stories.ts` story "Working".
11. **Agents learn how to signal.** The `read_session` description says that
a state with status `working` shows the anchor as busy on boards that ask for
it, and the `post_message` description says to move the session into a working
state when work starts. Test: `internal/mcp` channel tool description test,
substring case.
12. **Docs.** `docs/realtime.md` has a section "Busy indicator" with the PoC
example, the initial-working warning, a working-state timer that moves a stalled
run to a blocked state, and the cross-process limits per backend.
`docs/data-entry.md` documents `busy:` on card fields. The API reference
documents `?busy=` and `_busy`. Test: reviewed. The YAML examples load in
`docs_example_test.go` when that test covers the file.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach reuses `status: working`, `ListWaiting` and the comment-count row extra; the open choices are recorded below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `channels.Store.ListWaiting` with `WaitingQuery{States, Anchors}`
(`internal/channels/waiting.go`): open, live channels of listed types in listed
stored session states, on exact anchors. It is defined on all four backends and
pinned by `channelstest`. The inbox uses it (`internal/realtime/inbox.go`
`listWaiting`).
  - The inbox's per-page gate: `inboxRun.anchors` (one batch anchor read),
`prime` (`acc.Prime` once), `item` (`roleAdmits` on `ct.Read` per channel, from
primed answers). `busyRun` follows it.
  - `currentSession` (`internal/realtime/session.go`): an unset session
reads as the initial state, entered at `CreatedAt`.
  - `buildWaiting` (`internal/realtimecfg/waiting_index.go`): the stored
states per type, with `""` for an initial state.
  - `serveListRowExtras` and `serveCommentCounts`
(`internal/dataentry/comment_counts.go`): an opt-in row extra, served after the
row gate, one batch per page, logged and omitted on failure. `queryGet` reads
its parameter.
  - `KanbanCardField.Comments`: a card field that stands alone and opts the
board into a row extra.
  - `ValidateChannels` and `ChannelSchema`
(`internal/dataentryconfig/channels.go`): `data-entry.yaml` checked against
`realtime.yaml` without importing it. `projectsetup.CheckChannels` runs it for
`rela validate` and Configure.
  - `InboxView`'s plain slices with `omitzero` to tell absent from `[]`.
  - `LeaseBadge.vue`: a card badge with a visually hidden status text.
  - `RlChannelSession.vue`: the `role="status"` waiting region.
  - Session timers (`after:` with `since: state`): a state can move on
after a period, which is the stall pattern the docs show.
- Reference implementations: GitHub's "running" dot on a pull request
check, Linear's "agent working" chip, Slack's typing line. All show a small
indicator and leave details to the detail page. The A2A protocol's task state
`working` is what `SessionStatus` mirrors.
- rela prior art: TKT-WA25G2 (comment counts on cards), TKT-4YPG4Q (lease
badge), TKT-HJ5Z9Y (live board), TKT-ZU7P13 (topic rounds), the inbox (Inbox 1
to 5), RES-B0MJAZ (multi-instance realtime), and the visibility and
collection-reads rules in CLAUDE.md.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config (`internal/realtimecfg`).**
   - `session.go`: `(s *Session) WorkingStates() []string`: the names of
the states with `Status == StatusWorking`, in declaration order, plus `""` when
the initial state is one. No new YAML key.
   - `waiting.go`: record whether a state's `status:` was written or
defaulted, exposed as `SessionState.StatusDeclared() bool`, for the
initial-working warning.
2. **Batch read (`internal/realtime/busy.go`, new).** `Service` is at its
plimsoll cap (42 methods), so the entry point is its own type, and the per-call
state sits in an unexported `busyRun{s, c}`, as `inboxRun` does:
   - `NewBusyReader(s *Service) (*BusyReader, error)`; nil is rejected.
   - `Busy(ctx, channelType string, anchors []BusyAnchor)
(map[entity.Address][]BusyView, bool, error)`. `BusyAnchor{Ref entity.Ref; Type
string}` is exported because dataentry must pass each row's type. `BusyView` is
`{Type, TypeLabel, Name, State, Label string; Since time.Time}`.
   - Steps: `s.begin(ctx)` for the snapshot and access. `ErrRealtimeOff` and
the no-principal denial return `ok=false` with no error. Also return `ok=false`
when the snapshot is degraded, the type is unknown or has no session, or
`WorkingStates()` is empty. Keep only anchors whose entity type is in
`ct.AttachTo`. Map each to a `channels.Anchor`: face `""` for `AnchorFamily`,
the ref's face for `AnchorFace`. Deduplicate. With no anchor left, return an
empty answer without a store call.
   - One `store.ListWaiting(WaitingQuery{States: {type: working}, Anchors:
anchors, MaxRows: ceiling})`. The ceiling is `len(anchors)` for cardinality one
and `len(anchors) * realtimelimits.MaxChannelsPerAnchor` for many, clamped to
`channels.MaxWaitingRows`. The hard constant, not the operator's
`ChannelsPerEntity`, so that no stored channel set can reach the ceiling and its
count, taken before the read gate, can never decide whether `_busy` appears.
With `per_page` at most 100, the product is at most 20000, which is
`MaxWaitingRows`. `ErrTooManyRows` is returned as an error (the caller logs and
omits).
   - One `s.anchors.Anchors(ctx, addrs, roleProps(ct))` for the channels
found, then one `c.acc.Prime(ctx, ids)`. A channel whose anchor is not in the
answer is dropped.
   - Per channel: set `c.ct, c.anchor, c.key, c.head, c.ops` as
`inboxRun.item` does, and keep it when `roleAdmits(c, ch, ct.Read,
CapabilityRead)` holds. Then read `currentSession(ct.Session(), *ch)`, drop it
when stranded or when its state's status is not `working`, and build the entry
from it: the state, the label from the current config and `EnteredAt` as
`since`. `TypeLabel` is `ct.Label`.
   - The godoc states the cost: one channel-store call, one anchor batch,
one prime. It also says that the file backend's `ListWaiting` reads one face
directory and each `meta.yaml` per anchor, so its I/O grows with the page even
though the call count does not.
   - `internal/channels/waiting.go`: reword the `WaitingQuery` godoc so it
names both consumers (the inbox and the busy read) instead of only "the channels
an inbox looks at". No behavior change.
3. **Wire (`internal/apiwire/v1/responses.go`).** `Activity` gains
`Busy []BusyChannel json:"_busy,omitzero"`, as `InboxView` does. A nil slice is
absent ("not served"); an empty slice is `[]` ("none busy that you may see").
`BusyChannel` is `{type, type_label?, name?, state, label?, since}`.
4. **List row extra (`internal/dataentry/busy_rows.go`, new).**
   - `busyParam = "busy"`. `wantBusy(query)` returns
`queryGet(query, busyParam)`, the first value, as `comment_counts` is read.
   - `serveListRowExtras` calls `serveRowBusy(ctx, a.channels, typ, rows,
data)` when the parameter is set and channels are wired. A function, so
`channelsHandler` gains no method.
   - `serveRowBusy` builds one `BusyAnchor` per row (its ref and type),
calls `Busy` once, and sets `data[i].Busy` to a non-nil slice for every row when
`ok`. An error is logged with `slog.WarnContext` and leaves every row without
`_busy`.
   - The `channelsHandler` gains a field `busy busyReader`, a consumer-side
interface with the one method. `SetChannels` builds it with
`realtime.NewBusyReader(svc)`.
5. **Live (`internal/dataentry/livepath.go`, `requeststats.go`).** Add
`busy` to `liveListKeys` and to the query allowlist in `requeststats.go`.
Nothing else in the live path changes: `topicProbe`, `roundFilter`, `collect`
and `fallback` stay as they are, so a channel change never selects or frames a
list subscription. The follow-up "busy board live refresh on session moves" owns
that.
6. **Config (`internal/dataentryconfig`).**
   - `config.go`: `KanbanCardField` gains `Busy string yaml:"busy"`.
`DisplayLabel` returns the channel type name when `Label` is empty, as a
property field returns its property name (labels are authored, not derived;
DEC-6C1NAA). `show_label` works as on other fields: default true, so the label
renders before the indicator; `show_label: false` shows the indicator alone.
   - `validate.go`: the card checks of AC 8. `refuseKanbanOnlyFields`
refuses `busy` on calendars and gantts (`validate_calendars.go`,
`validate_gantts.go`).
   - `channels.go`: `ChannelSchema` gains `BusyFacts(channelType)
(BusyFacts, bool)`, with `AttachTo []string`, `HasSession bool` and
`InitialDefaultWorking bool`. `ValidateChannels` walks the kanban card fields
too. The early return for an empty `channels:` block becomes "no `channels:`
block and no busy field", so busy fields are checked, and the realtime-off
warning is given, without a `channels:` block.
   - `internal/appbuild/realtime_channels.go` implements `BusyFacts`.
7. **Validate and Configure paths.**
   - `internal/projectsetup/validate.go`: `channelsSection` returns the
config when it has a `channels:` block or a kanban busy field. `HasChannels`
becomes `HasRealtimePresentation`, with its godoc.
   - `internal/cli/validate.go`: calls `HasRealtimePresentation`, so
`rela validate` checks busy fields in a project without `channels:`.
   - `cmd/rela-server/configure.go`: already calls `CheckChannels` on every
save, so the `channelsSection` change covers it. Listed for its test.
   - Server boot and reload: `dataentrywire.Wire` calls `SetRealtime`
whenever the realtime holder exists, including with realtime off, and
`watcher.go` calls `checkChannelsPresentation` on reload. Both run
`ValidateChannels`, so the moved early return makes them check busy fields and
log the realtime-off warning. `realtime_wiring.go` gets a godoc touch-up that
names busy fields.
8. **MCP (`internal/mcp/tools_channels.go`).** The description sentences
of AC 11. No new tool.
9. **SPA: components (`rela-components`).**
   - New `components/feedback/RlBusyIndicator.vue`. Props `entries` (type
label, state label, since). It renders three dots, a static icon, a visible text
and a visually hidden accessible text (`rl-visually-hidden`, as `LeaseBadge.vue`
does). The dots, the icon and the visible text are `aria-hidden`. The visible
text is the first state label, the time in the state ("3h") and "+N" for more
entries. The hidden text names every entry with its type label and time. The
time is computed in the browser from the absolute `since`, so the server's live
hash does not change as time passes; it is recomputed on render.
   - Motion follows the repo convention (`RlBoardCard.vue`): the dots
animate by default, and `@media (prefers-reduced-motion: reduce)` hides the dots
and shows the static icon, so no row of still dots reads as truncated text.
Stories: default, long label, several entries, reduced motion.
   - `RlChannelSession.vue`: in the normal status, when `session.status ===
'working'`, render the activity line inside the existing `role="status"` div,
with `RlBusyIndicator`. Messages `sessionWorking` in `useMessages.ts` and
`messages.nl.ts`. A story "Working".
10. **SPA: app.**
    - `src/types/config.ts`: `KanbanCardField.busy?: string`;
`cardFieldLabel` returns `field.busy` when no label is set.
    - `src/types/entity.ts`: `Entity._busy?: BusyChannel[]`;
`ListParams.busy?: string`.
    - `src/views/KanbanView.vue`: `busyType` computed from the first card
field with `busy`; `boardParams` sets `busy`. `resolvedCardFields` pushes a busy
field when `_busy` has entries; the card renders `RlBusyIndicator` for it
through `CardFieldList.vue`.
    - `src/components/common/CardFieldList.vue`: a `busy` branch.
    - The live subscription already sends the board's params, so `busy`
travels with it.

**Alternatives considered:**

- *An ephemeral activity signal with a TTL.* Rejected for this slice: it is
process local, so it does nothing for a stdio agent, and it duplicates what the
session status already says. It fits as a later "typing" over MCP, once signals
cross processes.
- *A per-state `busy: true` key in `realtime.yaml`.* Rejected: `status:`
already declares it, and agents read the same status. Two keys could disagree.
- *A `session: run` card field with a display mode, instead of `busy: run`.*
It would let a later ticket show `input_required` on cards too, without a second
key. Rejected for now: `busy:` matches the standalone `comments: true` field and
the type-keyed `channels:` block, and this ticket shows only working states. If
a later ticket wants more session states on cards, it adds `session:` and keeps
`busy:` as a short form.
- *Busy derived from a lease held by a job identity.* Rejected as the main
signal: a lease is an advisory edit claim, and it crosses processes but says
nothing about the session. It stays a candidate for stall detection (Q3).
- *Copy the session state into an entity property (the PoC's `status`
outputs).* Rejected as the mechanism: it needs outputs on every move and write
access for `run_as`, and it is a second source of truth. Projects may keep doing
it.
- *A new channel-store method (`SessionsOn(anchors)`).* Rejected:
`ListWaiting` already selects by anchor and stored state on all four backends,
with conformance tests.
- *Per-row `Service.Session` calls.* Rejected: a per-row lookup on a list
path (collection-reads rule).
- *Serve every session-bearing type with `busy=1`.* Rejected: the card
names one type, and a named type lets the load check it.
- *A method on `Service`.* Rejected: it is at its plimsoll cap.
- *Live board wake by channel type in this ticket.* Rejected: selecting by
type replays every busy board on every message of that type, including channels
the subscriber cannot see, and a round fallback would then frame boards for
changes they cannot see. Doing it right needs per-page anchors on the
subscription. Moved to a follow-up.

**Dependencies:** `internal/channels` (godoc only), `internal/realtimecfg`,
`internal/realtime`, `internal/apiwire/v1`, `internal/dataentry`,
`internal/dataentryconfig`, `internal/projectsetup`, `internal/cli`,
`cmd/rela-server`, `internal/appbuild`, `internal/mcp`. Frontend: no new
packages. Not dependent on TKT-YK30D1.

**Files to modify:**

- `internal/realtimecfg/session.go`, `waiting.go`.
- `internal/realtime/busy.go` (new).
- `internal/channels/waiting.go` (godoc only).
- `internal/apiwire/v1/responses.go`.
- `internal/dataentry/busy_rows.go` (new), `comment_counts.go`
(`serveListRowExtras`), `channels_handler.go` (`SetChannels`, field),
`livepath.go`, `requeststats.go`, `realtime_wiring.go` (godoc).
- `internal/dataentryconfig/config.go`, `validate.go`,
`validate_calendars.go`, `validate_gantts.go`, `channels.go`.
- `internal/projectsetup/validate.go` (`channelsSection`,
`HasChannels` renamed to `HasRealtimePresentation`).
- `internal/cli/validate.go`.
- `cmd/rela-server/configure.go` (covered by `channelsSection`; test only
unless the call needs a comment).
- `internal/appbuild/realtime_channels.go`.
- `internal/mcp/tools_channels.go`.
- `frontend/packages/rela-components/src/components/feedback/RlBusyIndicator.vue`,
`.stories.ts` and `.test.ts` (new), `components/channel/RlChannelSession.vue`,
`RlChannelSession.stories.ts`, `composables/useMessages.ts`,
`composables/messages.nl.ts`, the package index export.
- `frontend/src/types/config.ts`, `frontend/src/types/entity.ts`,
`frontend/src/views/KanbanView.vue`,
`frontend/src/components/common/CardFieldList.vue`.
- Tests: `internal/realtimecfg/session_test.go`,
`internal/realtime/busy_test.go` (new), `internal/dataentry/busy_rows_test.go`
(new), `channels_budget_test.go`, `live_test.go`, `livepath_test.go`,
`realtime_wiring_test.go`,
`internal/dataentryconfig/{validate,channels,validate_calendars,validate_gantts}_test.go`,
`internal/cli/validate_channels_test.go`, `cmd/rela-server/configure_test.go`,
the `internal/mcp` channel tool test,
`frontend/src/views/KanbanView.cardBusy.test.ts` (new),
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs/realtime.md`, `docs/data-entry.md`,
`docs/data-entry/api-reference.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`?busy=` (client).* The first value. It is looked up in the snapshot's
channel types, an allowlist from config. An unknown or unsuitable name serves no
`_busy`; it is never echoed into an error. Channel type names are config and not
a secret, so the lack of an error is for robustness (a stale SPA config after a
reload must not fail the board), not concealment.
- *`busy:` in `data-entry.yaml` (operator).* Checked at load, on reload, by
`rela validate` and by Configure against `realtime.yaml` (AC 8). A failed reload
keeps the last good config.
- *Session state (stored).* Only names from `WorkingStates()` are queried,
and labels come from the current config, never from stored text.

**Security-Sensitive Operations:**

- *Row gate.* `_busy` rides on rows the list already gated and redacted. A
hidden entity gets no row, so its channels never show.
- *Channel gate.* A session's state is channel content. `BusyReader` reads
the anchors again through the realtime gated reader and applies the channel
type's `read:` role per channel, as `Service.Read` and the inbox do. A denied
channel reads as not busy (`[]`), so the answer does not say that a channel
exists.
- *Gate before fold.* The per-row list is built only from channels that
passed the gate. There is no count or truncation flag that a hidden channel
could change. The row ceiling uses the hard per-anchor limit, so
`ErrTooManyRows` cannot be reached by stored channels, and the ceiling depends
only on the page's own anchors.
- *Live.* A channel change never selects or frames a list subscription in
this ticket, so no frame depends on a channel the subscriber cannot see. A full
round replays each subscription as its own principal.
- *Per principal.* Computed per request; not cached across principals
(AC 5).
- *Client ceiling.* `roleAdmits` applies the tool ceiling as for reads.
- *Rendering.* State and type labels come from config and render through
Vue interpolation, never `v-html`.
- *Errors.* A failed read is logged server side with the error; the client
sees only the missing field. Expected `ok=false` cases log nothing.
- *Cost.* One `ListWaiting` per page, bounded by `MaxWaitingAnchors` and
`MaxWaitingRows`. On the file backend that call reads one directory per anchor
(see Risks).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestSession_WorkingStates` |
| 2 | `TestBusy_States` (with the never-moved case), `TestBusy_ReadRole`, `TestBusy_Anchor`, `TestBusy_ManyChannels` |
| 3 | `TestBusy_NotServed` (with no principal) |
| 4 | `TestListBusy_Rows`, `TestListBusy_NotRequested`, `TestListBusy_NotServed`, `TestListBusy_StoreErrorKeepsList` |
| 5 | `TestListBusy_HiddenRow`, `TestListBusy_TwoPrincipals`, `TestLive_BusyListPerPrincipal` |
| 6 | `TestChannelsBudget_ListBusy10vs50` (two faces, denied reads) |
| 7 | `livepath_test.go` case, `TestLive_BusyListFullRound`, `TestLive_BusyListIgnoresChannelRound` |
| 8 | `TestValidateKanbanBusyField`, `TestValidateChannels_Busy`, calendar and gantt cases, `validate_channels_test.go`, `configure_test.go`, `realtime_wiring_test.go` |
| 9 | `KanbanView.cardBusy.test.ts`, `RlBusyIndicator.test.ts`, `RlBusyIndicator.stories.ts` |
| 10 | `ChannelsPanel.test.ts` cases, `RlChannelSession.stories.ts` "Working" |
| 11 | MCP description substring case |
| 12 | Reviewed; YAML examples load in `docs_example_test.go` where it covers the file |

Integration: `TestListBusy_PoCFlow` in `busy_rows_test.go` runs the HTTP list
handler with the in-memory channel store and a PoC-shaped `realtime.yaml`. It
posts `go` and then `claim` as the runner (session to `planning`), and the board
row shows one busy entry with label "Planning". It posts `plan` (session to
`plan-review`, `input_required`), and the row shows `[]`. On the SPA side,
`KanbanView.cardBusy.test.ts` mounts a board and drives the indicator from a
mocked list response.

**Edge cases:**

- Initial state is working and the session never moved: matched through
`""`; the entry has the initial state's name and label and `since == CreatedAt`.
- Cardinality many with two busy channels on one anchor: two entries, by
name; the card shows the first and "+1".
- A face-anchored type on a board that shows one face: only that face's
channels.
- A family-anchored type on a faced entity type: the family channel shows on
every face's card.
- A config reload that changes a state's status: the next read follows the
new config.
- A config reload that removes the busy type: `_busy` absent; the SPA shows
nothing.
- A channel type whose working states are all removed: `ok=false`, absent.
- A board of 0 rows: no channel-store call.
- A row whose type the busy type does not attach to (a mixed list): no
query for it; it gets `[]` when `ok`.
- A paused (closed) channel in `working`: not busy.
- `ChannelsPerEntity` lowered below the stored count: the ceiling uses the
hard limit, so `_busy` is still served.
- Long state labels: the indicator truncates with CSS; the full text stays
in the accessible name.

**Negative Tests:**

- `busy` with `property`, `relation`, `comments` or `display`; two `busy`
fields; `busy` on a calendar or gantt: refused at load.
- `busy: nope` (undeclared), `busy: chat` (no session), `busy: run` on a
`feature` board when `run` attaches only to `ticket`: refused at load, in `rela
validate` and in Configure, with no `channels:` block.
- `?busy=` empty or naming an unknown type: the list answers 200 without
`_busy`.
- A request without a principal: `_busy` absent, nothing logged.
- The channel store returns an error: `_busy` absent on every row, 200, one
warning logged.
- `ErrTooManyRows` (forced by a fake store): `_busy` absent, 200, logged.
- A caller who may read the ticket but not the channel (an
`entity.<property>` read role): `[]`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *File backend cost.* `ListWaiting` on the file backend walks one face
directory and reads each `meta.yaml` per listed anchor (`filechannels/ops.go`),
so a page costs I/O per row even though it is one call. The database backends
run one statement, served by the channels unique index. This is the cost the
inbox already accepts. The budget test pins the call count but cannot see the
file I/O; the godoc states it.
- *Default `working` shows busy where nobody works.* A non-final state
without `waiting_for:` reads `working`, such as a state that waits on a timer. A
busy type whose initial state defaults to working gets a load warning, and the
docs say to set `waiting_for:` or an explicit `status:`. See Q2.
- *A stale busy card after an agent crash.* The session stays in a working
state. The docs show a working-state timer that moves a stalled run on, and the
card shows the time in the state. See Q3.
- *A board does not update on a channel-only move.* Moves with graph
outputs refresh it live; others show on reload or on the next graph write.
Follow-up "busy board live refresh on session moves".
- *Cross-process gaps confuse users.* The docs say which moves show live
(those with graph outputs) and that TKT-YK30D1 helps only on postgres.
- *Plimsoll.* `Service` is at its cap, so the read is a separate type with
an unexported run struct. `channelsHandler` and `App` gain no method;
`serveRowBusy` is a function.

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime config is documented in docs/realtime.md)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change; `rela validate` checks more but takes no new flag)
- [x] docs/data-entry.md, kanban card fields: `busy: <channel type>`, its
checks, its label and `show_label`, and how it renders.
- [x] docs/data-entry/api-reference.md: `?busy=` on list endpoints and the
`_busy` field, absent versus `[]`.
- [x] docs/realtime.md, new section "Busy indicator": which states count
(status `working`, including the default), the initial-working warning, the PoC
example, gating, a working-state timer with `since: state` that moves a stalled
run to a blocked state, and the cross-process limits per backend.
- [x] docs/realtime.md, "In the web app": the activity line in the session
strip.
- [x] docs/realtime.md, "Agents and other processes": move the session into
a working state to show that you work; no typing over stdio yet.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the existing row-extra and inbox patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions

- **Q1. What does "busy" mean?** Decided: a session state whose `status:`
reads `working`, read from the current session. It is declared in config,
crosses processes today, and is what agents already read.
- **Q2. Should a defaulted `working` count?** Decided: yes, so the board
and `read_session` agree. A busy type whose initial state defaults to working
gets a load warning, because every new channel of that type would show busy. If
operators need a quiet non-final state, a later ticket can add an `idle` status
rather than a board-only rule.
- **Q3. Stall detection.** Decided: the card shows the time in the state as
visible text ("3h") and in the accessible name. The docs show a working-state
timer with `since: state` that moves a stuck run to a blocked state after a set
period; that needs no new code. A lease-based check stays a later idea.
- **Q4. List rows.** Decided: no list column in this ticket. The server
answers `?busy=` on every list; the column kind is the follow-up "busy column on
list rows".
- **Q5. How does an agent signal "working on it" in a channel?** Decided:
by a post that moves the session into a working state (the PoC's `claim`,
`approve` and `answer` do). An MCP `typing` tool waits for a cross-process
channel path. TKT-YK30D1 gives one only on postgres; the file and SQLite
backends would need a stored activity entry with a TTL or a watch on the fs
channel directory.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

Findings 1 to 3 are resolved by moving live wake on channel changes to the
follow-up "busy board live refresh on session moves". Finding 4 adds the
validate, Configure and boot paths (step 7). Finding 5 builds entries from
`currentSession` (step 2). The minor findings and nits are folded into the
steps, the ACs and the open questions above.
