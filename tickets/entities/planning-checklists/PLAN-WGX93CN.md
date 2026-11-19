---
id: PLAN-WGX93CN
type: planning-checklist
title: 'Planning: Name the page and the app in the browser tab title'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood (index.html titles every tab rela and nothing but the inbox count changes it)
- [x] Scope defined (what's in/out documented below) (in: the tab title on every SPA screen and its docs. Out: the visible headings, the flow-run views' own headings (another agent))
- [x] Acceptance criteria documented with specific test scenarios (see the ticket body)

**Scope:**
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small frontend change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: a few lines; @vueuse useTitle is not a dependency and does not solve composing the parts)
- [x] Checked codebase for similar patterns or reusable code (usePageHeader publishes the header title through module-level refs; useTabTitleBadge wrote document.title)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art (data-entry-ui; FEAT-011 branding)

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented (useDocumentTitle in App.vue is the one writer; usePageTitle publishes a page title per screen (PageTitle and the views with their own heading); the page header title wins; the inbox badge publishes its count)
- [x] Approach builds on existing patterns (not reinventing) (same module-level outlet as usePageHeader)
- [x] Alternatives considered (document why rejected) (route meta titles were rejected: most titles come from config and entities, which only the view knows; setting document.title in each view was rejected: the badge and the app name would be repeated everywhere)
- [x] Dependencies identified (packages, APIs, types) (none)

**Technical Approach:**
<!-- Document the approach with enough detail that implementation is mechanical -->

**Files to modify:**
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs) (app.name and operator titles from data-entry.yaml; entity titles from the gated read)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: document.title is a text sink)
- [x] Security-sensitive operations identified (file access, auth, crypto) (an entity title shows in the tab; it is the title the page already shows to the same principal)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no errors)

**Input Sources & Validation:**
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:**
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (composeTitle parts; newest page title wins and falls back on unmount; header title wins; PageTitle publishes; badge in front)
- [x] Edge cases identified and documented (empty page title, page equal to the app name, app name still loading, embedded forms and entity panels must not rename the tab)
- [x] Negative test cases defined (invalid input, error conditions) (an empty page title is skipped)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: unit tests over the composable and PageTitle cover the composition)

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

- [x] Technical risks assessed with mitigations (low: title only)
- [x] Security risks assessed (see Security Considerations) (none: text the principal already sees)
- [x] Effort estimated (xs/s/m/l/xl) (s)

**Risks:**
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor) (docs/data-entry.md App section)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing)
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small frontend change; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
