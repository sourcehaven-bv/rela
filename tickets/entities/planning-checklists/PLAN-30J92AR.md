---
id: PLAN-30J92AR
type: planning-checklist
title: 'Planning: Schema namespaces and variants (load-time expansion)'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] ~~Problem/requirements clearly understood~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Scope defined (what's in/out documented below)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Acceptance criteria documented with specific test scenarios~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Checked codebase for similar patterns or reusable code~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] ~~Technical approach chosen and documented~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Approach builds on existing patterns (not reinventing)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] ~~Test scenarios documented for each acceptance criterion~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Edge cases identified and documented~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

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

- [x] ~~Technical risks assessed with mitigations~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Effort estimated (xs/s/m/l/xl)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~README.md - Project-level changes~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: retroactive closure; the work shipped in #2099 before this item was tracked)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
