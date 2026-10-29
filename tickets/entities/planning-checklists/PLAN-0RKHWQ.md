---
id: PLAN-0RKHWQ
type: planning-checklist
title: 'Planning: Hide agent-internal message types from people'
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
(`rela-agent-runner-poc/rela-mission/project/realtime.yaml`) posts message types
that only the runner needs. `claim` and `resume` carry a worktree path.
`progress` carries a status note, often many in a row. All three are `post:
[runner]`. A person who opens a `run` channel sees every one of them as a full
card, between the messages that need them (`plan`, `ask`, `permission`,
`finished`). The web app has no way to tone them down.

The other runner types are for people: `plan`, `ask`, `permission`, `finished`
and `fail` ask for a decision or report a result. So the granularity is the
message type, not the author: the runner posts both kinds.

`data-entry.yaml` already has a presentation block for channels:
`channels.<type>.messages.<mt>.fields` (`dataentryconfig.MessagePresentation`,
`internal/dataentryconfig/channels.go`). Its godoc says `realtime.yaml` stays
structural and this block holds presentation, as `forms:` does for entities. The
block reaches the SPA through `GET /api/v1/_config`, then `presentationView`
(`channelView.ts`), then `RlChannel`'s `presentation` prop. A display option
fits there.

**This is a display option, not access control.** A collapsed or hidden type
stays readable to everyone who may read the channel: through the REST API,
`read_channel` over MCP, `rela channel read`, and in the web app with the toggle
on. The config is not a secret either (CLAUDE.md, "The configuration is not a
secret"). Nothing here gates a read.

**Terms used below.**

- *Display*: the new key `display:` on a message type in the `channels:`
block. Its values are `collapsed` and `hidden`. Absent or empty means shown in
full.
- *Quiet type*: a message type with a display value.
- *Run*: a sequence of adjacent held messages of quiet types, with no other
row between them. A hidden message continues a run. A removed message that sits
between two members of a run joins it. A session notice, a kept row or any other
message ends a run.
- *Fold line*: one row that stands for the collapsed messages of a run,
with a button that expands them in place.
- *Kept row*: a message of a quiet type that is always shown in full,
because hiding it would hide something the reader must act on or asked for. Once
a row is kept, it stays kept until the channel unmounts. See AC 4.
- *Show-all toggle*: a button in the channel header that shows every
message in full, with no fold lines and nothing hidden.
- *Scroll anchor*: the row whose offset from the top of the log is kept
across a change, so the reader's view does not move.

**Scope (in):**

- `data-entry.yaml`: `channels.<type>.messages.<mt>.display: collapsed |
hidden`, checked at load, on reload, by `rela validate` and by Configure.
- Configure: `channels.*.messages.*.display` is an editable key.
- `GET /api/v1/_config` serves the key (it rides on the existing struct).
- SPA library (`rela-components`): a pure fold function, a fold line
component, the show-all toggle in `RlChannel`, scroll anchoring for row changes,
the reveal path for reply quotes, and the quiet rules for announcements and
signal lines.
- SPA app: `presentationView` passes `display` through, and the reply-quote
jump calls back into `RlChannel` once an older page holds the target.
- Docs: `docs/data-entry.md` and its mirror `GUIDE-data-entry.md`, plus a
pointer in `docs/realtime.md`.

**Scope (out):**

- A `display:` key in `realtime.yaml`. See Q1.
- Any change to the REST API, MCP, CLI, OpenAPI or `read_channel`. They show
every message, as today.
- Unread counts and the inbox. The server counts every message as unread,
quiet or not, because it does not read `data-entry.yaml`. Documented as a known
limit. See Q5 and the follow-up "Server-known quiet message types (unread,
inbox)".
- Remembering the toggle across page loads. See Q3.
- Treating a person's own posts differently. The SPA has no current-user
identity. See Q3.

**Acceptance Criteria:**

