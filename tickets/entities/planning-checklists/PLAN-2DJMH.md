---
id: PLAN-2DJMH
type: planning-checklist
title: 'Planning: Add Edit button to data-entry document view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

- IN scope:
  - Extend the document config schema with a new optional sub-block describing
how the standalone document view (`/document/:name/:entityId`) should expose an
Edit button: which form to navigate to, and the button label. Author opts in
explicitly per document.
  - Render the Edit button in `frontend/src/views/DocumentView.vue` only when
the new config block is present.
  - Validate that the referenced form exists at config-load time (matches the
pattern used by `list.edit_form` / `kanban.edit_form`).
- OUT of scope:
  - Server-side renderer, `edit://` rewriter, "Create" button.
  - Keyboard shortcut for the button (deferred — see RR-4P8I0).
  - Auto-resolution of the form from `entity_type`. The reviewer's RR-SFG9K
showed that the auto-resolve fallback in `getEditFormId` can land users on
create forms in silent edit mode. Explicit-per-document config sidesteps that
entirely. (No backwards-compat is needed: this feature has not been released
yet.)
  - Schema-load flicker mitigation (RR-WD6MB) — accepted as-is, matches
existing `EntityDetail.vue` behaviour.
  - entityId/entity_type prefix mismatch guard (RR-VFPYX) — accepted as-is,
surfaces via `DynamicForm`'s existing load-error path.

**Acceptance Criteria:**

1. A document config with a new `edit:` sub-block (`form: <form-id>`,
`label: <string>`) renders an Edit button in the page header on
`/document/:name/:entityId`.
2. Clicking the button navigates to
`/form/<edit.form>/<entityId>?return_to=<docPath>` (URL-encoded), and submitting
the form returns the user to the document URL.
3. A document config without the `edit:` sub-block renders no Edit button.
Existing Back and Refresh continue to work unchanged.
4. Config validation rejects a document config whose `edit.form` references a
form ID not present in `cfg.Forms` (same error class as `list.edit_form` /
`kanban.edit_form`).
5. Config validation rejects a document config with `edit.form` set but
`edit.label` empty (and vice versa) — both fields are required when the block is
present.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `KanbanConfig` and `ListConfig` already carry explicit `edit_form: <id>`
fields validated against `cfg.Forms`:
  - `internal/dataentryconfig/validate.go:758` (kanban edit_form)
  - `internal/dataentryconfig/validate.go:1050` (list edit_form)
  - `internal/dataentryconfig/config.go:160-180` (List shape)
We mirror that pattern, extended to also carry a button label.
- `DocumentConfig` (Go: `internal/dataentryconfig/config.go:442-463`,
TS: `frontend/src/types/config.ts:274-279`) currently has `title`,
`entity_type`, `command`, `script`, `timeout`. We add `edit`.
- `validateDocuments` (`internal/dataentryconfig/validate.go:976`) is the
natural place to add the form-existence and label-presence checks.
- Existing button styling in `DocumentView.vue` (`.btn .btn-secondary`) is
reused.
- `frontend/src/utils/returnPath.ts` provides `buildReturnTo` (already imported
by DocumentView for in-content link rewriting); reusable for the button's
outgoing `return_to` query.
- `frontend/src/components/forms/DynamicForm.vue` already consumes
`return_to` on submit via `readReturnTo`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach (backend, Go):**

1. Add a `DocumentEdit{Form, Label}` struct to
`internal/dataentryconfig/config.go`.
2. Extend `DocumentConfig` with `Edit *DocumentEdit` (pointer for
absent-vs-present semantics on the wire).
3. Extend `validateDocuments`:
   - `Edit != nil && Edit.Form == ""` → error.
   - `Edit != nil && Edit.Form != "" && cfg.Forms[Edit.Form]` missing → error
(with `did you mean` suggestion via `suggestForm`).
   - `Edit != nil && Edit.Label == ""` → error.

**Technical Approach (frontend, TypeScript / Vue):**

1. Mirror the new shape on the SPA config types
(`frontend/src/types/config.ts`): `DocumentEdit{form, label}`,
`DocumentConfig.edit?: DocumentEdit`.
2. In `frontend/src/views/DocumentView.vue`:
   - `editConfig = computed(() => docConfig.value?.edit)`.
   - `editEntity()` pushes
`/form/<editConfig.form>/<entityId>?return_to=<buildReturnTo(...)>`.
   - Button rendered with `v-if="editConfig"` showing `editConfig.label`.

Note: explicitly **diverging from `EntityDetail.vue`**, which does not attach
`return_to`. EntityDetail relies on `router.back()` because users reach it via
SPA navigation; DocumentView is a deep-linkable URL, so `router.back()` from a
fresh tab leaves the SPA. RR-6NIDO documents this.

