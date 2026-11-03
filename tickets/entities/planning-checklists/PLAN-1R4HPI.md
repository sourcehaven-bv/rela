---
id: PLAN-1R4HPI
type: planning-checklist
title: 'Planning: Lua HTML and markdown conversion helpers'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: not affected by this change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

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
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
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
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: not affected by this change)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: not affected by this change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: not affected by this change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: not affected by this change)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: not affected by this change)
- [x] ~~README.md - Project-level changes~~ (N/A: not affected by this change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: not affected by this change)
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

## Notes

- Scope: `rela.md.from_html` and `rela.md.to_html`, backed by the leaf package `internal/htmlmd`. Out of scope: images and attachments (dropped and reported as lossy).
- Approach: bluemonday allowlist shared by both directions; html-to-markdown v2 (commonmark, strikethrough, table) for HTML to markdown; goldmark (no raw HTML, strikethrough, table) for markdown to HTML. `FromHTML` returns a fixed point, so a round trip never changes the text again. Both return a lossy flag computed from the allowlist.
- Alternatives: a hand-written converter (rejected: more code, worse coverage); passing raw HTML through (rejected: unsafe).
- Security: remote HTML is untrusted and sanitized before conversion; markdown output is sanitized after rendering; links only http, https and mailto; 1 MiB input cap.
- Tests: `internal/htmlmd` table tests for both directions, loss flags, entity escaping, input limit and `TestRoundTripStable` (reviewer's breaking inputs included); `TestMdHTMLConversion` for the bindings.
- Review: cranky-code-reviewer found three significant issues (intraword emphasis, entity-like text, silent loss) and four smaller ones; all addressed, see the review responses.
- Docs: Lua scripting guide, "Rich text" under Sync connectors and the rela.md reference.