1. **The key loads and is checked.** `display: collapsed` and `display:
hidden` on a message type in the `channels:` block load. An absent key,
`display: ""` and `display: null` all load and mean shown in full, as empty does
for the other `display:` keys in `data-entry.yaml` (`config.go:1499`). Any other
value (including `shown` and `Hidden`) is refused with the path
`channels.<type>.messages.<mt>.display` and the valid values. A message type
that `realtime.yaml` does not declare is refused as today. `display` on a field
entry is refused as an unknown field key. A block with only `display` and no
`fields` loads. Test: `internal/dataentryconfig/channels_test.go`
`TestChannelsPresentation_Validate` new cases (`display collapsed`, `display
hidden`, `display empty`, `display bad value`, `display without fields`,
`display on a field`, `docs example`); `TestValidateChannels_Display` (new) for
the unknown message type case against a fake `ChannelSchema`.
2. **Configure may edit it.** `channels.*.messages.*.display` is in
`editable`, not in `protective`, because it restricts nothing. Test:
`internal/configedit/allowlist_test.go` `TestAllowlist_ClassifiesEveryKey` stays
green, plus a case in `TestCheckEdit_DataEntry` that sets `display` under
`channels.*.messages.*` and expects no violation.
3. **The SPA gets the key.** `GET /api/v1/_config` carries
`channels.<type>.messages.<mt>.display`. `presentationView` copies it into the
library's `ChannelMessagePresentation.display`. Test:
`internal/dataentry/channels_schema_test.go` `TestConfig_ServesChannels` asserts
the key; `channelView.test.ts` `presentationView` case "passes display".
4. **Runs fold and hide.** With the toggle off, `foldMessages` turns the
held messages into rows. The collapsed messages of a run become one fold line. A
hidden message becomes no row and does not end the run. A run that mixes both
becomes a fold line for its collapsed messages only, and expanding it shows only
those. A run with only hidden messages becomes no row. A removed message between
two members of a run joins it, counts in the fold line, and shows as the removed
placeholder when expanded. Kept rows are shown in full and end a run:
   - the row of `decisionSeq`;
   - a row with choices;
   - a row whose `actions` include a message-type action, that is a name
other than `edit`, `delete`, `remove` and `release`;
   - a row the reader asked for through a reply quote (AC 6);
   - a row that was kept for any of these reasons earlier in this mount.

