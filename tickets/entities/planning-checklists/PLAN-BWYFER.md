---
id: PLAN-BWYFER
type: planning-checklist
title: 'Planning: Relation-backed status: design boards and views for a single-valued status relation'
started: '2026-10-06'
status: done
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Design relation-backed status: a task's state is a single-valued relation to a status entity, and boards and views must work with it.

**Scope:**
- In: the model, a decision on how views read status properties, a stated behaviour for every feature row in the ticket table, and follow-up tickets.
- Out: implementation (follow-up tickets) and multi-valued relations on boards.

**Acceptance Criteria:**
1. A research entity records the chosen model and the rejected alternative. Check: RES-8CKUNJ options A to D and the recommendation.
2. Every row of the ticket table has a stated behaviour. Check: the table in this ticket and RES-8CKUNJ.
3. Follow-up tickets exist for the first iteration and are linked. Check: RES-8CKUNJ informs TKT-65LVAK, TKT-DA9C0L, TKT-KJ3Q07, TKT-JO8PN3, TKT-2EN0G5 and TKT-ZKPA1E.

## Research

- [x] For larger features: run `/research` to create a structured research doc (RES-8CKUNJ)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: an internal metamodel and view feature; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: done once for the whole design in TKT-ZNFGNJ and RES-8CKUNJ)
- [x] Reviewed relevant rela concepts for prior art (metamodel-types, views, data-entry-ui)

**Research Doc:** RES-8CKUNJ (option D: status entity plus relation paths).

**Existing Solutions:** Survey of develop at 2f7579199, recorded in the ticket table with file references (kanban validation, drag, group_by, sorting, filters, picker, cardinality, styles, transitions, automations, CalDAV).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Status as a global entity type; `has_status` with `max_outgoing: 1`; an orderable `offers_status` relation on the parent selects and orders columns; views read status properties through relation paths and conditions use `related()`.

**Alternatives:** A larger global enum with per-parent enabled values (option A) gives no metadata per status. Materialised lookup properties (option C) copy data and go stale. Both rejected in RES-8CKUNJ.

**Dependencies:** orderable relations (FEAT-FE5P), `related()` (TKT-205V2N).

**Files to modify:** none; design only. The demo is in examples/relation-status-demo.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** N/A for a design; each follow-up ticket plans its own inputs. Write-time cardinality (TKT-65LVAK) is the main new validation.

**Security-Sensitive Operations:** Relation writes and picker candidates; covered per follow-up ticket (ACL on removes, unreadable targets dropped).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each AC is checked against RES-8CKUNJ and the follow-up links. The design is also checked by the working demo.

**Edge Cases:** task with a status the parent does not offer (Other column, never hidden); task with no status; task under two parents (the page anchor decides).

**Negative Tests:** multi-valued relation as a board column source is refused by config validation (follow-up).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): l

**Risks:** Many features need to become relation-aware. Mitigation: split into small follow-up tickets in dependency order, and build a demo first.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Per follow-up ticket: docs/data-entry.md, docs/concepts.md and docs/metamodel.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was settled in TKT-ZNFGNJ and RES-8CKUNJ and checked with the working demo in examples/relation-status-demo; two code-review rounds covered the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design-review findings; code-review findings are tracked as review-responses on the ticket)

**Design Review Findings:** None. Code-review findings: see the review checklist.
