---
id: PLAN-L80FO7K
type: planning-checklist
title: 'Planning: Tenant review follow-ups: validate org_id on the request path, fix upgrade guide and plan scope'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: SchemaFor refuses an org id that checkOrgID refuses, so the
request path, Lookup, the provisioner and the CLI all apply the same check
(#2174); the threat model in PLAN-W11X48M says where the check runs; the
upgrade guide derives the schema name without the control table (#2172);
PLAN-W11X48M scope lists CLI revalidation as in scope (#2173). Out: changing
`tenant show` itself.

**Acceptance Criteria:**
1. An org id that is too long, not UTF-8 or holds a control character is ErrUnknownTenant from SchemaFor and from ClusterResolver.Resolve, and the control table is not read: TestSchemaFor, TestClusterResolver_RefusesInvalidOrgID.
2. The guide's step 1 works before the first tenant exists: a SQL expression that matches SchemaFor, checked by hand against `tenant show` output.
3. PLAN-W11X48M scope and security text match the code: reviewed in the diff.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small review follow-up)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reuses checkOrgID)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: small review follow-up)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: small review follow-up)

**Research Doc:** N/A

**Existing Solutions:** checkOrgID in internal/tenant/cluster.go already holds
the rule; Provisioner.Create calls it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** SchemaFor calls checkOrgID and wraps its error in
ErrUnknownTenant. Every path derives the schema through SchemaFor, so one
change covers Resolve, Lookup, Provisioner.record and the CLI. tenanthost
already maps ErrUnknownTenant to a refusal. Alternative rejected: a separate
call in Lookup only, which leaves the CLI and provisioner paths unchecked.

**Files to modify:** internal/tenant/cluster.go, internal/tenant/cluster_test.go,
docs-project/entities/guides/GUIDE-postgres-backend.md, docs/postgres-backend.md,
tickets/entities/planning-checklists/PLAN-W11X48M.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** org_id from a verified JWT or the operator CLI;
now checked by checkOrgID on every path before the schema is derived. A
refused org id is an unknown tenant.

**Security-Sensitive Operations:** None new. The check fails closed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: the resolver test runs over the control reader seam; tenanthost already tests the ErrUnknownTenant mapping)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Exactly 256 bytes is accepted, 257 refused; invalid UTF-8;
NUL, newline and DEL; empty.

**Negative Tests:** Each invalid org id is ErrUnknownTenant and the control
reader is not called.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A live tenant with an org id over 256 bytes would stop resolving.
None can exist: Create has refused such ids since the control table was
introduced. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore; the guide fix is part of the change)

**Documentation Impact:** GUIDE-postgres-backend (upgrade step 1).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: review follow-up with the fix named in the issues; design reviewed in TKT-HF5KE12)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