Session notices are never quiet. With the toggle on, every message is one row,
as today. Test: `frontend/src/components/entity/channels/channelFold.test.ts`
(new, table): one run; two runs split by a notice; collapsed, hidden, collapsed
is one run; expanding a mixed run shows only collapsed; hidden only; removed
inside a run joins it; removed outside a run is a row; decision row kept; choice
row kept; type-action row kept; operation-only actions not kept; revealed seq
kept; toggle on; no quiet types (output equals input); fold key unchanged after
append.
5. **A fold line expands in place.** The fold line is a button with the
text "3 messages: Claim, Progress" (en) / "3 berichten: Claim, Progress" (nl),
with the type labels of its collapsed messages in first-seen order. The text
does not change when it expands; `aria-expanded` carries the state. A click
shows the run's messages in place, below the button. The button keeps its offset
in the log, the run opens below it, and the log stops following new messages
until the reader scrolls back to the bottom. This holds for the run at the
bottom of the log too. The run stays expanded when a live frame adds a message
to it, when older messages load, and when the oldest are trimmed. Focus on the
fold button or inside an expanded run survives a live frame that adds to the
run. Test: `RlChannel.stories.ts` story "Quiet types" with a play function
(expand, collapse, `aria-expanded`, the tail fold's button keeps its offset on
expand while at the bottom); `channelFold.test.ts` cases "expanded run stays
expanded after append" and "after prepend"; `ChannelsPanel.test.ts` case "focus
on the tail fold survives an append".
6. **Kept rows keep earlier features working.** In a fixture schema based
on the PoC, with an extra hidden type `handoff` that moves the session into a
state where the owner has decisions:
- A hidden `handoff` that is the decision cause still shows, with its
decision buttons (TKT-T9G1AE). After the decision, its row and focus stay.
- A hidden question with `choices:` still shows its buttons (TKT-03FQPD).
After the answer, its row and focus stay.
- The session strip and its "Working…" line are unchanged (TKT-21G6JS).
- A reply whose quote points at a hidden message: a click on the quote
reveals that message in place, scrolls to it and focuses it. For a collapsed
target, its run expands. This also works when the target is on an older page
that the jump has to load.
- A reply quote to the last message of an expanded run focuses that
message, not the fold button. Test:
`frontend/src/components/entity/ChannelsPanel.test.ts`, new
`describe('ChannelsPanel quiet types')` with cases "hidden decision cause keeps
its buttons", "decided hidden cause keeps its row and focus", "hidden question
keeps its choices", "answered hidden question keeps its row and focus", "reply
quote reveals a hidden target", "reply quote expands a collapsed run", "reply
quote reveals a hidden target on an older page", "reply quote to the last
message of an expanded run".
7. **The show-all toggle.** The header shows a toggle "Show all messages"
only when the channel type has at least one quiet type. It is a button with
`aria-pressed`. On, every message shows in full and signal lines of quiet types
show. Off, the default, the rules of AC 4 apply. The toggle is not stored: each
mount starts off. Pressing it, or a config reload that changes a display value,
keeps the reader's view: the first visible row keeps its offset, or, if that row
is now hidden, the next shown row takes its place. At the bottom, the log stays
at the bottom. Test: "Quiet types" story play function (toggle present, pressed
state, fold lines gone); story "Quiet types, scrolled up" with a play function
that presses the toggle while scrolled up and checks the first visible row's
offset; `ChannelsPanel.test.ts` case "toggle absent without quiet types".
8. **Only shown messages are announced.** With the toggle off, a new
message is announced only when it renders as a full row: a kept row, a message
of a type that is not quiet, or a message inside an expanded run. A new message
that is hidden or inside a collapsed fold is left out. When nothing is left
after this filter, nothing is announced. A new signal of a quiet type,
`collapsed` or `hidden`, is not shown in the signal area. With the toggle on,
both behave as today. Test: `ChannelsPanel.test.ts` cases "live quiet message
not announced", "kept quiet row is announced", "only quiet additions announce
nothing", "quiet signal not shown"; the existing announcement cases stay green.
9. **An all-quiet page says so.** When no held message renders as a row,
the log shows a hint instead of "No messages". Without older pages: "Only hidden
messages here. Use Show all messages to see them." With older pages: "Only
hidden messages so far. Load older messages or use Show all messages to see
them." When "Load older" adds only messages that do not render, the polite
region says "4 older messages are hidden. Use Show all messages to see them."
Test: `ChannelsPanel.test.ts` cases "all hidden shows the hint", "all hidden
with older pages", "load older with only hidden is announced".
10. **Docs.** `docs/data-entry.md`, "Channels", documents `display`, its
two values, the fold line, the toggle, the kept rows, signals, the unread and
inbox limit, the rule for which types may be quiet, the contrast with a field's
`hidden: true`, and that it is not access control. Test: the documented YAML
example is a case in `TestChannelsPresentation_Validate` (`docs example`); the
prose is reviewed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the choices and their tradeoffs are recorded under Approach and Open questions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `dataentryconfig.MessagePresentation` and `channelsUnknownKeys`
(`internal/dataentryconfig/channels.go`): the presentation block and its key
checks. `messagePresentationKeys` gains `display`.
  - `display:` as a presentation key elsewhere in `data-entry.yaml`
(sections, card fields, dashboard cards), and `collapsed:` on kanban columns and
nav groups. `useKanbanCollapse.ts` stores reader overrides in localStorage; not
copied here (Q3).
  - `presentationView` (`channelView.ts`) and `RlChannel`'s `presentation`
prop: the path from config to the library already exists.
  - `decisionSeq` and `choices` props on `RlChannel` (TKT-T9G1AE,
TKT-03FQPD): the fold reads them to keep those rows.
  - `RlChannel.jumpTo` and `useChannelPaging.onJump`: the reply-quote path
that the reveal hooks into.
  - `RlChannel.announce`: the polite region the shown-row rule filters.
  - The scroll anchor in the `props.messages` watch (`firstVisible`): the
same capture and restore, reused for row changes.
  - `channelFields.ts`: a pure helper module beside the component. The app
already deep-imports it (`channelView.ts:59`).
  - `channelNewMessages` in `useMessages.ts`: the pattern for a
