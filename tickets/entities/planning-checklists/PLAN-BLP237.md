---
id: PLAN-BLP237
type: planning-checklist
title: 'Planning: Matrix card: full integer axis range, axis titles and order; schema min/max on integers'
started: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- `min:` / `max:` on `integer` properties in `schema.yaml`; write-time warning
  (soft `invalid_value`, 200 with warning per DEC-HWZHA), `analyze properties`
  finding, `_schema` exposure, `min`/`max` on the form number input.
- Load errors: `min > max`; `min`/`max` on any type other than `integer`
  (`max` keeps its meaning on `file`, where `min` is refused); `min`/`max` on a
  `list` integer.
- Matrix card: declared range fills the whole axis; otherwise fill lowest..highest
  observed; out-of-range and non-integer values count as not placed; cap 50
  (load-time for a declared range, render-time `truncated` otherwise).
- Axis titles on the matrix response (`row_title`, `column_title`), defaulting to
  the property label, else the name; `rows_label:` / `columns_label:` override.
- `rows_order:` / `columns_order:` (`asc` default, `desc`), applied to enum and
  integer axes alike.
- Docs: GUIDE-metamodel and GUIDE-data-entry in `docs-project/`, regenerated docs.

Out of scope:

- A `number`/float property type: rela has no such built-in type (only
  `integer`), so there is nothing to give min/max to.
- Relation-count constraints (`min_outgoing` etc.) are unrelated and unchanged.
- Configure-space editing of matrix cards (still YAML-only, as in TKT-2SGPJZ).

**Acceptance Criteria:**

1. `impact: {type: integer, min: 1, max: 5}` loads; `min: 5, max: 1` and
   `min: 1` on a `string` fail the load with a message naming the property.
2. Writing `impact: 7` through the API succeeds with a `property_value_invalid`
   warning at `/properties/impact`; `analyze properties` reports a stored 7.
3. `/api/v1/_schema` carries `min`/`max` on the property; the form number input
   has `min`/`max` attributes.
4. A matrix over impact 1..5 with data only at 2 and 4 shows rows 1,2,3,4,5.
5. Without min/max, data at 1 and 4 shows 1,2,3,4.
6. A value 9 on a 1..5 axis is in `unplaced`, not a row.
7. The card shows row and column titles: the property label, or `rows_label` /
   `columns_label`.
8. `rows_order: desc` lists rows 5..1.
9. A declared range wider than 50 is a config load error; an observed spread
   wider than 50 is truncated and flagged.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach decided with the user as option B)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo validation and fold code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: JSON Schema minimum/maximum is the obvious analogue)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `metamodel.PropertyDef.Max` already exists as the `file` attachment cap
  (`max:` YAML key, `FileMax()`), and `v1.PropertyDef.Max` carries it on the wire.
- `metamodel.Validator.validatePropertyValue` is the one place every write path
  (entitymanager) and `schema.ValidateEntityProperties` (analyze properties,
  validate, migrations) checks a value; `invalid_value` is already soft.
- `internal/dataentry/dashboard_fold.go` `axisValues` builds the matrix axes;
  `validateMatrixCard` checks matrix config at load.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `metamodel.PropertyDef`: `Min *int` (`yaml:"min"`) and `Max` changed from
   `int` to `*int`, so `max: 0` is expressible for an integer. `max` keeps its
   file meaning; `FileMax()` reads the pointer. Loader: the file-options check
   reads the pointer; a new range check refuses min/max on non-integer types
   (except `max` on `file`), on list properties, and `min > max`.
2. `validatePropertyValue` integer case: after the type check, compare the
   integer with min/max; out of range is `ValidationErrorInvalidValue`. Soft, so
   writes warn and analyze reports.
3. Wire: `v1.PropertyDef` gets `Min *int` and `Max *int` (same JSON key `max`);
   MCP schema tool and OpenAPI get min/max too (cheap). SPA `PropertyDef` gains
   `min`; `NumberWidget` binds `min`/`max` from `propertyDef`.
4. Matrix: `axisProperty` gains `min/max *int`, `title`, `desc`. An integer
   axis: lo = min or lowest observed integer, hi = max or highest observed; fill
   lo..hi capped at 50 (truncated); a value outside lo..hi or not an integer is
   unplaced. `desc` reverses the axis (enum too). `DashboardMatrix` gains
   `row_title`, `column_title`.
