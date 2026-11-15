---
id: PLAN-DZ6EJGH
type: planning-checklist
title: 'Planning: tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In and out of scope are listed in TKT-TNPRV8 ("Scope"). In:
`organization.created` webhook provisioning, the single-flighted lazy backstop
with an operator switch, `202` plus a pollable status route, schema-name
validation, a rate cap. Added during implementation: IdP suspension sync
(`organization.updated`) and an erased-tenant tombstone. Out: routing
(TKT-TNT9RS), erasure (TKT-TNERAS), IdP reconciliation, per-tenant seed data,
SPA polling.

**Acceptance Criteria:** The five criteria in TKT-TNPRV8 ("Acceptance
criteria"). Test scenarios are under Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: schema creation and migration already exist in `tenant.Provisioner` and `pgstore.Migrate`; the work is wiring them into the host)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: RES-D54281 surveyed the approach, apartment-style schema-per-tenant)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281

**Existing Solutions:** `tenant.Provisioner.Create` (also behind `rela-server
tenant create`), `pgstore.Migrate` under an advisory lock, the
self-authenticating IdP webhook in `internal/dataentry/webhook.go`, and the
`tenanthost` resolve/acquire path from TKT-TNT9RS.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See TKT-TNPRV8 "Design decisions" and "Suspension".
Provisioning sits in `internal/tenanthost` before `Acquire`, behind the
consumer-side `Provisioner` and `StatusSyncer` interfaces. Alternatives (eager
only, lazy only, holding the request open across migration) are weighed in
RES-D54281.

**Files to modify:**
`internal/tenanthost/{host,provision,provision_http,statussync}.go`,
`internal/tenant/{cluster,cluster_postgres,provision,provision_postgres,registry,tenant}.go`,
`internal/idporg/client.go` (new),
`cmd/rela-server/{main,tenantmode,tenantcmd}.go`, `docs-project` postgres guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Verified assertion `org_id` (validated by
`tenant.SchemaFor`, `^[a-z][a-z0-9_]{0,30}$`, before any run starts); webhook
events (verified with their own `aud`); the IdP operator API answer (HMAC-signed
request, only `active`/`suspended` accepted, https, unix or loopback http only);
operator flags (validated at startup).

**Security-Sensitive Operations:** Schema creation from a claim (trust
assumption documented at `tenanthost.provisioning`, default `off`, rate cap and
running cap); suspend/resume from the IdP (never undoes an operator suspension,
serialized per org, erased tenants untouched). The status route answers only for
the caller's own verified org.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Webhook creates and resolves: `provision_test.go` webhook cases and
`cmd/rela-server/tenantmode_postgres_test.go`.
2. Lazy, single-flighted, switchable: `TestProvision_LazySingleFlight`, mode
`off` cases in `provision_test.go`.
3. 202 and pollable status: status-route cases in `provision_test.go`;
`TestProvision_RunOutlivesRequest`.
4. Trust assumption: documented at `tenanthost.provisioning` (review item).
5. Schema names validated: `TestProvision_InvalidOrgNeverReachesProvisioner`.

**Edge Cases:** concurrent requests for one org; client disconnect during a run;
another process holding the tenant lock; erased, suspended and provisioning
tenants; empty IdP status; racing `organization.updated`.

**Negative Tests:** invalid org id, rate cap exceeded (503), failed run (503
then retry), erased tenant (403), operator suspension not resumed by the IdP,
non-loopback http operator API refused at startup.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Schema creation from a claim (mitigated by default `off`, rate and
running caps, documented trust assumption). Migration time with many schemas
(separate ceiling from the resident set, see ticket). Multi-node races (advisory
lock in `pgstore.Migrate`, tenant lock, per-org sync serialization with a
recheck). Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (not created yet: docs were updated on the branch; the docs-checklist is created before done)

**Documentation Impact:** `docs-project` postgres backend guide (tenant
provisioning, `-tenant-provision`, suspension sync, tombstone) and
`docs/idp-webhook-provisioning.md`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (not run: the design came from RES-D54281; the implementation was instead reviewed by the rela-security-reviewer agent and an automated reviewer)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No design-review findings. Security review findings,
all addressed: RR-RQ2INPS, RR-G9KL9T5, RR-6EHB7NR, RR-SM1A2S7, RR-GA0WMBD.
