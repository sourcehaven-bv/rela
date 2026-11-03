---
id: PLAN-BJ0VKP3
type: planning-checklist
title: 'Planning: Convert dialog uses form widgets and the Milkdown editor'
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the convert dialog renders target fields with the create form's
widgets, lists required unsupplied fields first, and edits the body with
Milkdown. Out: mentions of the old id (TKT-J6GISC3).

**Acceptance Criteria:**
1. Fields render through the shared field components with the target form's widgets and labels. Test: `ConversionModal.test.ts`.
2. Required fields the source cannot supply are listed first; other unmapped fields sit under "More fields". Test: `ConversionModal.test.ts`.
3. The preview's `required` errors show next to each field. Test: `ConversionModal.test.ts`.
4. The body uses `MarkdownEditor`. Test: `ConversionModal.test.ts`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: reuses existing form components)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reuses existing form components)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: reuses existing form components)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `FieldRenderer`/`FieldShell` and `MarkdownEditor` as
used by the create form.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A `conversionFields.ts` helper splits the target type's
form fields into required-unsupplied and optional. `ConversionModal.vue` renders
them with the shared field components and the body with `MarkdownEditor`.

Deviation from the ticket: no read-only preview of mapped content, because the
conversion preview does not send the mapped content to the browser.

**Files to modify:** `frontend/src/components/entity/ConversionModal.vue`,
`conversionFields.ts`, tests, i18n locales.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** User form input; validated server-side by the
existing conversion endpoint.

**Security-Sensitive Operations:** None new; the dialog posts to the existing
ACL-gated conversion endpoint.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** target type without a form config; no unsupplied required
fields.

**Negative Tests:** a server `required` error is shown on its field.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** widget behaviour differs from the create form; mitigated by reusing
the same components.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via GUIDE-data-entry): convert dialog fields

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled with the user in conversation before implementation; `/code-review` covers the result)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
