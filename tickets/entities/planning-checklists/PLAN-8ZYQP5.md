---
id: PLAN-8ZYQP5
type: planning-checklist
title: 'Planning: Document and e2e-test Create menu linking on entity pages'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the Entity pages and sidebar response sections of the data-entry
guide, and one e2e spec with a page-object helper. Out: any change to the
linking code from BUG-PFLS22.

**Acceptance Criteria:**
1. The guide describes Create menu linking, the timeline tab and the rules for several or no relations. Checked by reading the generated `docs/data-entry.md`.
2. The section examples use English names. Checked with grep.
3. An e2e spec covers a list tab, a timeline tab and the no-link case.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Reuses the spaces.spec.ts fixture rewrite and the
inline-create dialog selectors in form.page.ts.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Edit `GUIDE-data-entry.md` and regenerate
`docs/data-entry.md` with `just docs`. Add `createFromMenu` to `SpacesPage`. The
spec rewrites the fixture into one space, adds a feature-to-task `contains`
relation, a gantt over it and a feature entity page with a list tab
(`implements`), a timeline tab and a bugs tab.

**Files to modify:**
- docs-project/entities/guides/GUIDE-data-entry.md, docs/data-entry.md
- e2e/pages/spaces.page.ts
- e2e/tests/space-create-page-link.spec.ts (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None. Docs and tests only.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Tasks tab: Create > Task links the task with `implements`, not `contains`.
- Timeline tab: Create > Task links it with `contains`, not `implements`.
- Bugs tab: two candidate relations, so no link.

**Edge Cases:** Several relations for one type (covered). A type with no
relation is covered by the BUG-PFLS22 unit tests.

**Negative Tests:** The bugs tab case asserts no link is made. The two link
cases fail when the Create menu is reverted to its state before BUG-PFLS22.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Flaky wait on the create response; mitigated by waiting for the non
dry-run POST and the navigation that follows the link. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md (via GUIDE-data-entry).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None; docs and test only.
