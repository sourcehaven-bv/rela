---
id: PLAN-MQY1C5
type: planning-checklist
title: 'Planning: Restore the AC1.7 test: ACL deny returns a structured 403 body'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** PR #1029 deleted `internal/dataentry/acl_test.go` wholesale during
the `affordanceService` extraction, taking `TestHandler_ACLDeny_Returns403Structured`
with it. That was the only test pinning AC1.7 (gh#1044, CONTROL-8-29).

**Scope — IN:** one integration test restoring the contract, and correcting the
stale comment in `acl_write_test.go` that says such a test cannot be wired here.

**Scope — OUT:** any production change. The handler is correct; only its test
was lost.

**Acceptance Criteria:**

1. A `*acl.ForbiddenError` from `EntityManager` yields 403 with `rule_kind`,
   `rule_id` and `reason` in the body.
2. The denial originates in the real `AuthorizeWrite` path, not a stubbed error.
3. The scenario is *visible-but-write-denied* — a hidden entity 404s at the
   gate and never reaches the handler.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — restoring one deleted test.

**Existing Solutions:** the sixteen `acl_*_test.go` files in the package supply
everything needed: `mustNewACL`, `patchEntityAs`, `gateCtxFor`,
`newAppFromParts`/`rebindApp`, and `appbuildtest.WithDeclarative`.

**The load-bearing find:** `acl_write_test.go` claims this test *"would require
wiring the test entitymanager with the same ACL, which `newTestAppV1` does not
do"*. True when written; `appbuildtest.WithDeclarative` has since made it
possible. Without noticing that, the obvious conclusion would have been to stub
the error — which would pin the handler's formatting but not that the real write
path produces the error in the first place.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** seed an entity, build the services with
`WithDeclarative` over a policy granting `read` but not `update`, PATCH as that
principal, assert 403 and decode the body.

**Files to modify:** `internal/dataentry/acl_deny_403_test.go` (new),
`internal/dataentry/acl_write_test.go` (stale comment).

**Alternatives considered:**

1. *Stub the EntityManager to return a ForbiddenError.* Rejected — it pins the
   formatting but not that the real path produces the error, which is half the
   contract and the half that regressed.
2. *Add assertions to an existing ACL test.* Rejected — AC1.7 is a named control
   from a security review; someone auditing it should find a test named for it.
3. *Assert with substring matching on the body.* Rejected — decode instead, so a
   body that merely contains "forbidden" cannot pass.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

This restores a security *test*; it changes no behaviour. The one property worth
stating: the assertion checks the fields are **non-empty**, not their exact
values. `acl.Decision.Reason` is documented as never containing raw policy data
precisely so a 403 body cannot leak the effective-role set — asserting an exact
reason string would invite someone to widen it later to make a test pass.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** one integration test per the Approach. Its correctness is
established by mutation, not by its own green run — see the implementation
checklist for both mutations and their results.

**Edge Cases:** a no-read role 404s at the gate (already covered by
`TestACLWrite_PatchOnHiddenIs404`, and the reason this test grants read).

**Negative Tests:** the mutations *are* the negative tests — a test for a
missing test is only meaningful if it fails when the contract breaks.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| The test passes for the wrong reason (e.g. 403 from the gate, not the write path) | Grant read explicitly so the gate passes; mutation-verify |
| It re-rots the way the original did | The stale comment now points at it, so an extraction that deletes it has a second signal |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~All user-facing docs~~ (N/A: `kind=test`, no surface change)
- [x] In-tree comment in `acl_write_test.go` corrected — covered by the
      implementation checklist rather than a docs-checklist.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: restoring a
      deleted test with a contract stated verbatim in the issue. There is no
      design decision — the only judgement call, stub-vs-real-ACL, is recorded
      under Alternatives.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
