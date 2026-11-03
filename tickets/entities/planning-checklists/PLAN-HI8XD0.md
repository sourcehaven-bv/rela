---
id: PLAN-HI8XD0
type: planning-checklist
title: 'Planning: Pin the write-ACL refusal of relations a conversion removes'
started: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: one test in `internal/entitymanager/conversion_test.go`. Out: production
code changes; commit 563989583 already implements the ACL check, audit and
versioning that GitHub #1900 asks for.

**Acceptance Criteria:**
1. A conversion that must drop an incoming relation from a type the caller
   cannot write fails with `acl.ErrForbidden`.
2. That refusal writes one `denied-write` audit record and no other record.
3. The relation is still stored and the source entity is not converted.
4. With the write grant, the same conversion succeeds and removes the relation.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one test)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: test of existing code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: test of existing code)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
Reuses `newConversionFixture`, `conversionPolicy` and the table shape of
`TestConvert_ClientCeilingBoundsTheDestination`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Extend the fixture schema with a `cites` relation from `peer` to `note`
mapped to `drop`. Run the conversion as alice with and without update and
delete grants on `peer`. An outgoing edge was rejected as the subject because
alice holds write on `note`, so it would not exercise the incoming-edge path.

**Files to modify:**
`internal/entitymanager/conversion_test.go`

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: test only)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: test only)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: test only)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:**
The relation write-ACL check in `authorizeCascadeRelations`, which the test pins.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: the fixture uses the real Declarative ACL and memstore)

**Test Scenarios:**
Table test with two cases: grant present (criterion 4), grant absent
(criteria 1 to 3).

**Edge Cases:**
The denied case also checks that the source still exists, so a partial write
on a non-transactional store would be caught.

**Negative Tests:**
The denied case is the negative test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
None; test only. Effort xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: test only)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: test ticket)

**Documentation Impact:**
N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: test-only change with no design surface)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
