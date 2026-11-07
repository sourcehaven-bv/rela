---
id: PLAN-ZIU96YH
type: planning-checklist
title: 'Planning: tenant: assemble every tenant over one SharedBase so config is parsed once'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a build-agnostic `SharedBase.OpenAndAssemble(ctx, dsn)` that
opens this build's backend for one DSN and assembles the base over it; a
per-recipe `openAndAssemble` behind the build tags; a tenant opener over one
shared base replacing `AppBuildOpener`; `Registry` doc; CI and `just
test-postgres` running the tenant isolation suite. Out: wiring the registry into
rela-server, provisioning (TKT-TNPRV8), single-flighting cold opens.

**Acceptance Criteria:**
1. N tenants opened through the registry share one metamodel and one ACL policy: postgres test compares `Base().Meta()` and `ACLPolicy()` pointers across leases.
2. `TestSharedBase_AssemblyDoesNotMutateSharedValues` and the postgres isolation suite pass.
3. `Registry` doc matches.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small ticket; RES-D54281 already covers the design)
- [x] ~~Searched for existing libraries~~ (N/A: internal wiring)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal wiring)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281

**Existing Solutions:** `SharedBase.Assemble` and `ForReassembly`
(internal/appbuild/appbuild.go), `assembleSQLiteTenant` in
versionsweep_lifecycle_sqlite_test.go, the per-build `New` recipes.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `OpenAndAssemble` copies the base (the `ForReassembly`
pattern), sets the copy's `cfg.DatabaseURL` to the tenant DSN (the store and the
job queue both read it), and calls the build's `openAndAssemble`. Postgres and
memory `New` become `prepare` + `openAndAssemble`. The fs and sqlite builds
refuse: they have one project-local store, so every tenant would share it.
`tenant.SharedBaseOpener(base)` rejects a nil base and opens with
`context.WithoutCancel`. Rejected: an `Opener` taking store/search directly in
the tenant package (would import pgstore there and break the build-tag split);
passing the DSN through a new backend argument to `openBackend` (more churn in
every recipe for no gain).

**Files to modify:** internal/appbuild/appbuild.go, appbuild_postgres.go,
appbuild_memory.go, appbuild_fs.go, appbuild_sqlite.go,
internal/tenant/registry.go, internal/tenant/isolation_postgres_test.go, new
default-build tests, .github/workflows/ci.yml, justfile.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The DSN comes from the tenant map, already
validated and schema-pinned by the resolver. An empty DSN is refused by the
postgres recipe.

**Security-Sensitive Operations:** Tenant isolation. Each assembly gets its own
pool from its own DSN; the shared base holds only config-derived values. The
fs/sqlite builds now refuse instead of silently serving one shared store to
every tenant.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: postgres isolation test acquires two tenants and
asserts identical meta and ACL policy pointers, distinct stores. AC2: run
sharedbase_test and the isolation suite. AC3: doc review.

**Edge Cases:** A base marked for reassembly is refused. A base whose config
holds another DSN: the tenant DSN wins (isolation test uses a base with no DSN;
copy always overrides).

**Negative Tests:** `SharedBaseOpener(nil)` errors. Default (fs) build:
`OpenAndAssemble` errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Sharing `opts` across assemblies: same as one `prepare` today in the
single-store path; per-store state is built in `assemble`. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal wiring, nothing user-facing)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review, proportionate to an s ticket: found
that the tenant postgres suite never ran in CI (added to the plan); found that
the fs build silently gave every tenant one store (now refused). No
review-response entities needed.
