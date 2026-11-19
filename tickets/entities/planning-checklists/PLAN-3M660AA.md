---
id: PLAN-3M660AA
type: planning-checklist
title: 'Planning: Link text and truncation for link properties'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood (display: {link: true} shows the raw URL as the link text, and a long URL widens its row or cell)
- [x] Scope defined (what's in/out documented below) (in: an optional display.link_text, one-line truncation of a URL shown as itself, the hover title, docs. Out: per-value link texts from another property)
- [x] Acceptance criteria documented with specific test scenarios (see the ticket body)

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small schema option)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: a schema key and CSS)
- [x] Checked codebase for similar patterns or reusable code (display.currency (TKT-SVLQQ6) and display.link (TKT-0101IO4) travel the same path: metamodel, wire PropertyDef, store fallback, widget)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art (metamodel-types and data-entry-ui)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented (link_text on metamodel.PropertyDisplay and the wire; config load rejects it without link: true or when blank; UrlWidget shows it in place of the URL; without it the URL is cut off with an ellipsis at 40ch and the title shows the target)
- [x] Approach builds on existing patterns (not reinventing) (same display map and widget fallback as currency)
- [x] Alternatives considered (document why rejected) (a link_text_property naming another property was left out: no customer asked for a per-row text, and it would need a second read per cell; a % max-width was rejected after measuring: auto-layout tables and nowrap rows ignore it)
- [x] Dependencies identified (packages, APIs, types) (none)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs) (operator schema.yaml; the URL value is entity data)
- [x] Input validation approach defined (allowlist preferred over blocklist) (link_text needs link: true and must not be blank; it is rendered as text by Vue, never as HTML)
- [x] Security-sensitive operations identified (file access, auth, crypto) (the href keeps the linkHref scheme allowlist; the link text cannot change the target)
- [x] Error handling doesn't leak sensitive information (config errors name the schema property only)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (loader cases, wire mapping, widget text and title, store fallback with agreeing and disagreeing types, e2e on the entity page)
- [x] Edge cases identified and documented (value that is not a link keeps its own text; property declared by several types; blank text)
- [x] Negative test cases defined (invalid input, error conditions) (link_text without link, with currency, blank)
- [x] Integration test approach defined (not just unit tests) (url-link.spec.ts drives the schema response into the page)

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

- [x] Technical risks assessed with mitigations (a cell without an entity type could borrow another type's text; the store uses it only when all types agree)
- [x] Security risks assessed (see Security Considerations) (none beyond the existing scheme allowlist)
- [x] Effort estimated (xs/s/m/l/xl) (s)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor) (metamodel guide, Display formats)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features (via docs-project GUIDE-metamodel.md)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: the format is documented with the schema)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: there are user-facing docs)
-->

## Design Review

- [x] Run `/design-review` before starting implementation (cranky-code-reviewer over the first cut; findings recorded as review responses)
- [x] All critical/significant findings addressed in plan (wire field, cross-type fallback, truncation in tables and rows)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
