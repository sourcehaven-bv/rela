---
id: PLAN-621CLHR
type: planning-checklist
title: 'Planning: notify automation action: push to related users, skipping the actor'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `notify` automation action with `pile`, `reason` and `to: {via, type, property}`; recipients resolved along relations; the acting user is skipped and recorded as actor. Out: email delivery; Lua-only recipient logic.

**Acceptance Criteria:**
1. The project owner gets a notification when someone else completes a task, not when they complete it themselves (TestNotify_ProjectOwnerLearnsOfCompletedTask). 2. Schema validation rejects an undeclared pile, a pile without read_state, an unknown reason, a missing to.property, to.via without on.entity, and to.type without to.via (metamodel tests).

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

**Technical Approach:** The automation engine emits a notify side effect; autocascade resolves recipients through a RecipientFinder wired in appbuild (raw store, system read, since recipients are config-driven) and pushes with SkipSelf. Alternative rejected: a separate notifications store, because piles already give per-user, gated lists of refs.

**Files to modify:** internal/metamodel/addtopile.go and types.go, internal/automation, internal/autocascade/piles.go, internal/appbuild/piles_bindings.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Automation config (validated at load); recipient property values from entities (looked up as person ids or logins; unknown values are dropped).

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

**Risks:** A push to another user is a write across principals; it is logged and failures to a missing owner are dropped, not raised. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-metamodel, GUIDE-data-entry, GUIDE-lua-scripting, GUIDE-mcp-server, docs/data-entry/api-reference.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed with the user in session before the tickets were written)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded.
