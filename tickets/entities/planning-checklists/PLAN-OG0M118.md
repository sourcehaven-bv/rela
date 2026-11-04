---
id: PLAN-OG0M118
type: planning-checklist
title: 'Planning: Read state, actor and reason on declared pile items; unread badge and expiry'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: per-item actor, reason, added and read time on declared piles with read_state; unread counts; mark read, read all, auto read on open; bump; clear_read_after and expire_after pruning; read-first eviction. Out: a general entity-read tracking service.

**Acceptance Criteria:**
1. Summary carries unread and unread_addresses after the visibility gate (TestDeclaredPiles_SummaryAndItems gated case). 2. Mark read, open and read all clear unread; an own pile 400s, a foreign pile 404s (TestDeclaredPiles_MarkRead). 3. Bump, prune and eviction behave per pilestest RunReadStateTests. 4. The panel shows cause text and mark-read controls (PilePanel.test.ts).

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

**Technical Approach:** Read state lives on the pile item (ReadAt), not in a separate read log, so it is per user and per pile. Pruning runs lazily on access. Counts are computed on the gated rows. Alternative rejected: a separate notifications store, because piles already give per-user, gated lists of refs.

**Files to modify:** internal/piles/*, internal/apiwire/v1/piles.go, internal/dataentry/piles_handler.go and piles_read.go, frontend PilePanel.vue, usePiles.ts, sidebarNav.ts.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Item addresses in request bodies, parsed with parseAddresses; unknown addresses are ignored.

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

**Risks:** Counting hidden items would leak existence; mitigated by counting after the gate and a test that hides one. Actor titles resolve through the same gate in one batch. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel, GUIDE-data-entry, GUIDE-lua-scripting, GUIDE-mcp-server, docs/data-entry/api-reference.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed with the user in session before the tickets were written)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded.
