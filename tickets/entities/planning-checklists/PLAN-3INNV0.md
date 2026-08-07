---
id: PLAN-3INNV0
type: planning-checklist
title: 'Planning: Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: implement `unmatched_principal: provision` — lazily create a bare
stub user entity for a verified JWT principal whose subject resolves to no user
entity, on the principal's first authorized write; the triggering write then
proceeds and sees the new entity. OUT: group/local-role membership at provision
time (bare stub only, RR-28SCW3); IdP-webhook enrichment (separate path); org
*relations* (properties only).

**Acceptance Criteria:** (from the ticket, finalised)
1. `provision` ⇒ first authorized write by an unmatched verified principal creates the stub (keyed on `principal_property`=sub, plus email/org_id/org_slug when the user type declares them) under `system:provisioner`; a GET does not provision.
2. The triggering write sees its own newly-provisioned entity (re-stamp reaches the manager; read gate rebuilt) — no one-request-behind.
3. Concurrent first-writes from one sub create exactly one entity.
4. `provision` without the system principal/grant wired fails at LOAD.
5. The stub create is audited to `system:provisioner`.
6. Provisioning covers every entity-write path (CRUD, sync, action, attachment), not just CRUD — anti-bypass invariant, pinned like the reject test.

## Research

- [x] ~~run `/research`~~ (N/A: approach settled with user; design parked in `.ignored/provision-unmatched-principal-design.md`)
- [x] Searched for existing libraries — N/A, internal wiring
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations — prior art below
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (design parked in `.ignored/`)

**Existing Solutions / Prior art:**
- `principal.UserScheduler` + `migration/acl_scheduler_grant.go` — system-principal + migrated-grant pattern (mirror for `system:provisioner`).
- `webhook.go dispatchWebhookAction` — provisioning write under a dedicated principal, under `writeMu` (the other path that creates the same person).
- `Declarative.AuthorizeWrite` opening a FRESH Request from `principal.From(ctx)` with a live graph walk — why a re-stamp lets the triggering write see the new entity.
- `rela.bypass_acl` (TKT-D8T148/ACSBSA) — REJECTED as the mechanism: it is write-time *user-Lua* elevation wired at automation sites, over-grants (raw ungated reader + full-bypass manager), and doesn't address the ctx seam. Provision reuses only its *discipline* (elevated handle wired at exactly one site + its own audit op), with a create-user-only manager.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** Write-seam = **candidate (a): a write-only middleware
wrapping the mutating routes** (settled; RR-VI9XMY addressed).
`attachACLRequest` resolves the principal as today. A new write-middleware —
which owns `writeMu` + the manager, NOT the read middleware (avoids the
downstream-handler deadlock) — when it sees unmatched-verified AND
`unmatched_principal:provision`: provisions the bare stub under
`system:provisioner`, then **rebuilds both `acl.WithRequest` and the read gate
on the re-stamped ctx** and does `r = r.WithContext(newCtx)` before delegating.
Wrapping the routes (not per-handler) gives the anti-bypass coverage reject got
from the shared choke point.

Concurrency: rely on the `sub` `unique:true` constraint — both concurrent
first-writes may attempt the create; loser catches `store.ErrConflict`,
re-resolves, proceeds. No new lock beyond `writeMu`; correct on fs/mem/postgres
and multi-process.

**Files to modify (anticipated):**
- `internal/principal/` — `UserProvisioner`/`ToolProvisioner`; thread `email` through `Verified`/`Sanitized`/`Equal`/JSON.
- `internal/migration/acl_provisioner_grant.go` (new, mirrors `acl_scheduler_grant.go`) + lockstep literal-equality test.
- `internal/dataentry/` — write-middleware + provision helper; thread `email` through `AssertedIdentity`/`verifiedPrincipal`.
- `internal/acl/` — LOAD check that provision requires the system principal/grant wired (AC4).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the stub is built ONLY from a
**cryptographically verified** assertion (`principal.Verified` trust boundary) —
sub/email/org from the JWT, never from unverified headers. Org/email land as
properties only if the user type declares them (else soft-warn + drop). No
user-supplied Lua on this path.

**Security-Sensitive Operations:** the create runs under `system:provisioner`, a
create-user-only principal (RR-28SCW3 containment — cannot author edges, so no
cascade-authz contradiction). Audited to `system:provisioner` with its own op.
Provisioning is gated on unmatched-verified from the JWT gate only
(header/CLI/scheduler never trigger it).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** one e2e test (mirroring `unmatched_principal_e2e_test.go`)
driving an unmatched verified assertion through the REAL router: (AC1) first
write provisions + proceeds; (AC1) GET does not provision; (AC2) triggering
write reads back its own stub; (AC6) provisioning fires on CRUD, sync, and
action paths (distinct-handler coverage). Unit: (AC4) load fails without the
grant; (AC5) audit attribution = `system:provisioner`; migration lockstep
equality test.

**Edge Cases:** (AC3) concurrent first-writes from one sub → exactly one entity
(catch-and-re-resolve test on `store.ErrConflict`); user type does NOT declare
email/org → soft-warn + drop, stub still created with sub; matched principal →
never provisions.

**Negative Tests:** provision configured but system principal/grant absent →
LOAD error; header/CLI principal that is unmatched → NOT provisioned (flag
scope, like reject AC4).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:** (1) fs/mem non-atomic unique → duplicate stubs under multi-process →
mitigated by catch-`ErrConflict`+re-resolve; pgstore partial unique index is the
full close (documented as backend note). (2) Webhook + lazy-provision both
create the same person → reconcile through one shared idempotent helper (or
document fs-backend limitation). (3) `principal.Principal` field growth from
`email` → consider a claims struct instead of a 6th `Verified` param. Effort:
**m**.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/acl-security.md` / `docs-project/entities/` — document `unmatched_principal: provision` semantics and operator responsibility (bare stub; groups arrive via webhook/reconcile/admin).
- [x] CLAUDE.md-adjacent — the `system:provisioner` create-user-only containment note.

## Design Review

- [x] ~~Run `/design-review`~~ (design reviewed with user; two findings recorded)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-28SCW3 (addressed — bare-stub containment),
RR-VI9XMY (addressed — write-middleware seam + unique-constraint concurrency).
