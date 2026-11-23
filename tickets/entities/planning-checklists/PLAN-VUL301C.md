---
id: PLAN-VUL301C
type: planning-checklist
title: 'Planning: Record types: MCP tools, Lua list and aggregate, and CLI'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: MCP tools (stdio and remote), Lua list and aggregate, CLI `rela records`. Out: rollups, native SQL aggregator, per-type permissions, panel, audit.
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. Every surface goes through records.Service (tests per surface).
2. A hidden owner's rows are not listed and not folded (appbuild test with a real acl.yaml).
3. `expire` is CLI-only (documented).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the REST surface fixed the design)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal wiring)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Consumer-side interfaces in mcp, lua and cli, wired at the composition roots. records.UserText shares the caller-facing error text.
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
internal/mcp, internal/lua, internal/cli, internal/appbuild, internal/dataentrywire, internal/records, docs-project guides.
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Tool, Lua and flag arguments; the service validates owners and fields. Misspelled Lua options raise.
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
Owner gate stays in the service. Backend errors are logged, not shown. Expire is not exposed to agents.
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Per-surface unit tests with the real service, plus an ACL integration test.
<!-- Map each acceptance criterion to how it will be tested -->

**Edge Cases:**
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:**
<!-- What should fail? How should it fail? -->

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Low: adapters only. Effort m.
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs are updated in this ticket)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: thin adapters over records.Service; /code-review runs in review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