plural-aware message function.
- Reference implementations: Slack and GitHub fold bot and timeline events
("3 more events", "Show hidden items"); Discord hides "system" messages behind a
toggle. All fold runs into one expandable line and keep the content reachable.
- rela prior art: TKT-T9G1AE, TKT-03FQPD and TKT-21G6JS in the same view;
the `hidden:` channel field key; DEC-6C1NAA (labels are authored).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config (`internal/dataentryconfig/channels.go`).**
`MessagePresentation` gains `Display string yaml:"display,omitempty"
json:"display,omitempty"`. Constants `MessageDisplayCollapsed` and
`MessageDisplayHidden`. `messagePresentationKeys` adds `display`.
`validateChannelsShape` refuses any non-empty value other than the two, with the
path and the valid values. Empty means shown in full, so the struct decode is
enough and no raw-YAML check is needed. The existing `channelBlockErrors`
already refuses an undeclared message type. No change to `ChannelSchema`: a
signal type needs no separate rule (step 6).
2. **Configure (`internal/configedit/allowlist.go`).** Add
`channels.*.messages.*.display` to `editable`, beside
`channels.*.messages.*.fields.[].**` (`allowlist.go:184`), with a comment that
it is presentation only.
3. **SPA types.** `frontend/src/types/config.ts`: the message entry of
`ChannelPresentation` gains `display?: 'collapsed' | 'hidden'`. Library
`types.ts`: `ChannelMessagePresentation.display` with the same type.
`presentationView` copies it.
4. **Fold (`rela-components/src/components/channel/channelFold.ts`, new).**
`foldMessages(messages, opts)` returns `ChannelRow[] = { kind: 'message',
message } | { kind: 'fold', key, first, last, messages, labels, expanded }`.
`opts` holds `displayOf(type)`, `showAll`, `keep: ReadonlySet<number>` and
`expanded: ReadonlySet<number>`. A row is also kept when its `actions` hold a
message-type action. One pass, no DOM.
   - A fold's Vue `key` is `fold-<first seq>`. A live append changes only
the end of a run, so the fold and its expanded rows are not recreated and focus
inside them survives. Only loading older messages into the run's start, or
trimming it, changes the key.
   - The scroll anchor is a separate value: the fold `li` carries
`data-fold-seq` set to its last seq. It never carries `data-seq`, so `jumpTo`
and `focusRow`, which query `[data-seq]`, only find message rows.
   - A run is expanded when any of its seqs is in `expanded`. Expanding
adds all its seqs, so a message appended later joins an expanded run.
   - `rowFor(rows, seq)` returns the rendered row that holds `seq`: its
message row, the fold whose run contains it, or, for a seq that renders nothing,
the first rendered row after it.
   - Not exported from the package index. `channelFold.test.ts` in the app
deep-imports it, as `channelView.ts` does with `channelFields`.
5. **Fold line (`RlChannelFold.vue`, new).** A native button with
`aria-expanded`, and `aria-controls` set only while expanded, pointing at the
run's nested list. A chevron icon with `aria-hidden`. Its text is its accessible
name and does not change with the state. Emits `toggle`. Focus stays on the
button. The text comes from a plural-aware message function `channelFold({
count, labels })` in `useMessages.ts` and `messages.nl.ts`, like
`channelNewMessages`, not from a `ChannelTextKey` entry.
6. **`RlChannel.vue`.**
   - Before anything else, `ephemeralText` moves to `channelFields.ts` as a
pure function that takes the type map, the presentation, the enum options and
`t`. This frees about 25 counted lines.
   - State: `showAll` (ref, false), `expanded` and `revealed` (sets of
seq). `revealed` only grows during a mount. A watch on `decisionSeq`, `choices`
and the type-action rows adds every seq that is kept to `revealed`, so a row
stays kept after the decision or the answer that made it kept is gone.
`OpenChannel` is keyed by channel, so a new channel starts with empty sets.
   - `quiet` computed: the quiet types from `presentation`. The toggle
renders in the header only when `quiet` is non-empty, as an `RlButton` with
`aria-pressed`, text `channelShowAll`.
   - Row structure: one `v-for` over `rows` inside the existing `ol`. A
message row is an `li` with `RlChannelMessage`. A fold row is an `li` with
`data-fold-seq`, the `RlChannelFold` button and, when expanded, a nested `ol`
with an id and one `li` per message. So the log's list semantics stay valid. The
`RlChannelMessage` binding is written once, as `v-bind="messageBind(m)"` and
`v-on="messageOn"`, and used in both places.
   - Scroll anchor: `firstVisible` selects `[data-seq], [data-fold-seq]`
