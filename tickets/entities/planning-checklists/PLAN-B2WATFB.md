---
id: PLAN-B2WATFB
type: planning-checklist
title: 'Planning: Remove remaining dead code and dedupe helpers'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

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

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] ~~Test scenarios documented for each acceptance criterion~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Edge cases identified and documented~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

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
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~README.md - Project-level changes~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
