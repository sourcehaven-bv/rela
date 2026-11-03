---
id: PLAN-K6E0P8Z
type: planning-checklist
title: 'Planning: Pile toggle action (favorite star) and pile navigation entries'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `piles.toggles` in data-entry.yaml, a button per toggle on the entity page with operator icons, pressed while the entity is on the pile; a `pile:` nav entry can name a declared key. Out: toggles on list rows and kanban cards; a `placement` option.

**Acceptance Criteria:**
1. Config rejects a toggle with no pile, an undeclared pile, a duplicate, or an unknown entity type (dataentryconfig tests). 2. The button shows pressed from the summary's addresses and toggles on click (PileToggles.test.ts). 3. entity_types filters the buttons (PileToggles.test.ts).

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

**Technical Approach:** Toggles are served in the piles config bootstrap; the SPA reads `addresses` off the list summary, so a toggle costs no request. Add and remove reuse the existing item endpoints. Alternative rejected: a separate notifications store, because piles already give per-user, gated lists of refs.

**Files to modify:** internal/dataentryconfig/piles.go, internal/dataentry/piles_wiring.go, frontend/src/components/piles/PileToggles.vue, EntityDetail.vue, usePiles.ts, sidebarNav.ts.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml (validated at load); addresses come from the gated summary.

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

**Risks:** Addresses on the summary grow with pile size; bounded by the pile item cap. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel, GUIDE-data-entry, GUIDE-lua-scripting, GUIDE-mcp-server, docs/data-entry/api-reference.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed with the user in session before the tickets were written)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded.