and records a seq. The restore resolves that seq with `rowFor` and queries the
row's attribute, so a seq that moved into a fold, or out of one, still finds its
place. One helper, `keepView(anchor?)`, captures before a change and restores on
`nextTick`. The `props.messages` watch uses it as today. A second watch on
`showAll`, `expanded`, `revealed` and `presentation`, with `flush: 'pre'`, uses
it too, so the toggle, a reveal and a config reload keep the view. At the bottom
(`stuck`), the log stays at the bottom, as today.
   - Expand and collapse: the fold's `toggle` sets `stuck` to false and
calls `keepView` with the fold row as the anchor. The button keeps its offset
and the ResizeObserver no longer scrolls to the end.
   - Reveal: `reveal(seq)` adds a hidden or kept target to `revealed`, or
a collapsed target's run to `expanded`, then on `nextTick` scrolls the target
into view and focuses it. `jumpTo(seq)` calls `reveal` when the seq is held.
When it is not held, it emits `jump` as today. `defineExpose` gains `reveal`.
   - Announce: with `showAll` off, `announce` keeps only added messages for
which `rowFor` returns their own message row, or a message inside an expanded
fold. It returns early when neither messages nor notices are left, so
`messages!` is never read empty. When older messages arrive and none of them
render, it announces `channelOlderHidden({ count })`.
   - Signals: the signal area drops signals of a quiet type, `collapsed` or
`hidden`, while `showAll` is off.
   - Empty text: when `messages` is non-empty and `rows` is empty, show
`channelOnlyHidden`, or `channelOnlyHiddenOlder` when `hasOlder` is set.
   - Line budget: `RlChannel.vue` counts about 408 lines under the
max-lines options today, so about 92 spare. Moving `ephemeralText` frees about
25 more. The fold logic and the fold line live in their own files. `max-lines`
is warn-only, so this is hygiene, not a gate.
7. **App.**
   - `channelView.ts`: `presentationView` passes `display`.
   - `useChannelPaging.ts`: `useChannelPaging` takes a third argument,
`onHeld(seq)`. `onJump` calls it once the target is held after it loaded older
pages.
   - `OpenChannel.vue`: passes `(seq) => channelView.value?.reveal(seq)` as
`onHeld`. One line; the file is at about 481 counted lines.

**Alternatives considered:**

- *A `display:` key on the message type in `realtime.yaml`.* Rejected.
`realtime.yaml` is structural; the `channels:` block is where presentation
lives, and it already reaches the SPA. The key would also have to pass through
`realtimecfg`, `Describe`, `apiwire`, `read_channel`, the MCP golden, the
OpenAPI goldens and the CLI renderer, and it would suggest that MCP and CLI
clients should hide messages. That does not belong in this ticket. It becomes
worth it when unread counts and inbox rows must know quiet types (Q5).
- *A toggle only, with the SPA guessing by author kind (agent).* Rejected.
The runner posts both kinds: `ask` and `plan` must stay visible.
- *Booleans `hidden: true` / `collapsed: true`.* Rejected: two keys can
both be set. One `display:` key matches the rest of the file.
- *A neutral `audience: agents` key.* Rejected: it names intent but not
what the web app does, and a later reader would have to guess.
- *Only `collapsed`, no `hidden` and no toggle.* Smaller, and nothing is
ever out of reach. Rejected because the ticket asks for hidden, and `claim` and
`resume` say nothing a person needs. The toggle is the price of `hidden`, and it
is small.
- *Fold in the app (`channelView.ts`, `OpenChannel.vue`).* Rejected: the
library already owns the presentation and the scroll anchoring, and
`OpenChannel.vue` is near its line limit.
- *Refuse `display: ""` through a raw-YAML check.* Rejected: the struct
decode cannot tell it from an absent key, and empty already means the default
for the other `display:` keys.
- *A non-interactive "4 hidden messages" line for hidden runs.* Rejected:
it turns `hidden` into a second `collapsed`. The empty hint and the announcement
after "Load older" cover the case where nothing shows.
- *Two PRs (collapsed first, then hidden and the toggle).* Rejected: one
ticket and one PR, with the effort raised to m.

**Dependencies:** `internal/dataentryconfig`, `internal/configedit`. No change
to `internal/realtime`, `internal/realtimecfg`, `internal/apiwire`,
`internal/mcp` or `internal/cli`. Frontend: no new packages.

**Files to modify:**

