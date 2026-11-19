---
id: PLAN-WQADVFB
type: planning-checklist
title: 'Planning: Role-relation roles count for menu and dashboard permission gates'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the `permission:` of navigation entries, page tabs, spaces and
dashboard cards (list and card data), all decided by `permitsGatedUIElement`.
Out: every other entity-less permission check (commands, actions, documents,
history, config edit, comments, flow jobs, requires_permission, `rela acl map`).
They authorize an operation and stay global-only.

**Acceptance Criteria:**
1. A role held through a role relation edge on any entity opens these UX gates
(TestRoleRelationPermission_ShowsGatedUI, TestHoldsPermissionOnAnyEntity).
2. Edge `when:` and the client ceiling apply (expired-edge case, CeilingApplies).
3. One read per permission-granting role relation, once per Request
(ReadsOncePerRequest).
4. A read error counts no conferred role (ReadErrorFailsClosed).
5. Commands and the global check are unchanged (command and global columns in
the dataentry test; TestMode_RoleRelationPermissionShowsUIOnly).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change with a direct precedent)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal ACL rule)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal ACL rule)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `Request.PermitsWorld` already honours a world grant on
a relation-conferred role globally (permitsWorldThroughRelations). The batch
edge reads in localroleprime.go and aclstore.Graph.EdgesBetween show the
one-query shape.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `acl.HoldsPermissionOnAnyEntity(ctx, r, perm)`: global
roles first, then the roles conferred by role relations the member closure holds
an edge of. The conferred set is memoised on the Request (sync.Once) and read
with a new `Graph.HasOutgoingEdge` (one ListRelations over all members, zero
tail, stops at the first matching edge). Relations whose role grants no
permission are skipped. The read gate gains `HoldsPermissionOnAnyEntity`, and
`access.Mode.PermitsGatedUIElement` takes a `UIPermissionHolder` that asks it.

Alternatives: widening `Request.HoldsPermission` (rejected: it backs real gates
such as commands, history:read, config:edit and elevated documents); reading at
`ForIdentity` (rejected: adds a query to every request on every transport, and
ForIdentity is documented as graph-free).

**Files to modify:** internal/acl/{anyentityperm.go,graph.go,request.go},
internal/aclstore/graph.go, internal/access/mode.go,
internal/dataentry/readgate.go, tests, two guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** acl.yaml (operator config) and graph edges.
Edges that confer roles are already gated by `requires_permission`.

**Security-Sensitive Operations:** No data read is widened: row and field gates
are unchanged, and the gates touched only decide what to render. Operation gates
keep the global check. Errors are logged, never returned to the client.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** Group-held edge, expired conditional edge, conferred role
without the permission, role with no permissions (not read), global holder (no
read).

**Negative Tests:** Store error fails closed; client ceiling withholds; command
stays denied.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A later caller could use the new function for a real gate. Mitigated
by the godoc and the docs naming which gates use it. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry ("Hiding entries a user cannot act
on"), GUIDE-acl-security ("Sidebar menu structure is principal-independent").

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design reviewed as part of the security review of the diff)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none at planning
