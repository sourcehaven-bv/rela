---
id: PLAN-UEBU3BL
type: planning-checklist
title: 'Planning: Conditional grant edges: when: on role_relations and membership'
started: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `when:` condition on a `role_relations:` entry, including the
membership relation's entry; lowering into a store query on every backend; read
time from internal/readtime; audit finding; docs. Out: `now()` (arrives with
TKT-08ZI5XN); conditions on `inherit_roles_through` hops; write-time 422
validation of the condition properties (fail-closed reads cover it); a scheduled
expiry job.

**Acceptance Criteria:**
1. A grant edge with `valid_until` today or later confers its role; the day after, it does not. Test: TestConditionalGrant_GoAndStoreAgree.
2. Go probes, the store row gate, the primed batch and LocalRolePredicates agree. Same test.
3. An expired membership edge is not walked. Test: TestConditionalMembership.
4. A missing or malformed date, or no read time, grants nothing. Tests: TestConditionalGrant_GoAndStoreAgree (no read time), TestGraph_EdgeCondition, RunEdgePropsTests.
5. A condition the store cannot answer exactly fails the policy load. Test: TestValidateACLPolicy_GrantConditions.
6. `rela acl audit` reports each conditional grant. Test: TestAudit_A16_ConditionalGrant.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-Z5PYPDA)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Z5PYPDA

**Existing Solutions:** Option A (edge properties) from RES-Z5PYPDA. Reuses the
`visible:` lowering pattern (queryplan, exact store predicates), predicate's
Conjunction for the parse, and storetest's differential harness.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** The policy compiles each `when:` at load (appbuild,
against the relation's declared properties) into acl.GrantCondition terms. A
Request binds them once at its identity's read time into store.PropPredicate
lists with day arithmetic. RelationPredicate.EdgeProps applies them to the
conferring edge in every backend; acl.Graph lookups take the same predicates.

**Files to modify:** internal/store (graphquery, naive, pgstore, sqlitestore,
storetest), internal/acl, internal/aclstore,
internal/affordances/searchscope.go, internal/appbuild (grantcond, flowsteps,
identity), internal/aclaudit, internal/predicate/ordered.go, internal/dataentry
(router, watcher), docs-project ACL guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** acl.yaml `when:` (operator config): allowlisted
grammar, refused at load otherwise. Edge property values (user data): a
PropIsDate shape guard before every ordered comparison; a non-date grants
nothing.

**Security-Sensitive Operations:** Granting roles. A `when:` requires
`requires_permission:` on the entry, so only delegates can set or extend the
date (RR-3A6B31G).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria. Store conformance
(RunEdgePropsTests) and the graph differential run on memstore, fsstore, sqlite
and postgres.

**Edge Cases:** Last day at 23:59:59 UTC; midnight; a non-UTC read time; a
stored `TBD` or number; an absent property; a fs-decoded time.Time value; two
terms on one property; mixed conditional and unconditional conferring types.

**Negative Tests:** `or`, `not`, `~=`, datetime, custom date format, list,
undeclared property, the reserved word `until`, an uncompiled `when:`, a `when:`
without requires_permission.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Paths that disagree (Go vs SQL): pinned by the agreement test and the
differential. A missed acl.Graph lookup: pinned by TestGraph_EdgeCondition.
Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-acl-overview (new section), GUIDE-acl-security
(threat notes, audit info severity).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-3A6B31G, RR-9CJES4V, RR-D2UZU0V, RR-D8X02UC,
RR-FMT4AFT, RR-HN6I1U7, RR-O9NYUF7, RR-WF0WDIS, RR-Y5XSQ2Y