- `internal/dataentryconfig/channels.go`.
- `internal/configedit/allowlist.go`.
- `frontend/packages/rela-components/src/components/channel/channelFold.ts`
(new), `RlChannelFold.vue` (new), `RlChannel.vue`, `channelFields.ts`,
`types.ts`, `RlChannel.stories.ts`, `src/fixtures/channels.ts`,
`src/composables/useMessages.ts`, `src/composables/messages.nl.ts`.
- `frontend/src/types/config.ts`,
`frontend/src/components/entity/channels/channelView.ts`,
`frontend/src/components/entity/channels/useChannelPaging.ts`,
`frontend/src/components/entity/channels/OpenChannel.vue`.
- Tests: `internal/dataentryconfig/channels_test.go`,
`internal/configedit/allowlist_test.go`,
`internal/dataentry/channels_schema_test.go`,
`frontend/src/components/entity/channels/channelFold.test.ts` (new),
`frontend/src/components/entity/channels/channelView.test.ts`,
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs/data-entry.md`,
`docs-project/entities/guides/GUIDE-data-entry.md`, `docs/realtime.md` (one
pointer in "In the web app").

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`display:` in `data-entry.yaml` (operator, or Configure).* An allowlist
of two values, plus empty for the default, checked at load, on reload, by `rela
validate` and by Configure. A failed reload keeps the last good config.
- *Message types and seqs (server).* The fold reads only fields the SPA
already renders. Type labels render through Vue interpolation, never `v-html`.

**Security-Sensitive Operations:**

- None. This is presentation only. It adds no read path, no write path and
no gate. A hidden message is still sent to the browser and is one click away. It
must not be described or used as a way to keep content from a reader; the docs
say so. The key is in Configure's `editable` set and not in `protective`,
because removing it loosens nothing.
- Errors: the load error names the config path and the valid values. Both
are config, not secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestChannelsPresentation_Validate` new cases (with `docs example`), `TestValidateChannels_Display` |
| 2 | `TestAllowlist_ClassifiesEveryKey`, `TestCheckEdit_DataEntry` display case |
| 3 | `TestConfig_ServesChannels`, `presentationView` "passes display" |
| 4 | `channelFold.test.ts` table |
| 5 | "Quiet types" story play, `channelFold.test.ts` append and prepend cases, "focus on the tail fold survives an append" |
| 6 | `ChannelsPanel quiet types` cases |
| 7 | "Quiet types" and "Quiet types, scrolled up" story plays, "toggle absent without quiet types" |
| 8 | "live quiet message not announced", "kept quiet row is announced", "only quiet additions announce nothing", "quiet signal not shown" |
| 9 | "all hidden shows the hint", "all hidden with older pages", "load older with only hidden is announced" |
| 10 | `docs example` case, reviewed |

Integration: the `ChannelsPanel quiet types` cases mount the app's channel panel
with a fixture schema based on the PoC, plus a hidden `handoff` type that moves
the session into a state with owner decisions. Its `channels:` block sets
`claim`, `resume` and `handoff` to `hidden` and `progress` to `collapsed`. The
cases drive it with mocked reads and live frames, through `OpenChannel`,
`useChannelPaging`, `presentationView` and `RlChannel`. Scroll and layout checks
run in story play functions, because jsdom has no layout.

**Edge cases:**

- A run of one collapsed message: one fold line ("1 message: Progress").
- A notice between two `progress` messages: two fold lines.
- A hidden `claim` between two `progress` messages: one fold line for the
two `progress`; expanding it does not show the `claim`.
- A removed `progress` inside a run: it joins the run and shows as the
removed placeholder when expanded. A removed message outside a run is a normal
row.
- The decision cause is collapsed and sits in the middle of a run: the run
splits around it, and stays split after the decision.
- A reply quote to a hidden message that is not held: `jump` loads older
pages as today, then `onHeld` calls `reveal`, which shows, scrolls to and
focuses it. If the jump gives up, nothing is revealed.
- `loadOlder` returns only hidden messages: nothing new shows, the polite
region says how many were loaded, and "Load older" stays while `hasOlder` holds.
- `hasNewer` is set: folding works on the held page; "Jump to latest" is
unchanged.
- A config reload removes `display`: the next render shows every message,
and the first visible row keeps its offset.
- The toggle is on and a config reload removes every quiet type: the
toggle disappears and everything shows.
- The reader expands the tail fold while at the bottom, then a live
`progress` arrives: the log does not jump to it, and the run gains the new
message.
- A person posts a message of a hidden type from the composer: it is
hidden for them too (Q3). The docs rule excludes types people can post, so this
needs a misconfiguration.

