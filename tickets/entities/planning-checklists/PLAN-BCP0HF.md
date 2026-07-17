---
id: PLAN-BCP0HF
type: planning-checklist
title: 'Planning: Enum values support a display label/title for better UX on snake_case values'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: optional per-value display labels for enum properties (named custom types +
inline `type: enum`); labels carried over the v1 `_schema` API; rendered
wherever enum values display in the data-entry web UI — centrally via `Badge`
(lists, entity detail, kanban cards, side panel, form widgets) plus the
edit/filter dropdown option text and kanban column headers. Backwards compatible
— label-less enums render raw value.

OUT: changing stored/wire value identity; i18n/multi-locale; labels for non-enum
types; OpenAPI enum output (stays value-only); CLI display of labels.

**Acceptance Criteria:**
1. Named custom type with labels → data-entry select shows label, submits value. (test: schema API returns labels; widget renders label text, `<option>` value is the snake_case value)
2. Inline `type: enum` with labels → same behavior.
3. Badge display (single + multi) shows label; color styling still keys on value. Covers list rows, entity detail, kanban cards, side panel — all funnel through Badge.
4. Enum with no labels → identical to today (raw value shown).
5. Existing string-list `values:` metamodels load without error, no migration.
6. Validation error messages remain value-based and correct.
7. Kanban column headers and filter/edit dropdown option text show the label (label resolved) — no raw/label mismatch across surfaces.

## Research

- [x] ~~run `/research`~~ (N/A: approach clear, decision settled with user)
- [x] Searched for existing patterns
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Enum values are `[]string` end-to-end: YAML → Go (`metamodel.CustomType.Values`, `PropertyDef.Values`) → v1 wire → TS → Vue. No object shape today.
- **`Badge.vue` is the single shared enum render surface** — used directly in `EntityList` (rows+cards), `EntityDetail` (enum cells), `KanbanView` (card fields), `SidePanel`, and via the form widgets. It already imports `useSchemaStore()` for color lookup keyed on `(property, value)`. This is the leverage point.
- Existing label conventions to stay consistent with: `EntityDef.Label`/`LabelPlural`, `InverseDef.Label`. Per-value display string is a keyed set → a map is the right shape; keep the field named `labels`.
- Prior art: TKT-JVA0D/TKT-YR7OW worked these widgets; RR-506Q warns against borrowing a display label as a wire identifier → keep value as identity.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach (sidecar `labels:` map; frontend centered on Badge):**

Add optional `Labels map[string]string` (keyed by value) beside `Values
[]string`. No migration, no tolerant parser.

*Backend:*
1. **Structs** (`internal/metamodel/types.go`): add `Labels map[string]string \`yaml:"labels,omitempty"\``to`CustomType`and`PropertyDef`. **`omitempty` is required** (RR-O2JF23) — AST-based YAML write-back (`schema/cleanup.go:236`, `metamodel/rename.go:84`) preserves the field; omitempty avoids spurious `labels: {}` diffs.
2. **Loader validation** (`internal/metamodel/loader.go`): **silent tolerance** (RR-I57OBI) — a `labels` key not in `values` is harmless (never rendered) and the loader has no warn channel; matches permissive-storage philosophy. No load change beyond parsing.
3. **Validation** (`internal/metamodel/validation.go`): unchanged — stays keyed on `values`; labels never enter error messages.
4. **v1 wire** (`internal/apiwire/v1/responses.go`): add `Labels map[string]string \`json:"labels,omitempty"\``to`PropertyDef`and`CustomType`(mirror`values` omitempty).
5. **v1 serialization** (`internal/dataentry/api_v1.go`): copy `Labels` with the **exact same precedence as `Values`** (RR-UAV796) — `toV1PropertyDef`: `if ct,ok := meta.Types[propDef.Type]; ok { pd.Labels = ct.Labels } else if len(propDef.Values) > 0 { pd.Labels = propDef.Labels }`. Property referencing a custom type gets the custom type's labels; inline `labels` on a custom-typed property is intentionally ignored. Extract a shared `toV1CustomType(ct)` helper so `handleV1Schema`'s custom-type loop and `toV1PropertyDef` don't drift.
6. **OpenAPI self-schema** (`internal/openapi/schemas.go` 373–405): add `labels` to the self-describing PropertyDef/CustomType schema. Enum output (`base.Enum`) stays value-only (add a regression test asserting this).
7. **TS types** (`frontend/src/types/schema.ts`): add `labels?: Record<string,string>` to `PropertyDef` and `CustomType`.

