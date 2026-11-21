---
id: PLAN-RICAW5F
type: planning-checklist
title: 'Planning: Raise the archplan pins to v26.11.4'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

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

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: three version pins, no code)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: three version pins, no code)
- [x] ~~Checked codebase for similar patterns or reusable code~~ (N/A: three version pins, no code)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: three version pins, no code)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: three version pins, no code)

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
- [x] ~~Approach builds on existing patterns (not reinventing)~~ (N/A: three version pins, no code)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: three version pins, no code)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: three version pins, no code)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: three version pins, no code)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: three version pins, no code)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: three version pins, no code)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: three version pins, no code)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: three version pins, no code)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: three version pins, no code)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: three version pins, no code)

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
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: three version pins, no code)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: three version pins, no code)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: three version pins, no code)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: three version pins, no code)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: three version pins, no code)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: three version pins, no code)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: three version pins, no code)
- [x] ~~README.md - Project-level changes~~ (N/A: three version pins, no code)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: three version pins, no code)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: three version pins, no code)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: three version pins, no code)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

## Notes

**Scope:** the three archplan pins in the `justfile`. `.archplan/baseline.json`
is not changed: accepting findings is the owner's decision.

**Acceptance:** see the ticket. **Approach:** take the release commit and the
exporter tag from the v26.11.4 release; run the route trace and `archplan ci`
with the new version. **Risk:** finding ids move between versions. A traced run
shows which. Effort: xs.
