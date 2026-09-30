---
id: PLAN-3Z19BU
type: planning-checklist
title: 'Planning: Remove the sync feature'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `internal/sync`, `internal/cli/sync`, `rela sync`, the `/api/sync/`
routes and handlers, `pgstore.ManifestSince`, `entitymanager.ApplyEntity`
and `ApplyRelation`, `principal.ToolSync`, their tests, guard allowlist
entries, arch-lint rules, lint config entries and docs.

Out: anything another feature shares. The `deletions` table and seq indexes
(change-feed catch-up), the non-browser CSRF exemption on `/api/v1`
(documented curl use), `internal/canonical` (versioning, relation ETag), the
single-relation GET (public v1 API), and `RecreateEntity` (history restore).

**Acceptance Criteria:**

1. No sync package, command, route or doc remains: `grep` finds none, and
   `/api/sync/manifest` is no longer routed.
2. History restore still works: `RecreateEntity` tests pass.
3. All test suites and gates pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: removal)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: removal)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: removal)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Ruling D9 in the Stage 3 design (section 21) decides
the removal. Every sync symbol was traced to its callers to separate
sync-only code from shared code.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Delete sync-only code. Rewrite `RecreateEntity` as a
create-only function, since its update fall-through existed for sync.
Rename the CSRF helper to a generic name. Regenerate `docs/` with
`just docs`.

Alternative rejected: a forward migration dropping `deletions`. The table
feeds the change-feed catch-up, so it is not sync-only.

**Files to modify:** see the PR diff.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Removing routes shrinks the input surface.
No new input.

**Security-Sensitive Operations:** CSRF exemption list loses `/api/sync/`.
`RecreateEntity` keeps its create ACL check and audit record.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** router walk test (route gone), CSRF exemption test,
`RecreateEntity` tests, full suites on all four backends.

**Edge Cases:** recreate when the face already exists; recreate on a face the
principal may not write.

**Negative Tests:** recreate of an existing face returns
`ErrEntityAlreadyExists`; a denied face returns a forbidden error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Removing shared code by mistake. Mitigated by tracing callers and
running all backend suites.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore; docs edits are part of the removal)

**Documentation Impact:** GUIDE-sync removed; cli-reference, metamodel,
sqlite-backend, acl-security, ISMS tutorial and README updated.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: removal decided by ruling D9)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
