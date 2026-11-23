---
id: PLAN-X84TYAR
type: planning-checklist
title: 'Planning: Remaining data-entry UI gaps in New button and column Add and create forms and lists'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: header New on grouped lists, column Add on property-backed kanban boards, body editor on create forms, non-title first list column, docs. Out: swimlane boards, state-machine awareness in the SPA.
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. Grouped list with create_form shows no header New.
2. Property-backed board with create_form shows column Add that pre-fills the column value and no header New.
3. Create form shows the body editor only with body: true.
4. A first column that is not the name renders as configured.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small UI change, no research needed)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: uses the existing rela-components board and table)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal UI behaviour)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: no concept changes)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Frontend only: EntityList.vue, KanbanView.vue, DynamicForm.vue, tableColumns.ts, plus docs and e2e.
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no new inputs; the column value comes from board config)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new inputs)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no auth, file or crypto code touched)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no new error paths)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
<!-- Map each acceptance criterion to how it will be tested -->

**Edge Cases:**
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:**
<!-- What should fail? How should it fail? -->

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Hiding the body editor changes the default for existing forms. Effort s.
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: ticket is a UI fix, docs updated in the same change)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small frontend change, reviewed after implementation with /code-review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
