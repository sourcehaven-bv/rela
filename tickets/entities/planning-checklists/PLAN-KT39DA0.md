---
id: PLAN-KT39DA0
type: planning-checklist
title: 'Planning: Coloured cells on a kanban matrix'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a `cell_colors:` key on a kanban with `column_property` and
`swimlane_property`; load-time validation; server-side resolution of each colour
to a style class; a tinted cell with a visible tone label in the SPA; a `tone`
option on the library `Section` type that `RlSwimlaneColumn` paints; Dutch and
English labels for the five tones; guide docs.

Out of scope: `columns_from` boards (their columns are entity ids, unknown at
load); rule expressions over the two values; colouring plain boards without
lanes.

`cell_colors` is editable in Configure, like the column colours under
`kanbans.*.columns.[].**` (RR-JB0VIHD); see Security Considerations.

**Acceptance Criteria:**

1. A board with `cell_colors: {high: {likely: error}}` paints that cell with
the error tone and shows the label "Critical" (en) or "Kritiek" (nl) in the
cell. Test: e2e spec and a KanbanView unit test.
2. An unknown column key, lane key or colour fails load with
`kanban "<id>": cell_colors...`. Test: Go table test.
3. `cell_colors` on a board without `swimlane_property`, or with
`columns_from`, fails load. Test: Go table test.
4. A colour that is not a tone (such as `red/700` or an operator colour) needs
a `label`. Test: Go table test.
5. The served kanban config carries the resolved style class per cell. Test:
Go test on `resolveKanbanDirections` output.
6. Cards keep their own opaque background, so their text contrast does not
change. Test: library unit test that the card is unchanged; visual check in
light and dark.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small feature on an existing vocabulary)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the dashboard matrix card is the in-tree reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/dataentryconfig/colors.go` (TKT-ZKNWSK): `ParseColor`,
`ToneColor`, `SemanticTones`, `Color.StyleClass`.
- Dashboard matrix `colors:` validation in `dashboard_cards_validate.go` and
resolution in `matrixColorFor` (`internal/dataentry/dashboard_fold.go`).
- `frontend/src/utils/styleColors.ts` `styleFill` maps a style class to
`--rl-tag-*` or `--rl-kit-*` / `--rl-custom-*` tokens.
- No library needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config shape, column value then lane value:

```yaml
cell_colors:
  high:
    likely: error
    possible: { color: warning, label: Medium }
```

A cell value is a scalar colour or a mapping with `color` and `label`
(`KanbanCellColor.UnmarshalYAML`). Validation in `validateKanbanCellColors`:
needs `swimlane_property`, refuses `columns_from`; column keys must be the
board's shown columns (declared `columns:` or else the enum values); lane keys
likewise; the colour is a tone or `ParseColor` with the operator colours; a
non-tone colour needs a label. A shared `resolveToneOrColor` helper in colors.go
is used by both the dashboard matrix and the kanban, so both accept the same
names.

Serving: `resolveKanbanDirections` also fills `CellStyles`
(`json:"cell_styles"`, `yaml:"-"`): column, lane to `{class, label, tone}`. The
SPA resolves the class to two tokens with a new `styleTokens` helper and passes
`tone: {background, foreground, label}` on the lane's section. The library
paints the cell background with `var(background)` and shows the label as text in
`var(foreground)`. Both are `--rl-*` token names, so no literal colour reaches
CSS. Cards keep `--rl-color-bg`.

Alternatives rejected: a rule language over the two values (needs ordered
comparison of enum values and a new evaluator; a 5x5 map is short enough);
colouring client-side from the raw names (would duplicate `ParseColor` in TS).

**Files to modify:**

- internal/dataentryconfig/config.go, colors.go, validate.go,
dashboard_cards_validate.go, new kanban_cells.go + test
- internal/dataentry/api_v1.go, dashboard_fold.go + test
- frontend/packages/rela-components/src/types/index.ts,
components/board/RlSwimlaneColumn.vue + test
- frontend/src/views/KanbanView.vue, utils/styleColors.ts, types/config.ts,
i18n locales, tests
- e2e/tests/fixtures.ts, new e2e/tests/kanban-cell-colors.spec.ts
- docs-project/entities/guides/GUIDE-data-entry.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Two input sources write the same key: operator config files, and a user with
`config:edit` saving through the Configure API, where `kanbans.*.cell_colors.**`
is in the `editable` list (RR-JB0VIHD). Both pass the same load-time
validation: Configure runs it on the proposed config before it writes
(`Validator.Validate` in `internal/configedit/service.go`), so an unknown key or
colour is refused and never served.
Making the key editable is acceptable for these reasons: it adds no input path
that column colours do not already have; it holds no code, guard or permission,
so it is not `protective`; the save needs `config:edit` and is audited; and the
worst a bad value can do is mislabel a cell. Colours go through `ParseColor`,
an allowlist; the style class grammar is re-checked by `FILLED_CLASS` in the SPA
before it becomes a CSS custom property name. Labels are rendered as text by Vue
interpolation, never as HTML.

**Security-Sensitive Operations:**

None. The config is not secret (CLAUDE.md), so serving `cell_styles` to every
reader of the board is fine. No entity data is involved.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- Go table test for every validation error and the valid shapes (scalar,
mapping, tone, kit shade with label, operator colour with label).
- Go test that the YAML scalar and mapping forms both decode.
- Go test that the served config has `cell_styles` with the class and label.
- Vitest: `styleTokens` per class form; RlSwimlaneColumn paints a toned cell
and shows the label; KanbanView passes the tone to the right cell.
- e2e: a fixture board with cell colours; the spec checks the label text and
that the cell background is not transparent.

**Edge Cases:**

- A lane or column that has no cards still shows its tone.
- A collapsed column cell shows no label (it draws nothing).
- A label given for a tone overrides the default tone name.
- Empty `cell_colors: {}` is accepted and paints nothing.

**Negative Tests:**

- Unknown column, unknown lane, unknown colour, missing label for a non-tone,
`cell_colors` without lanes, with `columns_from`, empty colour string.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A saturated kit shade behind cards could look loud. Mitigation: cards stay
opaque; the docs recommend tones, which use the soft tag tints.
- The tinted cell needs padding so cards do not touch the tint edge; the
width is fixed so the column alignment does not change.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- GUIDE-data-entry.md kanban section: `cell_colors`, then `just docs`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self review. (1) Tone default labels must be
neutral, not risk words, because a board is not always a risk matrix: use Good,
Warning, Critical, Info, New. (2) The colour parse must be shared with the
dashboard so the two cannot drift: added `resolveToneOrColor`. (3) Keys must be
checked against the shown columns, not only the enum, or a key for a hidden
column is silent dead config. All folded into the plan.
