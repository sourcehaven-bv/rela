---
id: PLAN-0ROWSKU
type: planning-checklist
title: 'Planning: View section permission and hiding sections with only redacted fields'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: optional `permission:` on a view section; the server drops an
entry `display: properties` section whose every field is redacted for the
reader. Out: side panels (refuse `permission:`), collection-row sections, a
client-side filter (the view payload is per request, so the server decides).

**Acceptance Criteria:**
1. A section with `permission: X` is absent from `_views` for a principal
without X and present for one with X, and always present without acl.yaml.
2. An entry properties section whose every field is in the entry's redacted
set is absent; one readable field, or a relation field, keeps it.
3. A `create:` on a later section stays on the right section.
4. `permission:` on a side-panel section is refused at config load.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change on existing patterns)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config filter, no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `permitsGatedUIElement` (views_handler.go) for nav
entries, spaces and dashboard cards; `omitHiddenEmpty` (sections.go, TKT-JT101B)
for dropping sections after the create resolver; `visibility.Redact` records
`Redacted`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `buildSections` marks a section `withheld` when its
permission is not held (skipping the build) or when `allFieldsRedacted(fields,
entry)` holds. `omitHiddenSections` (renamed from `omitHiddenEmpty`) drops
withheld sections after `resolveSectionCreate`, which skips them. Rejected: a
frontend filter, which duplicates a decision the per-request payload already
makes.

**Files to modify:** internal/dataentryconfig/config.go, validate.go;
internal/dataentry/sections.go, sectioncreate.go, views_handler.go; tests; e2e
spec; data-entry guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `permission:` comes from operator config; an
unknown name is simply not held.

**Security-Sensitive Operations:** None new. The permission check uses the bound
request's read gate (`readGateFromContext`), no new identity resolution. It is a
UX filter only; values stay protected by `visible:` and the read gate. No
runtime deny is added to the ACL.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** 1-3: `TestWithheldSections_PerReader` (reader vs staff) and
`TestWithheldSections_NoPolicyShowsAll`. 2: `TestAllFieldsRedacted`. 4:
`TestSectionPermission_Config`. Playwright: `view-section-permission.spec.ts`.

**Edge Cases:** No fields; nothing redacted; mixed readable and redacted;
relation field.

**Negative Tests:** Side-panel `permission:` refused at load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Index alignment with `resolveSectionCreate`: withheld sections stay
in the slice until after it, pinned by a test. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry (view section keys, new "Sections the
reader cannot use").

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change reusing the nav/card permission filter and the hide_empty drop; reviewed in code review instead)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
