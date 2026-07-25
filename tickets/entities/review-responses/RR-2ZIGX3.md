---
id: RR-2ZIGX3
type: review-response
title: Detect checked assignment existence, not whether the scheduler could actually read
finding: |-
    Detect's predicate was `does an assignments key for the scheduler exist`. That goes quiet on three states where the scheduler still reads nothing:

    1. The assignment names an UNDEFINED role (the dangling case produced by the null-roles bug).
    2. The assigned role EXISTS but grants no read (`scheduler-system: {}`, or a write-only role). The old ensureRole guard was `GetMapValue(roles, name) != nil` — mere existence, not usefulness — so it deferred to an operator role that granted nothing, relocating the bug rather than fixing it. The original RespectsOperatorRole test happened to use `read: [ticket]`, the one variant where deferring is correct.
    3. asserted_role_assignments was never consulted, so an operator who had ALREADY scoped the scheduler there got a redundant read:["*"] role piled on top of their deliberately narrower grant — a silent widening. The Detect godoc claimed such operators were "left alone", which was false for that path.
severity: significant
resolution: |-
    Detect now asks the real question via a shared schedulerCanRead(root) predicate: is the scheduler bound to a role that EXISTS and lists at least one read target? It consults both assignments (principal->role) and asserted_role_assignments (principal->RoleList, scalar-or-sequence).

    ensureRole/ensureAssignment now repair a dead role instead of binding to it, while KEEPING the operator's role name and definition (only adding a read list where there was none). An existing operator assignment is never repointed.

    Apply asserts schedulerCanRead as a POSTCONDITION and returns an error when unmet — the runner then stops before writing, so a policy it cannot fix leaves the operator's file untouched with a loud, actionable error instead of a silent false success.

    Mutation-tested: reverting Detect to the bare key check fails 5 tests; removing the dead-role repair fails 2; removing the postcondition fails 1 (a test was added specifically for it, since the postcondition was initially unguarded).
status: addressed
---
