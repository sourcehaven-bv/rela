---
id: PLAN-IEG5X4
type: planning-checklist
title: 'Planning: Kanban column colour and column totals'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: enum column header colour from `styles:`; kanban key `column_total:
{property, label, currency}`; card field option `currency:`; config validation;
docs; Vitest; e2e. The plan was approved by the user up front.

Out of scope: totals in list views, swimlane totals (refused at load),
server-side aggregation, `currency` on calendar chips and gantt tooltips
(refused at load), non-integer amounts (rela has no decimal type).

**Acceptance Criteria:**
1. Enum column with a style shows that colour; without a style, none. Test: Vitest on KanbanView (styled and unstyled value), e2e checks the status dot colour.
2. `column_total` shows "<label>: <amount>" under each header, also on an empty column (0) and on a collapsed column. Test: Vitest (sum, empty column, collapsed rail, default label "Total", no currency plain number); e2e on a fixture board.
3. `currency: EUR` on a card field renders 3500 as "€ 3.500" in nl-NL. Test: unit test of `formatAmount` with an explicit locale; Vitest card render.
4. Config load rejects bad config. Test: table-driven Go tests in internal/dataentryconfig.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach fixed by the approved plan)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: Intl.NumberFormat is built in)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `columns_from.style_from` already colours relation
columns via `styleStatusColor` (frontend/src/utils/styleColors.ts,
useRelationColumns.ts). `schemaStore.stylesForProperty` resolves an enum
property's style map. `refuseCardCounts` (validate.go) already refuses
kanban-only card keys on calendar and gantt fields. The library `Section` type
(rela-components) carries title, colour, count; it gains a summary line.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Go config: `Kanban.ColumnTotal *KanbanColumnTotal{Property, Label, Currency}` and `KanbanCardField.Currency`.
- Validation (validateKanbans): `column_total.property` required, must be a property of the entity type with type `integer`; `currency` must match `^[A-Z]{3}$` (ISO 4217 shape; Intl.NumberFormat formats every well-formed code, so an allowlist adds no safety and would block valid codes); `column_total` with `swimlane_property` is an error. Card field `currency` needs a `property` of type `integer` and cannot combine with relation, comments or display. Calendar and gantt fields refuse `currency`.
- rela-components: `Section.summary?: string`, rendered under the column heading and on the collapsed rail.
- KanbanView: enum column colour = `styleStatusColor(stylesForProperty(column_property, entity), column.value)`. Column total = sum of finite numeric values of the property over the column's cards; label default "Total"; formatted by a new `formatAmount(value, currency?, locale?)` in utils/format.ts (Intl.NumberFormat with the browser locale, as formatDate does; fraction digits 0 for whole numbers, 2 otherwise). Card field with `currency` renders the formatted amount as text instead of the number widget.
- Configure editor: allow `kanbans.*.column_total.**` in internal/configedit/allowlist.go; rename `column_total.property` in frontend/src/configure/references.ts; label in changes.ts.

Alternatives: server-side aggregation (rejected: out of scope, the board already
loads every card); a currency allowlist (rejected, see above); summing inside
the library (rejected: the library stays data-agnostic, the app formats).

**Files to modify:**
internal/dataentryconfig/{config.go,validate.go,validate_kanban_total_test.go},
internal/configedit/allowlist.go,
frontend/packages/rela-components/src/{types/index.ts,components/board/RlBoardColumn.vue,RlBoardColumnCollapsed.vue},
frontend/src/{types/config.ts,utils/format.ts,views/KanbanView.vue,configure/references.ts,configure/changes.ts}
plus tests, e2e/tests/fixtures.ts, e2e/pages (KanbanPage),
e2e/tests/kanban-totals.spec.ts, docs/data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config (validated at load, shape check
on currency); entity values the reader already received (non-numeric values are
skipped).

**Security-Sensitive Operations:** none. The total is computed in the browser
over cards the server already gated and redacted, so it discloses nothing the
reader cannot already see. A field the reader may not see is absent and adds
nothing.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** empty column (total 0); missing or non-numeric value (skipped);
negative values (summed); collapsed column (rail shows total); filtered board
(total follows visible cards); columns_from board (supported); value without
style (no colour).

**Negative Tests:** unknown property, string property, missing property,
lowercase or 4-letter currency, swimlanes plus column_total, currency on a
relation field, on a string property, on a calendar field, on a gantt field.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** locale-dependent output makes tests flaky: tests pass an explicit
locale to formatAmount, and e2e asserts with a locale-agnostic pattern or the
browser's configured locale. The library change could affect other consumers:
the field is optional and renders nothing when absent. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md kanban section (fields table, new
"Column totals" subsection, card `currency`, column colour note).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the user reviewed and approved the plan before the ticket was created)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
