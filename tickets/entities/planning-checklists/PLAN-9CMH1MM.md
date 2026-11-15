---
id: PLAN-9CMH1MM
type: planning-checklist
title: 'Planning: Kanban quick-add'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A new board key `quick_add: true` in `data-entry.yaml` (`kanbans.<id>`). Off by default.
- A one-line title input at the foot of every column, and of every column-by-swimlane cell. Enter creates the card.
- All board shapes: an enum `column_property` board (with or without swimlanes), a `columns_from` board (the new card gets the edge to the column target; the Other column and a wall pile get no edge), and a plain wall (title only).
- On a page tab the new card is linked to the page entity, as the create form already does (`usePageTabScope.linkCreated`).
- Config-load validation of what quick-add needs.
- Allowlist the key in the Configure space (`kanbans.*.quick_add`, editable).

Out of scope:
- Setting properties other than the title, the column value and the swimlane value.
- Choosing a template variant. The default template applies.
- Quick-add on a list view.

**Acceptance Criteria:**
1. With `quick_add: true`, typing "Risk A" in the Draft column and pressing Enter creates an entity with `title: Risk A` and `status: draft`. The card shows in Draft, and the input is empty and keeps focus. (Vitest, e2e)
2. On a swimlane board the cell input sets both the column and the swimlane property. (Vitest, e2e)
3. On a `columns_from` board the new card carries the edge to the column target; in the Other column it has none. (Vitest)
4. Without `quick_add` no input renders. (Vitest)
5. When `_actions.create` is false, no input renders. (Vitest)
6. A refused create (422 or 403) shows the server message inline under the input, keeps the typed text, and creates nothing. (Vitest, e2e)
7. Config load refuses `quick_add` when a required property has no value source, when the type uses manual ids, or when the title property is missing or computed. (Go table test)
8. The create goes through `POST /api/v1/<plural>`, so ACL, field grants, validation, audit and the entity template apply. (Go test: a template default lands on a quick-add shaped create)

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small feature on existing primitives)

**Existing Solutions:**
- `validateCalDAVConstructible` (internal/dataentryconfig/validate_caldav.go) is the precedent for refusing, at config load, a create surface that cannot fill required properties. Quick-add follows it, including the note that templates are not consulted.
- `useWallGrouping` already creates entities from the board through `entitiesStore.create`, and `onColumnAdd` builds the `columns_from` relation prefill. Quick-add reuses both shapes.
- `entitymanager.buildCandidateEntity` applies `templates/entities/<type>.md` on every create, so the default template applies to a quick-add create with no extra code.
- Trello, Linear and GitHub Projects offer "add card" at the foot of a column, with Enter to save and the input staying open. Same model here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Go:
- `Kanban.QuickAdd bool` (`quick_add`). Served on `/_config` as is.
- `validateKanbanQuickAdd` (new file `validate_kanban_quickadd.go`):
  - The title property is `card.title`, else the type's primary property. It must be declared, not computed and not a roll-up.
  - Manual-id types are refused, because quick-add sends no id.
  - Every required property must be the title, the column property, the swimlane property, or have a schema default (property `default:` or custom type `default:`). Otherwise the error names them and says to add a default or turn quick_add off.
- `internal/configedit/allowlist.go`: `kanbans.*.quick_add` editable.

SPA:
- `KanbanConfig.quick_add?: boolean`.
- Library: a `footer` slot on `RlBoardColumn` and `RlSwimlaneColumn`, forwarded by `RlBoard`, `RlSwimlane` and `RlSwimlaneBoard` with `{ section }` and `{ section, lane }`.
- New `components/kanban/KanbanQuickAdd.vue`: an input whose accessible name names the column (and lane). Enter submits unless composing; Escape clears. It emits `submit(title)` and shows `error` inline (`role="alert"`) and a busy state.
- `KanbanView.vue`: `quickAddEnabled = quick_add && canCreate()`. `quickAdd(title, section, lane?)` builds `{ properties, relations?, world? }`, calls `entitiesStore.create`, then `tabScope.linkCreated`, then refreshes the board. Errors are kept per cell and shown inline. Warnings on a 201 show as a warning toast.

**Alternatives considered:**
- Fall back to the full form when required properties are unset: rejected. The board cannot know at Enter time which properties the template fills, and a form popping up on Enter breaks the facilitator's flow. A config-load error is seen once, by the operator.
- Consult templates at config load: rejected. The validator receives only (cfg, meta), as for CalDAV.
- A dedicated quick-add endpoint: rejected. The existing create endpoint already does ACL, validation, audit and templates.

**Files to modify:**
- internal/dataentryconfig/config.go, validate.go, new validate_kanban_quickadd.go (+ test)
- internal/configedit/allowlist.go
- internal/dataentry (Go test that a quick-add shaped POST picks up the template)
- frontend/packages/rela-components/src/components/board/RlBoard.vue, RlBoardColumn.vue, RlSwimlane.vue, RlSwimlaneBoard.vue, RlSwimlaneColumn.vue
- frontend/src/components/kanban/KanbanQuickAdd.vue (new), frontend/src/views/KanbanView.vue, frontend/src/types/config.ts
- frontend/src/i18n locales (en, nl)
- e2e/tests/fixtures.ts, e2e/pages/kanban.page.ts, new e2e/tests/kanban-quick-add.spec.ts
- docs-project data-entry guide, kanban section

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Typed title: free text, trimmed; empty is not sent. The server validates it like any create.
- Column and swimlane values come from the board's own columns (config enum values, or target ids the reader can see), never from free input.
- `quick_add` config: a bool, validated at load against the metamodel.

**Security-Sensitive Operations:**
- The create is an ordinary `POST /api/v1/<plural>`. Collection create ACL (`acl.OpCreate`), `fields:` create grants, relation affordances, validation and audit all run there and in `entitymanager.Manager`. Hiding the input on `_actions.create === false` is a UI hint only.
- The inline error text is the server's existing message for that refusal; no new information.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1-6: `KanbanView.quickAdd.test.ts` (Vitest) with a mocked `createEntity`.
- AC7: table test `TestValidateKanbanQuickAdd`.
- AC8: Go test in `internal/dataentry` posting the quick-add body shape to a type with a template.
- e2e `kanban-quick-add.spec.ts`: a column, a swimlane cell, and a refusal shown inline.

**Edge Cases:**
- Whitespace-only title: nothing is sent.
- Enter during IME composition: ignored.
- A second Enter while the first create is pending: ignored while busy.
- Collapsed column: no input, since a rail renders no cards.
- Wall pile and Other column: no edge.
- Faced type under a world: `world` rides the body, as in the create form.

**Negative Tests:**
- 422 from a `validations: enforce: write` rule or strict mode: inline message, text kept.
- 403 from ACL: inline message.
- Config: missing title property, computed title, manual-id type, required property without default.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The library slot change touches shared board components. The slot is additive and changes nothing when unused; the existing board tests cover the rest.
- A board that turns on `quick_add` for a type with required fields fails config load. That is intended, and the message says how to fix it.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project data-entry guide, kanban section: `quick_add`
- [x] docs/ regenerated by `just docs`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self review; no critical or significant findings, so
no review-response entities.
- Required-property handling decided: config-load refusal (CalDAV precedent), templates not consulted; template defaults still apply at create time.
- `_actions.create` is already on the board's list response (`collectionActions`), so no new affordance verb is needed.
- The create body needs `world` for faced types, as the create form sends it.
