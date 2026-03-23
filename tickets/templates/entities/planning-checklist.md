---
title: Planning
status: pending
---

## Understanding

- [ ] Problem/requirements clearly understood
- [ ] Scope defined (what's in/out documented below)
- [ ] Acceptance criteria documented with specific test scenarios

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [ ] Searched for existing libraries that solve this problem
- [ ] Checked codebase for similar patterns or reusable code
- [ ] Looked for reference implementations in other projects
- [ ] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [ ] Technical approach chosen and documented
- [ ] Approach builds on existing patterns (not reinventing)
- [ ] Alternatives considered (document why rejected)
- [ ] Dependencies identified (packages, APIs, types)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [ ] Input sources identified (user input, config, external APIs)
- [ ] Input validation approach defined (allowlist preferred over blocklist)
- [ ] Security-sensitive operations identified (file access, auth, crypto)
- [ ] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [ ] Test scenarios documented for each acceptance criterion
- [ ] Edge cases identified and documented
- [ ] Negative test cases defined (invalid input, error conditions)
- [ ] Integration test approach defined (not just unit tests)

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

- [ ] Technical risks assessed with mitigations
- [ ] Security risks assessed (see Security Considerations)
- [ ] Effort estimated (xs/s/m/l/xl)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
