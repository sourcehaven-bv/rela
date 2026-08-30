---
id: PLAN-M43BQF
type: planning-checklist
title: 'Planning: Regression test for empty FromType against a type-scoped policy'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `CreateRelation` computes a best-effort — possibly empty —
`FromType` before the ACL check, so authorization does not depend on peer
existence (BUG-K6FEVB). The existing tests cover that only against
`acl.ReadOnlyACL` (denies all) and `acl.NopACL` (allows all). Neither is
type-scoped, so neither can show what an empty `FromType` does against a
realistic `acl.yaml`. GitHub issue #1129 (IB-review rela#1115), CONTROL-5-15.

The safety argument exists and looks right — but as **prose**. Nothing fails if
a future change to `grantsVerb`, or to how `FromType` is derived, makes `""`
match something it should not. The ACL has been in production since 2026-07-07.

**Scope — IN:** a table test over realistic grant shapes, asserting an
unresolvable source is never authorized where the resolved one is refused.

**Scope — OUT:**

- Any production change. The behaviour is believed correct; this pins it.
- `UpdateRelation` / `DeleteRelation`. They share `authorizeRelationWrite` and
the same `FromType` derivation, so a per-verb copy would triple the table to
re-exercise one function. If they ever diverge, that is when to split.
- Client-baseline ceilings — see the Test Plan for why the one bypass shape they
would cover is out of reach here.

**Acceptance Criteria:**

1. For every realistic grant shape, absent-source is never *more* permissive
than present-source.
2. The realistic grant (`create: [decision]`) is allowed with the source and
**denied without** — the fail-closed claim, stated positively.
3. Any configuration where the invariant does not hold is **named in the test**,
not silently absent from the table.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — one table test.

**Existing Solutions:** `internal/entitymanager/acl_bypass_test.go` and
`internal/dataentry/acl_relation_write_bypass_test.go` are the tests the issue
says are insufficient — and it is right about why: both use all-or-nothing ACLs.
`acl_test.go`'s `newManagerWithACL` and `cascade_authz_test.go`'s
`acl.NewDeclarative` pattern give the type-scoped setup they lack.

**The load-bearing find — the issue's mental model of the gate is slightly off,
and so was mine.** `authorizeRelationWrite` keys the type-level gate on the
**source entity's type**, not the relation type:

> Type-level gate: principal needs the matching verb grant on the source
> entity's type. A relation create checks the source type's `create` grant
> (consistent with entity create).

My first table granted `create: [addresses]` (the relation type) and every row
failed. Reading the gate rather than adjusting the expectations is what produced
a table that tests the real model.

The same function already documents the exact property under test:

> Four of the five RelationSubject call sites leave it empty when the source
> entity is missing or unreadable, and today that fails closed because no role
> lists "". Honoring a relation grant on an empty FromType would silently turn
> "source unresolvable ⇒ deny" into "⇒ allow".

So this ticket converts a comment into a gate.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** a table over grant shapes; each row runs
`CreateRelation` twice — source seeded and source absent — against a real
`acl.Declarative`. A helper distinguishes an ACL denial (`*acl.ForbiddenError`)
from the not-found error that follows it, so "allowed" means "got past the
gate", which is the thing under test.

**Files to modify:** `internal/entitymanager/acl_empty_fromtype_test.go` (new).

**Alternatives considered:**

1. *Assert on `acl.Request` directly.* Rejected — it would test `grantsVerb` in
isolation and miss the derivation of `FromType`, which is half the concern.
2. *Extend an existing bypass test.* Rejected — those are about the elevated
handle; this is about type scoping, and merging them makes both harder to read.
3. *Fix the empty-string-grant case.* Rejected — see AC3 and the Risk table.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none — a test.

**Security-Sensitive Operations:** the subject is an authorization decision, so
the framing matters: the test asserts a **direction** (never more permissive),
not a specific verdict. A test asserting exact verdicts would pass while the
policy model changed underneath it; a directional invariant is what survives a
refactor of `grantsVerb`.

The `create: [""]` finding is real behaviour, not a vulnerability: it requires
an operator to write an empty-string grant deliberately in `acl.yaml`. It is
documented in the test rather than fixed, because the fix (rejecting empty
strings at policy load) is a separate change with its own migration question for
existing policies.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** five rows — grant on the source type, no grant, grant on a
different type, wildcard, and the literal empty-string grant — each run with and
without the source entity.

**Edge Cases:** the wildcard row is included precisely because it is the one
realistic shape where an empty type *does* match, and to record that it was
already that permissive before this code path existed.

**Negative Tests:** the invariant assertion is the negative test, and it fires
independently of the per-row expectations — so a row whose expectations are
wrong still cannot hide a bypass.

**Scope of what mutation testing can reach here (established, not assumed):**
substituting `"*"` for an empty `FromType` inside `authorizeRelationWrite` is
caught only by the `create: [""]` row, because with `create: [decision]` an
empty type fails to match either way. Catching that substitution on a realistic
grant needs a client-baseline ceiling — the one mechanism that denies a type a
role holds `*` on — which is a different subsystem. That limit is written into
the test's doc rather than left for a reader to discover.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| The test encodes my model of the gate rather than the gate | It did, at first — every row failed. Fixed by reading `authorizeRelationWrite`, not by adjusting expectations |
| A row's expectations are wrong, hiding a bypass | The directional invariant is asserted separately from the per-row wants |
| The empty-string exemption silently becomes wrong | Asserted in BOTH directions: the test fails if the exemption stops applying, so it cannot rot into a permanent excuse |
| A reader over-trusts the coverage | The doc states which bypass shape it cannot reach, and why |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~All user-facing docs~~ (N/A: `kind=test`, no behaviour or surface change)
- [x] The test's own doc comment carries the finding — that `create: [""]` is a
real (if unwritable-by-accident) exemption — since that is the one piece of new
knowledge this ticket produced.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: adding a test
for stated, unchanged behaviour. The judgement calls — directional invariant
over exact verdicts; document the empty-string case rather than fix it — are
recorded under Alternatives and Security.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
