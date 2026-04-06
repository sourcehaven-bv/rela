---
id: PLAN-UJ1T
type: planning-checklist
title: 'Planning: Add rrule property type with data-entry UI widget'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- New `rrule` built-in property type in the metamodel
- Backend validation of RRULE strings (via teambition/rrule-go)
- Data-entry UI widget for building RRULE strings visually
- Human-readable preview of the rule in the widget
- DTSTART enforcement when INTERVAL > 1

OUT of scope:
- RRULE occurrence expansion/preview (showing next N dates)
- EXDATE/RDATE support
- RRULE validation Lua helpers (already exist)
- Changes to CLI create/update commands (they accept any string)

**Acceptance Criteria:**
1. AC1: A property with `type: rrule` in metamodel.yaml is accepted and validated
2. AC2: RRULE values are validated on save — invalid RRULE strings are rejected with a clear error
3. AC3: INTERVAL > 1 without DTSTART is rejected with a clear error message
4. AC4: Data-entry form renders an RRULE builder widget with frequency, interval, weekday, day-of-month, DTSTART fields
5. AC5: The widget outputs a valid RRULE string that works with `rela.rrule_next()`
6. AC6: The widget shows a human-readable preview (e.g., "Every 2 weeks on Monday")
7. AC7: Existing RRULE values can be loaded into the widget for editing

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Go backend:
- `teambition/rrule-go` v1.8.2 already in go.mod — use `StrToROption` for validation

JS frontend:
- `rrule` npm package (~3k stars, TypeScript, 1M+ weekly downloads) — parse, build, and `toText()` for human-readable output
- No production-ready Vue 3 RRULE builder component exists — build custom widget
- `react-rrule-builder-ts` exists as reference for UI layout

Codebase patterns:
- Built-in types defined in `internal/metamodel/types.go:143-150`
- Type validation in `internal/metamodel/validation.go:113-232`
- Widget rendering in `frontend/src/components/forms/FieldRenderer.vue:21-51`
- Widget types in `internal/dataentryconfig/config.go:20-29`
- Schema API in `internal/dataentry/api_v1.go:62-106`

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Backend (Go)

1. Add `PropertyTypeRrule = "rrule"` constant to `internal/metamodel/types.go`
2. Add `"rrule"` to `IsBuiltinType()` in same file
3. Add validation case in `internal/metamodel/validation.go:validatePropertyValue()`:
   - Parse with `rrule.StrToROption()` — reject if parse fails
   - Reject INTERVAL > 1 without DTSTART (same logic as `luaRruleNext`)
4. Add `WidgetRrule = "rrule"` constant to `internal/dataentryconfig/config.go`

### Frontend (Vue 3)

1. Add `rrule` npm dependency
2. Create `frontend/src/components/forms/RruleBuilder.vue` — custom widget:
   - Frequency dropdown (daily/weekly/monthly/yearly)
   - Interval input (number, shows DTSTART date picker when > 1)
   - Weekday checkboxes (shown for weekly frequency)
   - Day-of-month selector (shown for monthly: specific day, last day, Nth weekday)
   - DTSTART date picker (shown when interval > 1)
   - Human-readable preview via `rrule.toText()`
   - Emits the serialized RRULE string on change
   - Can hydrate from existing RRULE string for editing
3. Update `FieldRenderer.vue` to render `RruleBuilder` when `propertyDef.type === 'rrule'` or `field.widget === 'rrule'`
4. Update `frontend/src/types/schema.ts` PropertyDef type union

**Files to modify:**

Backend:
- `internal/metamodel/types.go` — add PropertyTypeRrule
- `internal/metamodel/validation.go` — add rrule validation case
- `internal/metamodel/validation_test.go` — add rrule validation tests
- `internal/dataentryconfig/config.go` — add WidgetRrule constant

Frontend:
- `frontend/src/components/forms/RruleBuilder.vue` — new component
- `frontend/src/components/forms/FieldRenderer.vue` — add rrule case
- `frontend/src/types/schema.ts` — add rrule to type union
- `frontend/package.json` — add rrule dependency

**Alternatives considered:**
- Use custom type with regex validation instead of built-in type — rejected because RRULE validation requires actual parsing (regex can't validate semantic correctness like valid BYDAY values)
- Use `widget: rrule` override on a string property — works but loses backend validation; the type should carry the validation semantics

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- RRULE string from form submission → validated by `rrule.StrToROption()` (parse-based, not regex)
- RRULE string from markdown files → same validation on sync/load
- No file access, auth, or crypto involved

**Security-Sensitive Operations:**
- None — RRULE strings are data, not code. The rrule-go library only parses options, doesn't execute anything.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: Unit test in validation_test.go — property with type rrule passes validation with valid RRULE
2. AC2: Unit test — invalid RRULE string rejected with ValidationError
3. AC3: Unit test — INTERVAL=2 without DTSTART rejected
4. AC4-AC7: Manual verification via data-entry UI + E2E test if feasible

**Edge Cases:**
- Empty string (should pass if not required, fail if required)
- RRULE with just FREQ (simplest valid rule)
- RRULE with all options (FREQ, INTERVAL, BYDAY, BYMONTHDAY, DTSTART, COUNT, UNTIL)
- RRULE: prefix (strip before validation)
- Very long RRULE strings

**Negative Tests:**
- `INVALID_RRULE` → validation error
- `FREQ=INVALID` → validation error
- `FREQ=WEEKLY;INTERVAL=2` (missing DTSTART) → validation error
- `FREQ=WEEKLY;BYDAY=XX` (invalid weekday) → validation error

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- `rrule` JS library bundle size — mitigated by lazy loading the component
- Widget complexity for monthly rules (Nth weekday) — mitigate by keeping UI simple, progressive disclosure
- RRULE string format differences between Go and JS libraries — mitigate by testing roundtrip

Effort: **M** (mostly frontend widget work)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] User guide / reference docs (metamodel docs — new property type)
- [ ] ~~CLI help text~~ (N/A)
- [ ] ~~CLAUDE.md~~ (N/A)
- [ ] ~~README.md~~ (N/A)
- [ ] ~~API docs~~ (N/A)

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- Pending -->
