---
id: IMPL-BQ6OHC
type: implementation-checklist
title: 'Implementation: View section fields render as display by default; opt in to inline edit with `render: input`'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

18 new tests: 12 frontend (`sectionEditFields.test.ts`,
`SectionEditForm.test.ts`), 6 Go table-driven (`render_test.go`,
`sections_render_test.go`). Full-stack verification ran against a live server
and the full Playwright suite — see Manual Verification.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Existing `makeFields()` / `makeRow()` factories were extended with `render:
'input'` rather than duplicated, so the pre-existing suites keep exercising the
edit path they were written for while the new default is covered by dedicated
cases.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server --project tickets` and queried the live API (CSRF origin header
required):

1. **Configured + migrated view** (`_views/feature/FEAT-72NR1`) → `status`, `priority`,
`summary` all emit `render: 'input'`. Section-level `render:` inheritance works
on the wire.
2. **Synthesized default view** (`_views/ticket/TKT-HOIX1`, no `views:` entry) → `title`,
`kind`, `priority` all emit `render: 'display'`.
3. **Inert-mode warning** — temporarily added `render: input` to a `display: table` section;
server logged: `view "feature": section[3] sets render: on display mode "table",
which does not render fields; the setting has no effect (it applies to: cards,
list, properties)`. Config restored afterwards.
4. **Config validation** — `rela --project tickets validate` passes on the migrated config.
Note the CLI `validate` command does NOT surface warnings; only the data-entry
server does (`app.go:654`). Worth knowing for anyone verifying warning
behaviour.
5. **E2E — the #997 regression guard passes.** `entity-detail-list-unmount.spec.ts` asserts a
`.section-edit-form` mounts inside a `display: list` row; it passes with the
migrated fixture, proving the section still mounts an inline-edit form and the
unmount crash path is still exercised rather than passing vacuously.

Browser-level *visual* confirmation was not done — the Chrome extension was not
connected in this environment. Rendered-DOM behaviour is covered by the
component tests (display arm renders a bare widget with `mode="display"` and no
`.form-field` chrome) and by the e2e suite, but no human has looked at the page.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: one `ResolveFieldRender` helper owns the field→section→display inheritance
rule, called from both section builders (which previously duplicated their
label-fallback logic and would have duplicated this too). Re-exported into
`dataentry` as `resolveFieldRender` following the package's existing type-alias
convention rather than adding an import.

Security: `writable = render === 'input' && isFieldWritable(verdict)` — a
conjunction, applied at the `widgetRows` site only. Config downgrades
editability and can never upgrade it past the ACL; pinned by a dedicated test.
The `render` flag is kept off `verdict` so the flip-watcher cannot misread a
config change as a revoked permission (RR-PGGRBD).

## Pipeline

| Check | Result |
|---|---|
| `go build ./...` | pass |
| `go vet` (changed pkgs) | pass |
| `go test ./...` | pass — 80 packages, exit 0 |
| `golangci-lint run ./...` | **0 issues** (full repo) |
| `just arch-lint` | OK — no warnings |
| `just coverage-check` | pass — 76.9% total, all floors satisfied |
| `npm run test:run` | pass — 1488 tests / 90 files (was 1470) |
| `npm run typecheck` | pass |
| `npm run lint` | 0 errors (90 pre-existing warnings, untouched files) |
| `npx playwright test` | **pass — 231 passed, 0 failed** (8 skipped: postgres-gated history specs) |
