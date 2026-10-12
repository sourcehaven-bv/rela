---
id: PLAN-HYRAIJ
type: planning-checklist
title: 'Planning: writes require read access to the existing row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The entity manager authorizes a write on type, id and face, but
does not check that the caller can read the existing row. `DeleteEntityFace` and
`RenameEntity` do. Update, patch, delete, soft delete, the copy target and
relation writes do not, so any entry point that does not resolve the target
through the read gate first (CalDAV PUT and DELETE today; any script or
automation written against the raw id) can write to a row the caller cannot
read, and the answer confirms the row exists.

**Scope — IS in scope:** the read check in the manager for update, patch,
delete, soft delete and the copy target; endpoint visibility for relation
create, update and delete; CalDAV PUT answering a hidden row like a deleted one;
docs.

**Scope — NOT in scope:** conflict resolve (BUG-0E6S7Q), the IdP webhook
principal name (BUG-S6S1Z0), CLI rename without a principal (BUG-AVHQA6),
raw-store writers (migrations, importer, normalize), version history.

**Decisions (Jeroen, 12 Oct):** an update that makes the row unreadable to the
writer afterwards is allowed; relation writes are in this change; the other gaps
get their own tickets.

**Acceptance Criteria:**
1. Patch, update and delete of a row the caller cannot read give the same error
as for an id that does not exist, and leave the row unchanged.
2. Relation create to a hidden head or from a hidden tail gives the same error
as for a missing endpoint; relation update and delete of a hidden edge give the
same error as for a missing edge, and leave it in place.
3. A copy onto a hidden stored target is refused as a taken id and does not
name its type.
4. A CalDAV PUT to a hidden to-do gets the same 404 as one to a deleted to-do.
5. Readable rows and edges are written as before.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: reuses existing helpers)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal seam)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. An inventory of every write path and its principal was
made first; its gaps are listed under scope.

**Existing Solutions:** `faceGateOf`, `readableRows` and `requireReadableRow`
(manager.go) already do this for `DeleteEntityFace` and `RenameEntity`.
`edgeVisibility` (hiddenedges.go) restates the read path's edge rule
(`visibility.EndpointsReadable`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** call `requireReadableRow` right after each existing-row
read and before the ACL check. For relations, load `edgeVisibility` for the key
after the ACL check (BUG-K6FEVB order kept) and report a hidden endpoint or edge
with the existing not-found error. The copy probe gates a stored target and
answers `ErrEntityAlreadyExists`. CalDAV PUT treats a gated miss as gone.

Rejected: a check inside `authorizeAndAudit` (it has no row and also serves
creates and affordance probes); per-handler checks (they drift, and the
arch/acl-store-adapters refactor relies on one manager-side rule).

**Files to modify:**
internal/entitymanager/{manager,softdelete,copy,hiddenedges}.go,
internal/entitymanager/CLAUDE.md,
internal/dataentry/{caldav_write,copies_handler}.go, the ACL guide, tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** caller-supplied ids and relation keys; the read
verdict comes from the ACL's `PermitsReadFace`.

**Security-Sensitive Operations:** every refusal must match the missing-row
answer byte for byte. A gate error is returned, never treated as readable.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `TestWriteRequiresRead_Entity` (1, 5),
`TestWriteRequiresRead_Relation` (2, 5), `TestWriteRequiresRead_CopyTarget` (3),
CalDAV tests in internal/dataentry (4).

**Edge Cases:** a hidden target of another type (copy); a face-tailed edge.

**Negative Tests:** each refusal is compared with the missing-id answer.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** one extra read verdict per write, and one header query per relation
write. A dangling edge whose endpoint is gone can no longer be deleted through a
gated handle; the CLI with NopACL or bypass_acl still can.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-security.md,
internal/entitymanager/CLAUDE.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: extends an existing rule)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)
