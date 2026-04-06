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
4. AC4: Data-entry form renders an RRULE builder widget with frequency, interval, weekday, DTSTART fields
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

Codebase patterns:
- Built-in types defined in `internal/metamodel/types.go:143-150`
- Type validation in `internal/metamodel/validation.go:113-232`
- Widget rendering in `frontend/src/components/forms/FieldRenderer.vue:21-51`
- Schema API in `internal/dataentry/api_v1.go:62-106`

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Backend (Go)

1. Add `PropertyTypeRrule` constant, update `IsBuiltinType()`
2. Shared `ValidateRrule()` in `internal/metamodel/rrule.go` — used by both metamodel validator and Lua helper
3. Add validation case in `validatePropertyValue()`
4. Add `WidgetRrule` constant and `ResolveWidgetFromType` case

### Frontend (Vue 3)

1. Custom `RruleBuilder.vue` widget with `rrule` npm package
2. Frequency, interval, weekday checkboxes, DTSTART, human-readable preview
3. `formatValue()` renders human-readable text in entity detail and list views
4. No client-side validation needed — widget constructs valid strings, backend validates

**Files modified:**

Backend: `types.go`, `validation.go`, `validation_test.go`, `schema_output.go`,
`config.go`, `rrule.go` (new), `date.go` (refactored) Frontend:
`RruleBuilder.vue` (new), `FieldRenderer.vue`, `schema.ts`, `format.ts`,
`package.json`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- RRULE string from form submission → validated by `rrule.StrToROption()` (parse-based)
- No file access, auth, or crypto involved

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1-AC3: Unit tests in `validation_test.go` — valid rules, invalid rules, INTERVAL/DTSTART enforcement
2. AC4-AC7: Manual verification via data-entry UI with puppeteer

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

Effort: **M**

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: metamodel reference docs deferred)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-OX8G (addressed), RR-6PGS (addressed), RR-Y203
(addressed), RR-9CT2 (addressed), RR-NNJC (addressed), RR-GY03 (deferred)
