---
id: PLAN-79T0MJI
type: planning-checklist
title: 'Planning: Flows: fire due runs of evicted tenants'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a sweep that fires due flow runs and realtime (channel session)
timers of tenants that are not resident. One sweep serves both. Out: running
workers outside a resident tenant.

**Acceptance Criteria:**
1. On postgres in tenant mode, a timer of a tenant nobody opens fires within SweepInterval (1 min) plus one worker tick (`TestSweeper_Pass`, `TestClusterResolver_DueTenants`).
2. The sweep reads at most one row per tenant per pass (each part of the due query reads a partial index's first row) and opens at most two tenants per pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach given in the ticket)
- [x] Searched for existing libraries that solve this problem (none needed; standard library and existing rela packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-internal lifecycle work)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: approach given in the ticket

**Existing Solutions:** Builds on the tenant registry
(`internal/tenant/registry.go`), `tenant.Provisioner`, `state.KV` and the flow
run stores.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `tenant.Sweeper` with a postgres `dueTenants` reader
that builds one due query per tenant from the columns present (pg_attribute),
each with a 5s timeout. A tenant it opens is held 30s; cooldown 10 min, doubling
up to 6h while DueAt is unchanged.

**Files to modify:** internal/tenant/sweep.go, sweep_postgres.go,
cluster_postgres.go, registry.go; cmd/rela-server/tenantmode.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No external input; schema names come from the
control table and are quoted.

**Security-Sensitive Operations:** See above. Errors name the failing step; DSNs
and tokens are not logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Sweeper unit tests with fakes; postgres test against real
tenant schemas.

**Edge Cases:** Missing tables or columns; failing tenant; list error; context
cancel; same due time forever.

**Negative Tests:** Covered in the edge cases above: failing stores, refused
orgs, closed provisioner or sweeper.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Resident budget pressure: per-pass cap. Retry loops: backoff.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-postgres-backend: sweep section.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (skipped: no separate design review was run; the finished change went through /code-review, findings recorded as review-responses)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings; code review findings are all addressed)

**Design Review Findings:** None (see review checklist for code review
findings).