**Alternatives considered:**

- *Two top-level fields, `edit_form:` and `edit_label:`* — rejected for the
sub-block, which keeps related fields together and allows growth without
flattening.
- *Auto-resolve via `getEditFormId(schemaStore, entity_type)`* (the original
plan) — rejected. RR-SFG9K showed it can silently send users into a
create-form-as-edit-mode trap. Explicit config eliminates the magic.
- *Keep auto-resolve as a fallback when `edit:` is absent* — rejected per
user direction (no backwards compat needed; pre-release feature).
- *Wire the `e` keyboard shortcut to match `EntityDetail.vue`* — deferred
(RR-4P8I0).

**Files to modify:**

- `internal/dataentryconfig/config.go`
- `internal/dataentryconfig/validate.go`
- `internal/dataentryconfig/validate_test.go`
- `frontend/src/types/config.ts`
- `frontend/src/views/DocumentView.vue`
- `e2e/pages/document.page.ts` (new) + `e2e/pages/index.ts`
- `e2e/tests/document-edit-button.spec.ts` (new)
- `e2e/tests/fixtures.ts` (extended `DATA_ENTRY_YAML`)
- `docs/data-entry.md` (corrected from `GUIDE-data-entry.md` — RR-HPOYQ)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `edit.form`: comes from `data-entry.yaml`. Validated at config-load time
to be a known form ID (allowlist via `cfg.Forms[id]`), with typo suggestion.
- `edit.label`: free string, rendered with Vue text interpolation (auto-escapes).
Required to be non-empty so misconfigured docs fail loudly at load time.
- `route.fullPath` → `buildReturnTo`: existing same-origin guard
(`isSafeReturnPath`).
- `props.entityId` (URL param): same flow as `EntityDetail.editEntity`.

**Security-Sensitive Operations:**

None. Same-origin `router.push`. No file access, no shell, no template
interpolation into anything dangerous.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: e2e — inject a doc config with `edit:`, navigate, assert button
visible with the configured label.
2. AC2: e2e — click, assert URL with `searchParams.get('return_to')`
(decoded value, not coupled to encoding — RR-BS0O4).
3. AC3: e2e — submit, assert URL is back at the document.
4. AC4: Go unit test — unknown form → expected error (with typo suggestion
variant).
5. AC5: Go unit tests — empty label, empty form, both empty.

**Edge Cases:**

- `docConfig` undefined during initial schema load → button hidden. Cold
deep-link flicker accepted (RR-WD6MB).
- `edit.form` known but its `entity` differs from the doc's `entity_type`:
not validated; `DynamicForm` surfaces a load error if entityId prefix doesn't
match (RR-VFPYX).
- Bare `edit:` YAML line (no subkeys) deserialises to `nil`, so it's treated
as "field absent." Documented in struct comment + docs (RR-4AXJR).
- Label contains markdown / HTML: Vue text interpolation escapes; renders
literally.

**Negative Tests:**

- `edit.form: ""` → config-load error.
- `edit.label: ""` → config-load error.
- `edit.form: not-a-form` → config-load error (with typo suggestion).
- Both empty → both errors fire (`TestValidateConfig_DocumentsEditBothEmpty`).

**Integration test approach:**

E2E only is consistent with this codebase. The repo has effectively zero
view-level Vitest tests (only `SettingsView.palette.test.ts`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Schema not yet loaded on cold deep-link* — small layout shift in
`.header-right` when `editConfig` resolves. Same as `EntityDetail`. Accepted
(RR-WD6MB).
- *Author misconfigures `edit.form` to point at a form whose entity differs
from the document's `entity_type`* — surfaces via `DynamicForm`'s load error.
Accepted (RR-VFPYX).

Effort: **s**.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/data-entry.md` Documents section
(corrected from `GUIDE-data-entry.md` — RR-HPOYQ).
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns)
- [x] ~~README.md~~ (N/A: project-level scope unchanged)
- [x] ~~API docs~~ (N/A: no API change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-6NIDO (significant) — addressed: `return_to` divergence rationale
documented.
- RR-SFG9K (significant) — addressed: switched to explicit `edit:` config.
- RR-HPOYQ (minor) — addressed: docs path corrected.
- RR-6J2LQ (minor) — obviated by the redesign.
- RR-WD6MB (minor) — accepted as wont-fix, behaviour documented.
- RR-VFPYX (minor) — accepted as wont-fix, surfaces via DynamicForm.
- RR-BS0O4 (nit) — addressed: e2e uses `searchParams.get`.
- RR-YNH4W (nit) — addressed: e2e-only rationale stated.
- RR-4P8I0 (nit) — deferred: keyboard shortcut out of scope.
- RR-22GS6 (nit) — obviated: label is author-controlled.
