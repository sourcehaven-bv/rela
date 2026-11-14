---
id: PLAN-52TIKW6
type: planning-checklist
title: 'Planning: Structured questions in realtime channels'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: one-level object list properties; `choices.label`; batch post (store,
service, HTTP, MCP, CLI); open questions recorded on the session with a
`session.open_questions` guard variable; buttons on every open question of the
moving batch; grouped question display with Change; `display: cards` hint in
data-entry.yaml; plain-text reply quotes.

Out: nested objects; object types in schema.yaml; checking that an answer is one
of the options; agent-runner changes. Rejected designs (nested containers, flat
lists with a group id) are recorded in the ticket body.

**Acceptance Criteria:**

1. Object lists: a post with valid items succeeds over HTTP, MCP and CLI
(`--props-json`). An unknown sub-key, a wrong type, a missing required
sub-property or too many items is refused, and the error names the path
(`options[2].label`). A config that nests an object in an object is refused at
load.
2. `choices.label`: one button per item, a click posts the item's label into
`into`. Load refuses a missing `label` on an object list, a `label` that is not
a declared non-list `string` sub-property, and a `label` on a string list.
3. Batch post: N messages post in one step with ascending seqs (notices may
sit between) and one batch id; if any is refused (validation, permission,
session state, size), none are stored and the error names the index. A batch
over the cap is refused. Each message passes the session in order, so a batch of
four questions moves `working → asking` once.
4. Asked questions: after that batch, four questions are asked; three answers
leave the session in `asking` and a `read_session` watch does not wake; the
fourth moves it to `working` via `when: "session.open_questions == 0"`, also
when that transition has `outputs:`; the watch wakes once. Removing one of two
answers keeps the question answered; removing the only answer reopens it. Any
move clears the set. The author cannot edit or delete an asked question during
the span.
5. Web app: the batch shows as "Question k of N"; an answered question collapses
to its value with Change; Change edits or removes the own answer as the answer
type allows; another device sees the same state after reload.
6. `display: cards` renders body prose, cards with + / − lines and a badge, and
no raw items list; load refuses map entries that name undeclared or wrongly
typed properties.
7. A reply quote of `**bold** _x_` shows `bold x`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: design options (nested containers, flat lists with a
group id, one message per question in a batch) were weighed with the user on
2026-11-11; the decision and reasons are in the ticket body.

**Existing Solutions:**

- Choices: `internal/realtimecfg/choices.go`, wire
`internal/apiwire/v1/channels.go:567`, SPA `channelView.ts:530` `choiceViews`,
click `useSessionDecisions.ts:492` `onChoose`.
- Properties: compile `realtimecfg/validate.go:511`, normalize
`realtimecfg/props.go:126`, JSON Schema `realtimecfg/jsonschema.go:60`.
- Store append with a session step: `channels.Store.Append`
(`internal/channels/store.go`), `SessionStep.Decide` is pure, so the open asked
set must sit on the stored channel record, beside the session.
- Reply counts are kept in the append step already; closing an open question
is the same kind of in-step update.
- Session watch wakes only on a session change (`read_session` in
`internal/mcp/tools_channels.go`), so agents need nothing new.
- Presentation: `dataentryconfig/channels.go:75` `MessagePresentation`.
- Markdown: `frontend/src/utils/markdown.ts:427` `renderMessageMarkdown`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

PR 1: object lists, `choices.label`, quotes.
- `rawProperty.Properties`; `PropertyObject` with ordered `Fields`; list-only;
sub-properties are string, integer, boolean, enum or string list (no ref or
markdown: refs and mentions are resolved at top level only).
- Every `p.List` site is handled: `messageRecordType`, `PredicateValue`,
button presets, `describe.go`, CLI coercion. Object lists are refused at load in
`when:` environments, presets and triggers. `Property.Normalize` validates items
with paths. Limits: items `MaxListItems`, fields per object 16.
- JSON Schema: `array` of `object`, `additionalProperties: false`.
- Wire `ChannelPropertySchema.properties`; TS types; CLI render of items.
- `choices.label` checks; button-size warning and the `into` fit check use the
label's `max`; duplicate label values are refused on post when the list is a
`choices.from`.
- SPA: `choiceViews` reads `item[label]`; default rendering of object items.
- `excerptOf`: render with `renderMessageMarkdown`, take text, then truncate.

