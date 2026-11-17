---
id: PLAN-GH5KC2N
type: planning-checklist
title: 'Planning: Create a column from the board'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**How columns are defined today:**
- `column_property` must be an enum: an inline `type: enum` with `values:`, or a named custom type from `types:` (validate.go refuses anything else).
- `columns_from` boards take their columns from relation targets. A new column there is a new target entity, which the board's existing "add group" button already creates on a wall.
- A plain wall has no columns.

So a new column on a `column_property` board is a new enum value: a schema
write. Configure is the only server-side config writer
(`.claude/rules/configedit.md`): its allowlist is per key and default-locked,
and a save builds a new server generation. The feature uses that path from the
SPA. No new server endpoint writes config.

**Scope:**

In scope:
- A new board key `add_column: true` in `data-entry.yaml`. Off by default.
- An "Add column" control at the end of the board, shown when `add_column` is on and `/_me.can_configure` is true.
- Only an inline enum: `entities.<type>.properties.<column_property>` with `type: enum`. The control appends the value (and a label when it differs) and, when the board lists explicit `columns:`, appends the column there too. One Configure save with `base_version`.
- Config-load validation: `add_column` is refused unless the board has a `column_property` whose property is an inline enum.
- Allowlist: `kanbans.*.add_column` editable.

Out of scope:
- Named custom types (`types.<name>.values`). Adding a value there changes every property and every board that shares the type. The Configure space's choice-list editor is the place for that.
- `columns_from` boards and plain walls: the add-group button already covers them.
- Removing or renaming a column.

**Acceptance Criteria:**
1. With `add_column: true` and config edit rights, the board shows "Add column". Typing "In review" and pressing Enter saves `values: [..., in_review]` and `labels.in_review: In review` through `/_configure/save`, then reloads; the new column shows. (Vitest, e2e)
2. With explicit `columns:`, the column is appended to the board's `columns` in the same save. (Vitest)
3. Without `add_column`, or without `can_configure`, no control renders. (Vitest)
4. A name that maps to an existing value, or to an empty machine name, is refused inline before any request. (Vitest)
5. A save with problems (422) or a version conflict (409) shows the message inline and keeps the text. (Vitest)
6. Config load refuses `add_column` on a `columns_from` board, a wall, or a named-type column property. (Go table test)

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small feature on the existing Configure API)

**Existing Solutions:**
- `frontend/src/api/configure.ts` (getConfigure, saveConfigure) and `configure/tree.ts` already edit the schema and data-entry trees in the SPA.
- `configure/useReview.ts` is the save flow: `beginConfigSave`, then `markConfigSaved` and a reload on success. The board follows it.
- `machineName` (configure/names.ts) derives the value from the label, as the Configure editor does.
- Trello and GitHub Projects put "Add column" at the end of the board.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Go:
- `Kanban.AddColumn bool` (`add_column`).
- `validateKanbanAddColumn`: refuses `add_column` without `column_property`, with `columns_from`, or when the property's type is not `enum` (a named type).
- Allowlist: `kanbans.*.add_column` editable. The keys the save writes (`entities.*.properties.*.values.[]`, `entities.*.properties.*.labels.*`, `kanbans.*.columns.[].**`) are already editable.

SPA:
- New pure module `configure/addEnumColumn.ts`: given the snapshot, entity type, property, board id and label, returns the edited trees or a reason it cannot (no name, exists, not an inline enum).
- New `components/kanban/KanbanAddColumn.vue`: a button that turns into an input; Enter saves, Escape cancels. Rendered through the board's `addSection` slot.
- `KanbanView.vue`: `addColumnEnabled = add_column && column_property && !columns_from && me.can_configure`.

**Alternatives considered:**
- A dedicated server endpoint that appends an enum value: rejected. It would be a second config writer, which `configedit.md` forbids.
- Scope to non-enum properties: not possible. `column_property` must be an enum.
- Allow named types: rejected for now, see scope.

**Files to modify:**
- internal/dataentryconfig/config.go, validate.go, new validate_kanban_addcolumn.go (+ test)
- internal/configedit/allowlist.go
- frontend/src/configure/addEnumColumn.ts (+ test), components/kanban/KanbanAddColumn.vue, views/KanbanView.vue, types/config.ts, i18n
- e2e fixtures, kanban page object, new kanban-add-column.spec.ts
- docs-project data-entry guide

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The typed label: free text. The value is `machineName(label)` (lowercase letters, digits, `_`). The label is stored as given; YAML quoting is the server's job.
- The tree edited is the server's own snapshot, sent back with `base_version`.

**Security-Sensitive Operations:**
- A config write. `/_configure/save` enforces `config:edit`, the interactive-principal rule and the per-key allowlist. Hiding the control without `can_configure` is a UI hint only.
- No ACL keys are touched.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1-5: `addEnumColumn.test.ts` and `KanbanView.addColumn.test.ts` (Vitest).
- AC6: Go table test.
- e2e `kanban-add-column.spec.ts` with `RELA_CONFIG_EDITING=1` and an acl granting `config:edit`: add a column and see it after the reload.

**Edge Cases:**
- Label equals its machine name: no `labels` entry.
- Label with accents or symbols: machine name strips them.
- Board without explicit `columns:`: only the schema changes.

**Negative Tests:**
- Duplicate value, empty machine name, 422 problems, 409 conflict.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A save reloads every open tab (the `config-changed` broadcast). That is how every Configure save works; the control is opt-in per board.
- A concurrent config edit: the save carries `base_version`, so a 409 shows inline.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project data-entry guide, kanban section: `add_column`
- [x] docs/ regenerated by `just docs`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self review; no critical or significant findings, so
no review-response entities.
- Decision recorded: enum columns need a schema write, done through the Configure API only (configedit.md), scoped to inline enums.
- Named custom types excluded because the change would reach every property sharing the type.
