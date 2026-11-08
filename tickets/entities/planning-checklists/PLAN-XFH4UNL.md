---
id: PLAN-XFH4UNL
type: planning-checklist
title: 'Planning: rela-server: multi-tenant host that routes a verified org_id to its tenant''s App'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `tenanthost` front handler that verifies the assertion once,
routes by `org_id` through `tenant.Registry` and serves that tenant's App;
rela-server tenant mode when `tenants.yaml` exists; a per-tenant audit sink
(TKT-SNVR8UV); a per-tenant Lua engine and collab relay; docs. Out: provisioning
(TKT-TNPRV8), erasure (TKT-TNERAS), scheduler, CLI and stdio MCP (TKT-M3SHYE0),
anonymous tenant choice (TKT-8D1I8PA), config hot reload and Configure in tenant
mode.

**Acceptance Criteria:**
1. Two tenants cannot read each other's data through the HTTP API.
2. No assertion or an invalid one: 401, no store opened.
3. Verified, but no or unknown org: 403 `unknown_tenant`, no store opened.
4. Resolver or open failure: 503, never a default store.
5. Without tenants.yaml rela-server behaves as today.
6. Each tenant's audit records land in its own directory.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281

**Existing Solutions:** `tenant.Registry` and `appbuild.SharedBase` (TKT-P938T7)
already exist; the host reuses rela-server's `buildApp` and the dataentry JWT
gate. Schema per tenant follows RES-D54281.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See `.ignored/d6mf3sw/design.md`. The registry becomes
generic over its resident value so one resident holds `{Services, App, handler,
relay, workers}`. The host verifies the assertion, memoises the verified
identity on ctx for the tenant App's gate, acquires a lease for the request and
serves the tenant handler. Rejected: tenant routing inside `dataentry.App` (god
object) and one shared App over a switching store (state captured per App).

**Files to modify:** internal/tenant/registry.go, config.go; new
internal/tenanthost; internal/appbuild/appbuild.go;
internal/audit/filesystem.go; cmd/rela-server/main.go, serve.go;
internal/dataentry (StaticShell); .go-arch-lint.yml; docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The assertion header and the webhook body JWT
are verified by jwtauth before `org_id` is read. `org_id` is resolved against
the `tenants.yaml` allowlist. The request path is cleaned before the tenant-free
table is consulted.

**Security-Sensitive Operations:** Opening a tenant store by DSN: the DSN never
reaches logs or flags. Logs carry only org_id and schema. Denials open no store.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** 1: postgres two-tenant HTTP test. 2-4: tenanthost unit tests
with a fake acquire that counts opens. 5: existing rela-server tests unchanged.
6: postgres audit isolation test.

**Edge Cases:** Path cleaning (`/static/../api/...`), empty org, eviction during
a request, revive of an evicting resident, double release, concurrent acquire.

**Negative Tests:** Every fail-closed path above; boot refusal on non-postgres
builds and on conflicting identity flags.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Cross-tenant leaks through process-wide state (Lua cache, collab
relay, audit sink): each is made per tenant and pinned by a test. Connection
budget: bounded resident set. Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-security.md (`org_id` section), postgres
backend docs for tenant mode.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-ZIKLZ02, RR-Q6K62YV, RR-42CM1YD, RR-4R3J2XM,
RR-3P2TTKY, RR-NJ7IG1V, RR-3WPKYPW, RR-BJWUKKX