PR 2: batch post and asked questions.
- One write path: `channels.Store.AppendBatch(ctx, key, BatchPost)`; `Append`
becomes a batch of one in every backend. All or nothing under one lock or
transaction. Seqs ascend; a session notice may sit between posted messages. Each
posted message (not notices, and kept on removal stubs) carries `Batch` (the
first seq). A `reply_to` into the same batch is refused. The file backend builds
every record in memory and writes meta once and the records once; the Store doc
states what a crash mid-batch leaves.
- A batch is one post for the session. One `expect_generation` per batch,
checked against the pre-batch session. Every message is admitted (`in:`, roles,
`when:`) against the pre-batch state. Decide runs per message in order until one
moves; later messages are not decided, so a batch moves the session at most
once.
- Asked questions live on `channels.Channel.Asked map[seq]{AnswerType,
Answers}` (not on `flow.Instance`; flows untouched). Realtime sets
`NewMessage.Asks` (the answer type) on each top-level message whose type has
`choices:`. After the batch, those messages join `Asked`; a move first clears
it, so they join the new span's set. An append whose `reply_to` is in `Asked`
and whose type matches adds one answer; `Remove` subtracts one, reading the type
before it erases. All under the same lock; no config in the store.
- The same logic is one pure function, `channels.PlanBatch(cur, batch)`,
used by the store and by the service pre-check, so `precheckOutputs`, the output
gate, `personMark` and `runInline` see the state the store will see.
- `session.open_questions` (entries with zero answers) is a variable of the
session `when:` environment. The guard is optional; without it the first answer
moves the session, as today.
- Locks: author edits and deletes of a question in `Asked` are refused while
the span lasts, as for the session message. A moderator removal drops the
question from `Asked`; the session then moves on the next post (documented).
Answer edits after the move stay allowed (documented).
- Storage: pg and sqlite migrations for the asked column; `meta.yaml` field;
a cap check on read and write (at most `MaxBatch` entries per span plus
questions from later batches, capped at `MaxAsked = 20`).
- Transports: `POST .../_batch` with its own body limit
(`MaxBatch * MaxMessageBytes` plus overhead) and a total-bytes cap; MCP
`post_messages`; CLI `rela channel post-batch --json`. `MaxBatch = 10`. Rate
tokens are charged once the batch validates; mentions are capped per batch and
notifications coalesced per recipient.
- Wire: `Asked` (seqs, answer counts, batch size) on the HTTP channel head
for the SPA only, never on `read_session`, so a session watch wakes only on the
move.
- SPA: buttons on every question in `Asked` with zero answers; the group and
N come from `Asked`, not from held messages. `RlChannelQuestions` renders a
batch as "Question k of N"; Change uses the answer type's edit_own or
delete_own.

PR 3: cards.
- `MessagePresentation.Cards{Body, Items, Title, Plus, Minus, Badge}` with
`Display: cards`; `ValidateChannels` checks names and types against facts
extended with list and sub-property types; `cards:` without `display: cards` is
refused.
- `RlChannelMessage` cards branch with per-card buttons; hides `items`.

Alternatives rejected: fixed slots (status quo); nested containers (depth and
size caps, custom shape per use case, one answer message loses cross-device
partials); flat lists with group ids (references inside a message need integrity
checks); a server-side "answered" flag per message (the open set on the session
is enough and is cleared on every move).

**Files to modify:**