*Frontend (centered on Badge — RR-TRMD4O):*
8. **`Badge.vue`** resolves the label itself via `useSchemaStore()`, keyed on `(property, value)`, same lookup path it uses for color. Display = `labelFor(value) ?? value`; **`:value` prop stays the raw wire value** (color key untouched). This automatically fixes list rows, entity detail, kanban cards, side panel — no per-call plumbing.
- Drop `text-transform: capitalize` when a real label is present (RR-L6XI6S) — the author already chose the display form; only capitalize the fallback raw value.
- Add `max-width` + ellipsis on `.badge` for long labels (RR-KQ1DXS).
9. **`SelectWidget.vue` / `MultiSelectWidget.vue`**: display mode already delegates to Badge → free. For **edit mode**, `<option>` text = label, `:value` = value; TagSelect options show label. (RR-UZ6Q3G)
10. **`KanbanView.vue`** column header: resolve the enum label for the grouping property so header matches card badges (RR-UZ6Q3G).
11. **Filter dropdowns** (`FilterBar`, `AdHocFilterMenu`): option text shows label; filter value stays the raw value (RR-UZ6Q3G).

**Alternatives considered:** object-list `values: [{value,label}]` — rejected
(per user): breaks every `.Values []string` consumer, needs tolerant
unmarshaller + migration. Threading labels through only the form widgets —
rejected (RR-TRMD4O): misses the majority of display surfaces; Badge is the
correct chokepoint.

**Files to modify:**
- `internal/metamodel/types.go`, `loader.go`, `validation.go` (validation: verify-only)
- `internal/apiwire/v1/responses.go`
- `internal/dataentry/api_v1.go` (+ `toV1CustomType` helper)
- `internal/openapi/schemas.go`
- `frontend/src/types/schema.ts`
- `frontend/src/components/common/Badge.vue` (primary frontend change)
- `frontend/src/widgets/SelectWidget.vue`, `MultiSelectWidget.vue` (edit-mode option text)
- `frontend/src/components/.../KanbanView.vue` (column header)
- `frontend/src/components/.../FilterBar.vue`, `AdHocFilterMenu.vue` (filter option text)
- Tests + `docs/metamodel.md`, `docs/data-entry.md`

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `labels` comes from the trusted metamodel YAML (author-controlled, same trust level as `values`). Not end-user input.
- Rendered via Vue `{{ }}` text interpolation (auto-escaped), never `v-html`. VERIFIED in review: the only `v-html` uses are sanitized markdown, none on the enum/label path. Add a test asserting label text is escaped.

**Security-Sensitive Operations:** none (display-only string mapping).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (extend existing tests, don't duplicate):**
- Go — `internal/dataentry/api_v1_test.go`: `toV1PropertyDef`/`handleV1Schema` copy `Labels` with custom-type-vs-inline precedence (AC1/AC2); string-list-only metamodel round-trips unchanged (AC5). `internal/openapi/generator_test.go`: `enum` output stays value-only (reach guard). `internal/metamodel` loader test: parses `labels:` on both forms.
- Frontend — `frontend/src/components/common/Badge.test.ts`: Badge resolves label from store, shows label, color class derived from value, no-label falls back to raw + capitalize (AC3/AC4). `frontend/src/widgets/widgets.test.ts`: SelectWidget `<option>` text=label / value=value; MultiSelect per-chip label.

**Edge Cases:**
- Partial labels → unmapped values render raw (per-element for list enums).
- `labels` key not in `values` → silently ignored.
- Empty `labels: {}` → behaves as no labels.
- Label = a different value's string → harmless for display (doc note).
- Unicode → escaped text, fine.
- Very long label → ellipsis via `.badge max-width` (RR-KQ1DXS).

**Negative Tests:**
- `type: enum` with no `values` still fails load (unchanged).
- Submitting a label string as a value fails enum validation (value-based).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (m)

**Risks:**
- Frontend scope was under-estimated (Badge is used in ~5 surfaces) — mitigated by centering on Badge, which covers them in one place. Still m; edit/filter/kanban option text is incremental.
- XSS via metamodel label → text interpolation only; escaping test.
- Badge color regression → color key stays on value; test asserts color-from-value while text shows label.
- Wire drift (Go struct / OpenAPI self-schema / TS) → update in lockstep; shared `toV1CustomType`; `just arch-lint` + build.
- Author-casing override by `capitalize` → drop capitalize when label present.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md — document `labels:` on enum types (custom + inline), value-stays-identity
- [x] docs/data-entry.md — label rendering across badges/lists/kanban/forms
- [ ] ~~CLI reference~~ (N/A: no CLI change)
- [ ] ~~README~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-TRMD4O (critical, Badge-centered — addressed in
plan §8), RR-L6XI6S (significant, capitalize — §8), RR-UZ6Q3G (significant,
kanban/filter/edit surfaces — §9–11, AC7), RR-UAV796 (significant, label
precedence + shared helper — §5), RR-I57OBI (significant, silent tolerance —
§2), RR-O2JF23 (minor, omitempty — §1), RR-KQ1DXS (minor, ellipsis + :value
guardrail — §8). All folded into the approach; to be marked `addressed` during
implementation.