**Negative Tests:**

- `display: shown`, `display: Hidden`, `display: 1`: refused at load.
- `display` on a field entry: refused as an unknown field key.
- `display` under an undeclared message type: refused as today.
- `display` with realtime off: the existing "realtime is off" warning, no
error.
- A row whose only actions are `edit`, `delete`, `remove` or `release` is
not kept.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Hiding something a person must act on.* Mitigated by kept rows (AC 4),
which stay kept for the mount, the announcement rule (AC 8) and the toggle. The
docs say to use `display` only on types that people cannot post and that do not
mention or reply to people.
- *Scroll jumps when rows change.* Mitigated by one anchor helper used for
message changes and for row changes (toggle, expand, reveal, reload), a separate
anchor attribute on folds, and `rowFor` to find a seq that moved into or out of
a fold.
- *Focus loss on live frames.* Mitigated by keying a fold by its first
seq, which a live append does not change.
- *Unread badge counts quiet messages.* A channel can show "5 unread" while
the reader sees one fold line. Documented as a known limit; Q5.
- *Inbox rows lead to a hidden message.* An inbox mention or reply row
opens the channel without a seq, so a quiet message that mentions or replies to
the reader is hard to find. Mitigated by the docs rule; recorded under Q5.
- *Line limit on `RlChannel.vue`.* Mitigated by the two new files and by
moving `ephemeralText` up front. The limit is warn-only.

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema or realtime.yaml change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md, "Channels": the `display` key with a PoC example,
its two values, the fold line, the show-all toggle, the kept rows, that both
values keep a signal out of the signal area, the unread and inbox limit, and the
sentence that it is not access control. The rule: use `display` only on types
that people cannot post and that do not mention or reply to people. One sentence
contrasts the two meanings of hidden: a field's `hidden: true` is never shown
and never sent, while `display: hidden` on a message type is sent and shown
behind the toggle. Mirrored in `GUIDE-data-entry.md`.
- [x] docs/realtime.md, "In the web app": one sentence that points to
`display` for agent-only message types.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the existing presentation block)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions

- **Q1. Config key or toggle?** Decided: both, in one PR. A per-type key in
`data-entry.yaml`, `channels.<type>.messages.<mt>.display: collapsed | hidden`,
plus a show-all toggle in the channel header that appears only when the type has
a quiet type. Rejected: a `realtime.yaml` key (not structural, and not needed
until Q5), a toggle only (no right granularity), two booleans, `audience:`.
- **Q2. What does collapsed look like?** Decided: the collapsed messages of
a run fold into one fold line that expands in place, below a button that keeps
its place. Hidden messages continue a run and stay hidden when it expands.
Paging, live frames and trimming keep a run's expansion. Kept rows (decision
cause, choices, type actions, reply target, earlier kept) and session notices
end a run. Unread counts and the inbox are unchanged (Q5).
- **Q3. Who sees what?** Decided: the same for everyone in the web app,
including the poster. The SPA does not know the current user, and the type is
the unit an operator marks. Agents read through MCP, the CLI or the API, which
ignore `display`, so they always see every message. The toggle is not persisted,
so each visit starts with the operator's choice.
- **Q4. Accessibility.** Decided: the fold line is a native button with
`aria-expanded`, and `aria-controls` only while expanded. Its visible text is
its name, says the count and the type labels, and does not change with the
state. The fold is an `li` in the messages list with its run in a nested list.
Expanding moves no focus and announces nothing beyond the state change. The
toggle is a button with `aria-pressed`. While the toggle is off, only messages
that render as a full row are announced.
- **Q5. Should unread counts and inbox rows know quiet types?** Decided:
out of scope for this ticket and documented as a known limit. The fix is a later
key in `realtime.yaml`, so the server knows quiet types; the server will not
read `data-entry.yaml`. The same follow-up covers inbox mention and reply rows
that lead to a quiet message. Follow-up: "Server-known quiet message types
(unread, inbox)".

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 23 findings (7 significant, 12 minor, 4 nit). All
significant findings are addressed in this plan; see the review responses linked
to the ticket.