5. Config: `DashboardCard` gains `RowsLabel`, `ColumnsLabel`, `RowsOrder`,
   `ColumnsOrder`; validated as matrix-only, order in {asc, desc}, declared range
   width <= 50.
6. SPA `DashboardMatrix.vue`: a header row with the column title spanning the
   value columns; a row-title `th` with `rowspan` beside the row values; the
   unplaced note names the titles and mentions out-of-range values (en + nl).

Alternatives rejected:

- A separate key (`minimum`/`maximum` or `range:`): the user asked for
  `min`/`max`, and `max` already means "the most" on a property.
- Keeping `Max int`: `max: 0` would be read as unset on an integer.
- Hard 422 on out-of-range: a hand-edit can produce it, so DEC-HWZHA makes it soft.

**Files to modify:**

- internal/metamodel/types.go, loader.go, validation.go (+ tests)
- internal/attachment, internal/mcp tests (Max pointer)
- internal/apiwire/v1/responses.go, internal/dataentry/api_v1.go
- internal/mcp/tools_schema.go, internal/openapi/schemas.go
- internal/dataentryconfig/config.go, dashboard_cards_validate.go (+ tests)
- internal/dataentry/dashboard_fold.go, dashboard_card.go (+ tests)
- frontend/src/types/schema.ts, types/config.ts, widgets/NumberWidget.vue,
  components/dashboard/DashboardMatrix.vue (+ tests), i18n/locales/en.json, nl.json
- docs-project/entities/guides/GUIDE-metamodel.md, GUIDE-data-entry.md; docs/

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- schema.yaml / data-entry.yaml (operator config, not secret): validated at load.
- Entity values (user input): the range check gives a soft warning that names
  only the bound.
- Observed matrix spread: bounded at 50 before any allocation, so a value of
  1e9 cannot make the server build a huge axis.

**Security-Sensitive Operations:**

- The fold still runs on row-gated, redacted rows; a hidden axis value stays
  `partial` and never sets the observed range, so axis extent discloses no
  hidden value. Titles come from config and schema, which are not secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: loader table test (valid range, min>max, min on string, max on boolean,
  min on file, min/max on list integer, max: 0 on integer accepted).
- AC2: validator table test (below, above, in range, string "7", float 7.0);
  dataentry API write test asserting 200 + warning; analyze via
  schema.ValidateEntityProperties.
- AC3: `toV1PropertyDef` test; NumberWidget unit test for min/max attributes.
- AC4-6, 8: `foldMatrix` table tests (declared, observed fill, out of range,
  non-integer, desc on integer and enum, only-min declared).
- AC7: card endpoint test for titles (label, name fallback, override).
- AC9: config validation test (range 1..60 refused; bad order refused; labels
  on non-matrix refused); fold test with spread 0..1e9 truncated.
- SPA: DashboardMatrix test for titles, rowspan/colspan.
- Manual: binary from the branch against a copy of the demo project.

**Edge Cases:**

- No rows: a declared range still shows the full grid; no declared range and
  no observations gives an empty axis.
- Hidden axis value: counted as partial, does not widen the observed range.
- Negative ranges and `max: 0`.
- Only one of min/max declared.

**Negative Tests:**

- Config errors above; out-of-range writes warn, not reject.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- `Max` int to pointer touches file-attachment code: the compiler finds every
  site; existing attachment tests cover behaviour.
- `max` on the wire now means two things by type: documented on the wire type;
  the file widget only renders for file properties.
- Rebase conflicts with TKT-5GX50Y and the colspan work in locale files and
  dashboard components: keep the frontend diff to DashboardMatrix.vue and the
  locale lines it needs.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/metamodel.md (via GUIDE-metamodel) - min/max on integer properties
- [x] docs/data-entry.md (via GUIDE-data-entry) - matrix range, titles, order

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-I3EV4O (overflow-safe width), RR-10LOPK (one
declared bound, no data), RR-LSNVJF (range kept out of the data shape),
RR-VA41OH (string and float integers), RR-O3XBS3 (NumberWidget reads the def).
All minor or nit, all addressed in the plan.
