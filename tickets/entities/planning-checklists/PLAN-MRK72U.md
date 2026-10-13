---
id: PLAN-MRK72U
type: planning-checklist
title: 'Planning: Template picker as dropdown with readable variant labels'
started: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: the create-form template picker in `DynamicForm.vue` and its labels.
Out: an authorable label key for template variants (DEC-6C1NAA follow-up);
backend template loading.

**Acceptance Criteria:**
1. With more than one template, the create form shows a "Template" dropdown
   listing every variant; with one template it shows nothing.
2. The requested variant (`?template=`, `embeddedTemplate`, section
   `create.types.<type>.template`) is preselected.
3. Choosing a variant resets the form and applies that template.
4. `corrective_action` and `follow-up` read "Corrective action" and
   "Follow up"; the base template reads "Default".

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small UI change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: design system already has a select)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
`rela-components` `RlSelect` (native select with label, used across the
configure screens) fits a text-only list. `RlOptionSelect` is for options
drawn through a slot, which template names do not need.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Replace the pill row with `RlSelect` bound to `selectedTemplate` and emitting
into `selectTemplate()`. Always use the dropdown when there is more than one
template; keeping pills for a few templates would mean two controls for one
choice. Map `-`/`_` runs to spaces in `getTemplateLabel`.

**Files to modify:**
`frontend/src/components/forms/DynamicForm.vue`, its unit tests, the e2e
form page object and templates spec, `GUIDE-data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: none)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Template names come from operator-authored file names via
`GET /api/v1/_templates/{type}`; they are rendered as text by Vue, never as HTML.

**Security-Sensitive Operations:**
None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Unit tests in `DynamicForm.embedded.test.ts` cover criteria 1-4; the prefill
test switches template through the dropdown; the e2e templates spec checks
option labels and selection against a real server.

**Edge Cases:**
A single template (no dropdown); the default template named `''`.

**Negative Tests:**
A requested variant that does not exist falls back to the default (existing
behaviour, unchanged).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
The label derivation is in scope for DEC-6C1NAA; `DynamicForm.vue` is already
allowlisted in `label_derivation_lint_test.go` for exactly this function.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small UI change reusing an existing component)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** None
