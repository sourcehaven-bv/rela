---
id: PLAN-GZUYVB4
type: planning-checklist
title: 'Planning: Remove the jump bar from the entity detail page'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: the `nav.jump-bar` block, its `.jump-bar`/`.jump-link` CSS
and the `scrollToSection` helper in
`frontend/src/components/entity/EntityDetail.vue`, plus the one sentence in the
data-entry guide that mentions the jump bar. Not in scope: section ids, section
rendering, other navigation.

**Acceptance Criteria:**
1. No jump bar renders on the entity detail page, for any number of sections.
Test: grep for `jump-bar`/`jump-link` in the frontend and e2e returns nothing;
EntityDetail tests pass.
2. No dead code remains. Test: `scrollToSection` has no definition or use;
vue-tsc and ESLint report no errors.
3. Docs no longer mention the jump bar. Test: grep `docs/` and `docs-project/`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: removal of one UI element)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: removal)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: removal)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A for small changes

**Existing Solutions:** The jump bar came from FEAT-002 (sticky headers). It is
used only in `EntityDetail.vue`; no e2e page object or unit test refers to it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] ~~Alternatives considered (document why rejected)~~ (N/A: straight removal requested by the user)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Delete the template block, the CSS rules and the helper.
Drop the guide sentence in both `docs-project/` (source) and `docs/` (generated
copy).

**Files to modify:**
- `frontend/src/components/entity/EntityDetail.vue`
- `docs-project/entities/guides/GUIDE-data-entry.md`
- `docs/data-entry.md`

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no new input)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: UI removal)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no error paths)

**Input Sources & Validation:** None.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: removal, no inputs)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. Run vue-tsc, ESLint and the
EntityDetail Vitest suite; e2e specs do not reference the jump bar.

**Edge Cases:** A view with one section never showed the bar; a view with
several sections now shows the sections directly.

**Negative Tests:** N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Users lose quick section navigation on long pages. Accepted: the user
asked for removal. Effort: xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore ticket; the single doc sentence is updated in this change)

**Documentation Impact:**
- [x] docs/data-entry.md - drop the jump-bar sentence

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: deletion of one UI element, no design surface)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
