---
id: PLAN-6KZA4T4
type: planning-checklist
title: 'Planning: tenant: erase an org — DROP SCHEMA CASCADE, a documented retention SLA, and the audit-log residue'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: `rela-server tenant erase`, the `erase: true` mark, the schema advisory
lock held by open tenants, removal of the per-tenant audit directory, the
host-level erasure record, the organization.deleted erasure-needed log, and
the docs (retention guarantee, procedure, limits). Out: provisioning
(TKT-TNPRV8), automatic erasure from the webhook, per-entity purge.

**Acceptance Criteria:**
See the ticket's revised criteria 1-6; each names its test.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-D54281 already chose schema-per-tenant and DROP SCHEMA erasure)
- [x] Searched for existing libraries that solve this problem (PostgreSQL advisory locks and pgx.Identifier; no library needed)
- [x] Checked codebase for similar patterns or reusable code (pgstore advisory-lock migrations, history purge confirmation prompt, backendtest schema helpers)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: standard DROP SCHEMA CASCADE pattern)
- [x] Reviewed relevant rela concepts for prior art (RES-D54281, TKT-SNVR8UV)

**Research Doc:** RES-D54281 (informs TKT-TNERAS)

**Existing Solutions:**
`internal/cli/history_purge.go` types-the-id confirmation; pgstore uses
single-key advisory locks, so erasure uses the two-key space.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Operator command in rela-server. Mark in tenants.yaml, restart, erase.
Open tenants hold `pg_advisory_lock_shared(class, hashtext(schema))` on a
dedicated connection; erase takes it exclusively, refuses when `public`
holds tables, drops the quoted schema, then removes the audit directory.
Rejected: erasing from the webhook (irreversible, weaker authorization);
in-process erasure in the server (several servers share one map); per-pool
AfterConnect locks (the neoq pool takes only a DSN).

**Files to modify:**
internal/tenant/{config,erase,erase_postgres,registry,tenant,dsn_*}.go,
cmd/rela-server/{main,tenantmode,tenanterase}.go, internal/dataentry/webhook.go,
internal/appbuild/backendtest/backendtest_postgres.go, GUIDE-postgres-backend.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
org id (argument; must name a marked tenant), tenants.yaml (validated as the
server does; schema allowlist pattern plus reserved names), typed
confirmation (exact match), DSN (file or env only, never printed).

**Security-Sensitive Operations:**
DROP SCHEMA (quoted identifier, exclusive lock, public-tables guard);
RemoveAll of the audit directory (single path element, symlink refused).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Postgres tests on private databases: two-tenant erase, open-tenant refusal,
open during erasure refused, lost lock fails closed, public-tables refusal,
audit failures; CLI end-to-end with typed confirmation.

**Edge Cases:**
Rerun after a partial erasure; missing schema; missing audit directory;
symlinked audit directory; every tenant marked.

**Negative Tests:**
Unknown, unmarked or empty org; shared, traversing or quoted schema names;
confirmation mismatch or EOF; usage errors; non-postgres build.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Lock lost while a stale server keeps the tenant open: mitigated by the
public-tables guard and the required restart (documented). Wrong database
via RELA_DATABASE_URL: the prompt names the target. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
GUIDE-postgres-backend "Erasing a tenant".

## Design Review

- [x] Run `/design-review` before starting implementation (design and code reviewed together by cranky-code-reviewer and rela-security-reviewer)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-7VW31EB, RR-FRPAC4U, RR-TS7HEYH, RR-QZT22A4, RR-Y92JSZ9 (significant, addressed) and ten minor/nit responses linked to TKT-TNERAS
