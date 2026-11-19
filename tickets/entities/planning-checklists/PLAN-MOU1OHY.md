---
id: PLAN-MOU1OHY
type: planning-checklist
title: 'Planning: Hide Add to pile on lists with list_buttons'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood (customer lists show checkboxes because Add to pile counts as a bulk action and no config hides it on lists)
- [x] Scope defined (what's in/out documented below) (in: a list_buttons.add_to_pile toggle for lists and search results, the selectable rule, docs. Out: piles.toggles and the entity header (header_buttons))
- [x] Acceptance criteria documented with specific test scenarios (see the ticket body)

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change on an existing pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config toggle)
- [x] Checked codebase for similar patterns or reusable code (header_buttons (ChromeToggle, headerButtonsWire, /_sidebar) is the pattern)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art (data-entry-ui; FEAT-4FLTQ9 piles)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented (ListButtonsConfig with a ChromeToggle, resolved per principal on /_sidebar as list_buttons; EntityList selectable and the AddToPileMenu use it; SearchView too)
- [x] Approach builds on existing patterns (not reinventing) (mirrors header_buttons)
- [x] Alternatives considered (document why rejected) (widening header_buttons.add_to_pile to lists was rejected: its docs promise it does not touch lists, and changing it would alter existing configs; a per-list option would need every list edited)
- [x] Dependencies identified (packages, APIs, types) (none new)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs) (data-entry.yaml)
- [x] Input validation approach defined (allowlist preferred over blocklist) (ChromeToggle shape check and an allowlist of keys under list_buttons)
- [x] Security-sensitive operations identified (file access, auth, crypto) (none: a UX filter; the pile endpoints keep their ACL)
- [x] Error handling doesn't leak sensitive information (config errors name the key only)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (Go: config validation and per-principal resolution; Vue: no checkboxes when hidden, delete still selectable)
- [x] Edge cases identified and documented (an older server without list_buttons shows the button; deletable rows keep selection)
- [x] Negative test cases defined (invalid input, error conditions) (unknown key under list_buttons is rejected)
- [x] Integration test approach defined (not just unit tests) (the /_sidebar handler test with a real ACL policy)

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

- [x] Technical risks assessed with mitigations (low: default is unchanged)
- [x] Security risks assessed (see Security Considerations) (none: UX filter only, configedit allowlist keeps it locked like header_buttons)
- [x] Effort estimated (xs/s/m/l/xl) (s)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor) (docs/data-entry.md and its docs-project mirror)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing config)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change that copies the reviewed header_buttons design; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
