---
id: PLAN-VU7RCB
type: planning-checklist
title: 'Planning: Show a related entity''s property on a kanban card'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: kanban card field `{relation, direction, property, label?,
show_label?}` showing the property of each related entity (deduplicated),
rendered as the property renders elsewhere; config validation; refusal on
calendar and gantt fields; a Go test that a hidden related property never
reaches the include block; docs; Vitest; e2e. The user approved the plan up
front.

Out of scope: multi-hop paths and sort/filter by a related property
(TKT-DA9C0L), the form on calendar chips and gantt tooltips, backend changes.

**Acceptance Criteria:**
1. An incoming relation field with an enum property shows a badge coloured by the enum's `styles:`. Test: Vitest on KanbanView; e2e checks the badge colour on a fixture board.
2. Several related entities show each distinct value once. Test: Vitest with two related entities sharing a value and one with another value; e2e with two fixing tasks.
3. A related integer with `display: {currency}` renders as an amount; a related date is formatted. Test: Vitest; e2e on an outgoing field.
4. A related property hidden by `visible:` is absent from `included`, so the card shows nothing. Test: Go list test with a hiding resolver; Vitest that a missing property renders no line.
5. Config load rejects an unknown relation, the wrong side for the direction, a property missing on one target type (the message names the type), `property` with `display: count`, and the form on calendar and gantt fields. Test: table-driven Go tests.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach fixed by the approved plan)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-app rendering)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The board already fetches with `include=*` when a card
has a relation field. `resolveV1Includes` row-gates included peers and
serializes them with `forWireRelated`, which strips `visible:`-hidden
properties; `include=*` also pulls incoming peers. So the related entity's
properties are already in `included`, redacted. Card property fields resolve a
dense widget once per field (`densePropertyRoutingHint`), and widgets take an
`entityType` for enum styles and display formats. `refuseCardCounts` refuses
kanban-only card keys on calendar and gantt fields. TKT-DA9C0L is the larger
relation-path design; this ticket does not preempt its syntax.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Go config: no new field. `KanbanCardField` already has `Relation`, `Direction` and `Property`; the combination becomes meaningful. `DisplayLabel` derives the property name when both are set.
- Validation (kanban card fields): with relation and property, resolve the direction (explicit or inferred); the board type must be on the source side for outgoing and the target side for incoming; `property` must be declared on every type at the other end, and the error names each type that lacks it. `display: count` plus `property` is an error. Calendar and gantt fields refuse relation plus property with an explicit message (in the shared refusal helper, renamed to cover the new key).
- SPA: `KanbanView` resolves a relation-property field by reading the related ids from the gated `relations` map, looking each up in `included`, collecting `properties[property]`, skipping missing values and deduplicating by type and value. Widgets resolve once per field and target type. `ResolvedCardField` gains `values` (one entry per distinct value with its entity type); `CardFieldList` renders each with the widget and the related type, so enum styles and currency come from the related type. Default label is the property name.
- Configure editor: renaming a property on the related type renames the card field's `property`; renaming a property on the board type no longer renames a relation field's property.
- Backend: unchanged. A Go test pins that `include=*` on a list strips a hidden property of an incoming peer.

Alternatives: a `<relation>.<property>` path string (rejected: that is TKT-DA9C0L's syntax and needs single-valued relations); a server-side computed field (rejected: the data is already on the wire and gated, a second channel would need its own gating).

**Files to modify:**
internal/dataentryconfig/{config.go,validate.go,validate_calendars.go,validate_gantts.go} plus a new test file,
internal/dataentry/list_incoming_relation_test.go,
frontend/src/{types/config.ts,views/KanbanView.vue,components/common/CardFieldList.vue,configure/references.ts,stores/configDraft.ts} plus tests,
e2e/tests/fixtures.ts, e2e/pages/kanban.page.ts, e2e/tests/kanban-related-property.spec.ts,
docs-project GUIDE-data-entry (renders docs/data-entry.md).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config, validated at load; entity
values already served to the reader.

**Security-Sensitive Operations:** reading a neighbour's property. The SPA
reads only what `included` carries, which is row-gated (hidden neighbours are
absent and their ids are removed from `relations`) and field-redacted. No new
endpoint or server read. A Go test pins the redaction for an incoming peer on
the list endpoint.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** no related entity (no line); related entity without the value
(skipped); unresolved id (skipped, no raw-id fallback for a property); same
value on several related entities (one badge); a relation with several target
types (widget per type); list-valued property (rendered by its list widget).

**Negative Tests:** unknown relation, wrong side for the direction, property
missing on one of two target types, property with `display: count`, the form on
a calendar event field, on a gantt tooltip field.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** an existing config with `relation` and `property` both set would
change meaning. Before, kanban ignored `property` on a relation field, so the
field showed titles; now it shows the property, and a property missing on the
target types is a load error. Mitigation: the docs never described the
combination, and validation names the problem. CardFieldList is shared with
calendar chips: `values` is optional and the calendar never sets it. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry kanban "Card Configuration": new
"Properties of related entities" subsection; note on calendar and gantt.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the user reviewed and approved the plan before the ticket was created)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
