---
id: PLAN-5QXSKDM
type: planning-checklist
title: 'Planning: LLM actions on collection screens (kanban, lists, piles)'
started: "2026-11-13"
status: done
---
<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: a `batch:` block on an entity-bound `llm:` action. One run takes up to
`batch.max` entity addresses, gates each as the detail action would, sends
the readable rows in one model call and returns one proposal per row. Batch
actions run from list `actions:` (the selected rows) and from a new kanban
`actions:` key (a header button over the cards the board shows). A batch
review dialog applies the changes the user ticks through the ordinary PATCH.

Out (follow-ups): `inputs.related` and `outputs.relate` in batch mode; a
kanban column-header menu; piles; MCP `run_action` with several entities;
running the per-entity action once per row on `jobs.Queue`; proposals drawn
on the cards; answer-only (text) outputs.

Design change against the ticket body: the client sends the addresses of the
rows on screen, and the server gates each one. The kanban board filters on the
client, so the server cannot rebuild "the rows on screen" from the view
config. An address list is safe because every row is read as the caller and
must pass the action's `available_on`, `when` and `permission`; it is also
exactly what the user is looking at.

**Acceptance Criteria:**

1. A batch run returns one proposal per eligible row and writes nothing.
   Test: handler test with a fake provider; store unchanged.
2. A row the caller cannot read, or that fails `when`, is skipped and counted;
   it never reaches the provider. Test: handler test records the request.
3. A `visible:`-hidden field of any row never reaches the provider. Test:
   handler test.
4. A reply with an unknown record id, a missing record, an unknown property or
   an invalid value is rejected. Test: llmtask parse table tests.
5. More ids than `batch.max`, duplicate ids, `entity_ids` on a non-batch
   action, or both `entity_id` and `entity_ids` give a 400. Test: handler.
6. Invalid batch config fails to load naming the key (batch with related or
   relate, max out of range, size budget, kanban/list referencing a non-batch
   or wrong-type action). Test: dataentryconfig table tests.
7. The dialog applies only checked changes per entity, and skips stale ones.
   Test: frontend unit test.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: extends TKT-4USH1HO's design; options discussed with the user in session and recorded in TKT-I2SRNWR)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: same provider wire format as TKT-4USH1HO)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-house extension)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `actionOps.boundEntity` / `newDetailActionCheck` gate one row; reused per row.
- `llmtask.Task` builds the per-entity request and parse; batch mode reuses
  its schema, value checks and `parseSet`.
- `useListActions` / `useActionFanout` run list bulk actions per row; batch
  actions branch there to one request.
- `LLMProposalDialog` and `planApply` (TKT-4USH1HO) give the per-entity apply
  logic; the batch dialog reuses `planApply`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- Config: `llm.batch.max` (1..50, default 20). Batch actions refuse
  `inputs.related` and `outputs.relate`; `max * content.max_chars` must stay
  under a total budget. List `actions:` may name an entity-bound action only
  when it is a batch llm action on the list's type. Kanban gets `actions:`,
  each a labelled batch llm action on the board's type. The action's JSON
  carries `llm_batch` (the max) so the SPA knows to send one request.
- API: `POST /_action/{id}` with `{"entity_ids": [...]}`. Permission once;
  each address gated as the detail action; unwritable rows skipped. One
  provider call; response `{"proposals": [...], "failed": [...], "skipped": n}`.
- llmtask: `BatchRequest(rows)` and `ParseBatch(resp, rows)`. The reply is
  `{"records": {<id>: {"set": {...}}}}`, every id required, each record's
  `set` schema narrowed to that row's writable properties.
- SPA: `runBatchAction`; `LLMBatchProposalDialog` grouped per entity; list
  bulk bar and kanban header buttons open it.

Alternatives rejected:
- Server re-resolves the screen's rows from config: the kanban board filters
  client-side, so the server would send different rows than the user sees.
- One model call per row: N calls inline is slow and costly; it belongs on
  `jobs.Queue` with progress and is a follow-up.

**Files to modify:**

- `internal/dataentryconfig/llmaction.go`, `config.go`, `validate.go` (+tests)
- `internal/llmtask/llmtask.go`, new `batch.go` (+tests)
- `internal/actionproposal/actionproposal.go` (title)
- `internal/script/action.go`, `internal/apiwire/v1/responses.go` (proposals)
- `internal/dataentry/actions_request.go`, `actions_ops.go`, new
  `actions_llm_batch.go` (+tests)
- `frontend/src/api/actions.ts`, new `LLMBatchProposalDialog.vue` (+test),
  `useListActions.ts`, `EntityList.vue`, `KanbanView.vue`, types, i18n
- `docs-project/entities/guides/GUIDE-data-entry.md`, regenerated `docs/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `entity_ids` (client): a bounded list of strings; each resolved through the
  gated reader. Unreadable and not-matching rows are skipped alike, so the
  skipped count reveals nothing beyond what the caller sent.
- Entity data (untrusted, may carry prompt injection): sent only as JSON in
  the user message. One row's text can steer proposals for other rows; every
  change is reviewed and free-text values start unticked.
- LLM reply: record ids must be exactly the ids sent; values checked as in
  TKT-4USH1HO.

**Security-Sensitive Operations:**

- Data to the provider: declared inputs only, per-row gated and redacted,
  bounded by `batch.max` and the size budget. `permission:` required.
- Writes: only on accept, per entity, through the existing PATCH.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria. Handler tests go through the
real HTTP route with a fake provider.

**Edge Cases:**

- Every row skipped: 200 with no proposals, provider not called.
- Row with no writable output: skipped.
- Model proposes the current value: no change for that property.
- Board shows more cards than `batch.max`: the SPA refuses before sending.

**Negative Tests:** extra or missing record ids; empty `entity_ids`; non-string
id; ids above max; duplicate ids; batch with related/relate in config.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Long replies for many rows: `max_tokens` applies; a cut-off reply is an
  error naming `max_tokens`.
- Cross-row prompt injection: review per change; property outputs only.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - `batch:` on llm actions, kanban `actions:` (via docs-project)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see review-response entities linked to TKT-I2SRNWR.

Decisions from the design review:

- Gate in batch: compile the `when:` lookup once, read the rows through the
  gated reader in one pass, fill bodies with `loadRowContent`, and run one
  `MatchPage` over the redacted page. Per-row ACL verdicts stay per row, as
  on list pages. A `storetest.Counting` budget test pins the store reads.
- Cross-row prompt injection: every batch value starts unticked; each row has
  an "accept row" control. Accepted risk, documented.
- Per-record failures: a reply that is not JSON, names an unknown record or
  was cut off fails the whole run. A missing record or a bad value for one
  record becomes a `failed` entry `{entity, title, reason}` beside the
  proposals.
- Size: the load-time content budget stays; the encoded user message is also
  capped at run time, and list values are capped in length.
- Schema size: rows that share a writable set share one `$defs` entry.
- `entity_ids` is parsed only for llm actions, with a typed decode.
- Kanban sends the cards in expanded, declared columns. Above `batch.max` the
  SPA refuses and tells the user to filter or collapse columns.
- The response is never a 204 for a batch run: `proposals` is always set.
- Batch runs get a 5-minute provider timeout.
- List and kanban carry `_actions["action:<id>"]` for batch actions.
- The list batch branch keeps the selection and the rows.
- The SPA confirms with the row count before sending.
- One info log line per batch call: action, row count, model; no ids.
- A batch action still runs single-row on the detail page and through MCP.
