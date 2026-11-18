---
id: PLAN-5LLVQ2K
type: planning-checklist
title: 'Planning: tenant mode: background work and webhook dedup across eviction; faster shutdown'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: background work of evicted tenants (due-work sweep), webhook
dedup across eviction (BUG-ZO33J9M), faster tenant-mode shutdown (retire
streams, close residents in parallel). Out: a reconnect-later hint for pinned
streams; `max_open` remains the bound.

**Acceptance Criteria:**
1. A tenant with due work that nobody opens is opened by the sweep (`TestSweeper_Pass`, `TestClusterResolver_DueTenants`).
2. A replayed webhook is refused after eviction (`TestSeenSet_SurvivesEviction`).
3. Shutdown ends streams and closes residents in parallel (`TestRegistry_RevokeAll`, `TestRegistry_CloseIsParallel`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach given in the ticket and TKT-ZOU5BCT)
- [x] Searched for existing libraries that solve this problem (none needed; standard library and existing rela packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-internal lifecycle work)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: approach given in the ticket and TKT-ZOU5BCT

**Existing Solutions:** Builds on the tenant registry
(`internal/tenant/registry.go`), `tenant.Provisioner`, `state.KV` and the flow
run stores.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A control-plane `Sweeper` lists active tenants whose
earliest due work (flow runs, channel session timers, queued jobs) is due and
opens at most two per pass for 30s, with a cooldown that doubles while DueAt is
unchanged, up to 6h. The webhook seen-set moves to the tenant's `state.KV` with
a TTL and fails closed (503). A shutdown hook (`RegisterOnShutdown`) revokes
every resident's streams and `Registry.Close` closes residents in parallel.
Rejected: keeping residents with queued work open until drained (breaks the
resident budget).

**Files to modify:** internal/tenant/sweep.go, sweep_postgres.go, registry.go,
cluster_postgres.go; internal/dataentry/webhook.go; internal/tenanthost/host.go;
cmd/rela-server/tenantmode.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new external input. The sweep reads
control-plane and tenant schemas with per-tenant timeouts. Webhook dedup fails
closed so a replay is never accepted when state is unavailable.

**Security-Sensitive Operations:** See above. Errors name the failing step; DSNs
and tokens are not logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Fake lister and registry tests for the sweeper; postgres
test for DueTenants on real schemas; seen-set tests with a broken KV; registry
tests for revoke and parallel close.

**Edge Cases:** Tenant with an old schema missing a column (left out via
pg_attribute); slow tenant (5s timeout, skipped); list failure; cancel while
holding a tenant; due work that never clears (backoff to 6h).

**Negative Tests:** Covered in the edge cases above: failing stores, refused
orgs, closed provisioner or sweeper.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Sweep opening tenants in a loop: cooldown with backoff and per-pass
cap. Sweep pushing active tenants out: per-pass cap of 2.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-postgres-backend: due-work sweep and shutdown
behaviour.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (skipped: no separate design review was run; the finished change went through /code-review, findings recorded as review-responses)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings; code review findings are all addressed)

**Design Review Findings:** None (see review checklist for code review
findings).
