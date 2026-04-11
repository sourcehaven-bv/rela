---
id: PLAN-4W5Z3
type: planning-checklist
title: 'Planning: Investigate view system supersession by Lua scripting'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
IN: Remove internal/views/ package, CLI view commands, MCP tools/resources, schema integration, docs. Add Lua example scripts.
OUT: Data-entry ViewConfig (separate system, unchanged).

**Acceptance Criteria:**
1. All `internal/views/` code removed
2. All CLI `view` commands removed
3. All MCP view tools/resources removed
4. Schema analysis no longer references views.yaml
5. Example Lua scripts demonstrate deps/affected patterns
6. All tests pass, lint clean

## Research

- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: removal task)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: removal task)
- [x] Reviewed relevant rela concepts for prior art

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Remove views.yaml system entirely. Lua scripting already covers all capabilities.

**Files to modify:**
12 files removed (internal/views/), 3 CLI commands removed, MCP handlers removed, ~20 files edited to remove references.

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: removal reduces attack surface)
- [x] ~~Input validation approach defined~~ (N/A: removal)
- [x] ~~Security-sensitive operations identified~~ (N/A: removal)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: removal)

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined~~ (N/A: removal)
- [x] ~~Integration test approach defined~~ (N/A: removal, existing tests verify)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low — removal only, no new functionality.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs updated inline)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: straightforward removal)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review needed)
