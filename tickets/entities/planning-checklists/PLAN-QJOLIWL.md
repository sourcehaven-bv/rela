---
id: PLAN-QJOLIWL
type: planning-checklist
title: 'Planning: tenant: choose the tenant for anonymous requests (or decide there is none)'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a decision record choosing "no anonymous tenant access", and a
docs note in the tenant mode section. Out: any code change; host-to-org mapping,
org slugs in paths, and shared read-only links.

**Acceptance Criteria:**
1. DEC-9EZ3E8H states the option and the reasons.
2. N/A: the chosen option is "none", so no cross-tenant test is needed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: decision made by the project owner; options listed in the ticket)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: no code change)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: no code change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Tenant mode already fails closed on a missing `org_id`
(GUIDE-postgres-backend, "How a request is routed"). DEC-PEYCJZ already
re-scoped public read to authenticated users.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Record DEC-9EZ3E8H and document the behaviour.
Host-to-org mapping and org slugs were rejected: every network deployment runs
behind Pratique with a verified `org_id`, and public content belongs in Pratique
or a separate publishing service. No dependencies.

**Files to modify:** docs-project/entities/guides/GUIDE-postgres-backend.md,
docs/postgres-backend.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no code change)

**Input Sources & Validation:** The only input is the Pratique assertion; its
`org_id` remains the sole tenant selector.

**Security-Sensitive Operations:** Tenant selection. The decision keeps the
fail-closed path; no caller-set value can pick a tenant.

## Test Plan

- [x] ~~Test scenarios documented for each acceptance criterion~~ (N/A: no code change)
- [x] ~~Edge cases identified and documented~~ (N/A: no code change)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: existing tenant routing tests cover the missing-org refusal)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: no code change)

**Test Scenarios:** N/A

**Edge Cases:** N/A

**Negative Tests:** N/A

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None: the behaviour does not change. Effort: xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/postgres-backend.md (via docs-project) - tenant mode note

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: decision only, no design to implement)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