- internal/realtimecfg (raw, compiled, validate, props, jsonschema, choices)
- internal/realtimelimits/limits.go
- internal/store/pgstore/migrations (new asked column), internal/sqlitedb
migrate.go
- internal/channels (store.go, session.go, channelstest, filechannels,
sqlchannels, pgchannels, sqlitechannels)
- internal/realtime (service.go, session.go, validate.go, describe.go,
render.go)
- internal/apiwire/v1/channels.go, internal/dataentry/channels_handler.go,
channels_schema.go
- internal/mcp/tools_channels.go; internal/cli/channel.go, channel_render.go
- internal/dataentryconfig/channels.go
- frontend: types/channels.ts, api/channels.ts, channelView.ts,
useSessionDecisions.ts, OpenChannel.vue, inbox/*, utils/markdown.ts,
rela-components channel/RlChannelMessage.vue, new RlChannelQuestions.vue and
RlChannelCards.vue
- e2e: realtime-project.ts, channels.spec.ts, channels.page.ts
- docs-project: GUIDE-realtime.md, GUIDE-data-entry.md, GUIDE-mcp-server.md,
GUIDE-cli-reference.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Posted object values: allowlist of declared sub-keys, per-field type and
length, item and field caps; errors name the path, never echo values.
- Batch posts: cap on count; every message is checked exactly as a single
post (role, `in:`, `when:`, size, mentions); one refusal refuses all.
- Config: object nesting refused at load; cards map checked at load.

**Security-Sensitive Operations:**

- Rendering: markdown sub-properties through `renderMessageMarkdown`; strings
as text. Button labels and `into` stay plain strings, so a click never posts
markup the asker chose.
- Quote stripping inserts text, not HTML.
- No new read path: messages keep the channel gates; `Channel.Asked` holds only
seqs of messages in the same channel.
- A batch cannot bypass the per-message session checks, because each message
runs the same Admit/Decide.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. realtimecfg table tests (compile, refuse nesting), props Normalize paths,
jsonschema golden; HTTP, MCP and CLI post tests.
2. choices_test for `label`; SPA unit test for `choiceViews` with objects.
3. channelstest conformance for `AppendBatch` (order, atomicity, session step
per message, batch id) on file, postgres and sqlite; service and HTTP tests.
4. channelstest for Asked (set, count on answer, reopen on remove, clear on
move); realtime test for the guard; MCP watch test that three answers do not
wake a session watch and the fourth does.
5. SPA unit tests for grouping and Change; e2e: answer three of four, reload,
change one, answer the last, session moves.
6. dataentryconfig tests for `cards`; component test for the cards branch.
7. markdown util test for the excerpt.

**Edge Cases:**

- Batch of one equals a single post. A batch mixing questions and other
types: only `choices:` types become open.
- Answer to a question not in Asked (old question, other span): stored, no
effect on Asked.
- Two answers to one open question: the first closes it; the second is stored
as today and raises the count again.
- Session moved by a timer or another post: Asked cleared; buttons drop.
- Empty object list; duplicate labels (buttons keyed by index); unicode.

**Negative Tests:**

- Nested object config; unknown sub-key; wrong sub-type; over-cap items;
`label` on a markdown or list field; batch over the cap; batch where the third
message is not postable (nothing stored, error names index 2).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Size: xl. Mitigation: three PRs under this ticket, each green on its own.
- `AppendBatch` across three backends: mitigate with the conformance harness
and by building it on the existing single-append step code, called in a loop
under one lock/transaction.
- Pre-check and store drifting apart: one pure `PlanBatch` used by both,
with tests that compare them for the last answer and each batch message.
- Remove changes `Asked` outside a session step: same lock, type read before
erase; answer/remove races covered on all three backends.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] GUIDE-realtime.md: object lists, `choices.label`, batch post, open
questions and `session.open_questions`, agent waiting pattern
- [x] GUIDE-data-entry.md: `display: cards`
- [x] GUIDE-mcp-server.md / GUIDE-cli-reference.md: batch post, object values

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 20 findings from an independent review on 2026-11-11
(1 critical, 10 significant, 7 minor, 2 nits), recorded as review-response
entities linked to TKT-GR1MXL5 and all addressed in this plan.

Added test gaps: pre-check/store agreement; read_session watch wakes once;
answer/remove race on file, pg and sqlite; removing an open question and editing
an answered question; migration round-trips; object-list `when:` refused at
load; batch over the body and byte limits; crash mid-batch on the file backend.
