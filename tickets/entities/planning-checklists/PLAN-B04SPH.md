---
id: PLAN-B04SPH
type: planning-checklist
title: 'Planning: gate MCP counts through the read ACL'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The remote MCP reports per-type entity and relation counts from the
raw store (`gatedGraphReader.CountEntities` / `CountRelations`). A caller learns
how many rows exist of a type it may not read, and how many rows of a readable
type are hidden. `docs/acl-security.md` already says no aggregate may come from
an unfiltered source.

**Scope — IS in scope:** gated `CountEntities` / `CountRelations` on the script
readers; `gatedGraphReader` forwards to them; docs note.

**Scope — NOT in scope:** a SQL pushdown for relation gating (no store predicate
exists for "both endpoints readable"); local stdio MCP (no ACL by design);
counts in the CLI (operator shell).

**Acceptance Criteria:**
1. For a principal, the entity count equals the length of the gated list, for
global, face-restricted global and relation-conferred read.
2. A principal without read on a type gets 0.
3. A relation count excludes edges with a hidden endpoint.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal seam)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `listPushdown` (internal/visibility/pushdown.go) already
composes the read scope as a store query; `store.CountMatched` counts a
GraphQuery; `dataentry.scopedTotal` uses it for list totals.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** extract `composeReadScope` from `listPushdown`; add
`countPushdown` over the same scope (DenyAll 0, AllowAll plain count with
FaceIn, Query via CountMatched). `ScriptReader.CountEntities` uses it and falls
back to counting the gated `ListEntities`. `CountRelations` counts
`ListRelationsStrict`.

Rejected: counting the header stream in the fallback (ranks faces before it
gates, so it can disagree with the list under a world); hiding counts for gated
types (less useful to an agent, and still a raw count for others).

**Files to modify:**
internal/visibility/{pushdown,luareader,unrestricted,denyreader}.go,
internal/appbuild/appbuild.go, docs/acl-security.md, tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the MCP caller's principal and the requested
type; the scope comes from acl.yaml through readQuery.

**Security-Sensitive Operations:** the count itself. A scope error fails closed
(error, never a raw count).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `TestCountPushdown_EqualsListPushdown` (criteria 1, 2),
`TestScriptReader_CountsEqualGatedLists` (fallback, criterion 3),
`TestGatedCounts_UseTheGatedReader` (wiring).

**Edge Cases:** faced entities with a hidden face; type-less query (fallback).

**Negative Tests:** DenyAll does not touch the store; a scope error is returned;
DenyReader refuses.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** `CountRelations` iterates the gated relation list, so the schema
overview costs one relation scan per relation type. Acceptable for the remote
MCP overview; a relation pushdown is a separate change.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-security.md, remote MCP residuals.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change on an existing pattern)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
