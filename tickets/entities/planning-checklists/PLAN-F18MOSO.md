---
id: PLAN-F18MOSO
type: planning-checklist
title: 'Planning: Declared piles: operator-defined, locked piles in schema.yaml'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a top-level `piles:` block in schema.yaml; each key is a pile every user holds, created on first use, locked (no rename or delete), outside the per-user pile limit and name space. Out: per-role piles, operator-chosen ordering in the menu.

**Acceptance Criteria:**
1. A declared pile appears in GET /_piles with key and locked=true (TestDeclaredPiles_SummaryAndItems). 2. PATCH and DELETE on it give 409 pile_locked (TestDeclaredPiles_Locked). 3. A key is unique per owner and moves with its owner (pilestest RunDeclaredTests). 4. Schema validation rejects a bad key or missing name (metamodel tests).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: design agreed with the user in session; the Atlas design was the reference)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: builds on the in-tree piles subsystem)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the Atlas favorites and notifications menus were the reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Reuses the piles store, service and gated read path (internal/piles, internal/dataentry/piles_read.go).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Parse `piles:` into metamodel.PileDecl, map to piles.Declared at appbuild wiring. Store a Key column on a pile (kvpiles field, pgpiles migration 0041). piles.Service.ByKey creates the pile lazily. The SPA lists locked piles in their own sidebar group. Alternative rejected: a separate notifications store, because piles already give per-user, gated lists of refs.

**Files to modify:** internal/metamodel/piles.go, internal/piles/{service,declared}.go, internal/piles/kvpiles, internal/piles/pgpiles, internal/appbuild/piles.go, internal/dataentry/piles_handler.go, frontend sidebar and PilePanel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** schema.yaml (operator config, validated at load), pile ids and keys from HTTP paths (looked up per owner, so a foreign id is a 404).

**Security-Sensitive Operations:** Pile reads go through the visibility reader; a foreign pile is a 404.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria; each names its test.

**Edge Cases:** Empty pile, hidden items, unknown addresses, a pile not yet created.

**Negative Tests:** Invalid config fails load; locked and foreign piles are refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Migration on postgres: additive nullable column with a partial unique index; mitigated by pgpiles tests. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel, GUIDE-data-entry, GUIDE-lua-scripting, GUIDE-mcp-server, docs/data-entry/api-reference.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed with the user in session before the tickets were written)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded.
