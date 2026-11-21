---
id: PLAN-R2369SQ
type: planning-checklist
title: 'Planning: seqtrace routes suite: trace the audit, changes and next-action routes'
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

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Checked codebase for similar patterns or reusable code~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: trace scenarios and tooling, no behavior change)

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
- [x] ~~Approach builds on existing patterns (not reinventing)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: trace scenarios and tooling, no behavior change)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: trace scenarios and tooling, no behavior change)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: trace scenarios and tooling, no behavior change)

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
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: trace scenarios and tooling, no behavior change)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~README.md - Project-level changes~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: trace scenarios and tooling, no behavior change)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: trace scenarios and tooling, no behavior change)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

## Notes

**Scope:** new requests in the routes suite of `tools/seqtrace/demo/run.sh`
and the project-copy config they need. `.archplan/baseline.json` is not
changed: the findings are fixed, not accepted.

**Acceptance:** see the ticket. **Approach:** one request per untraced route,
with an `EXPECT` status; a `permission:` on a nav entry for the sidebar guard.
**Risk:** a new trace can raise new findings on the routes it reaches. A
traced `archplan ci` shows them. Effort: xs.
