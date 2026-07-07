---
id: TKT-G6R5YE
type: ticket
title: Enum values support a display label/title for better UX on snake_case values
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

Enum property values are snake_case identifiers (`in_progress`, `wont_fix`,
`needs_investigation`). The data-entry UI renders them raw in select dropdowns
and badges — poor UX for end users who see `in_progress` instead of "In
Progress".

## Goal

Allow the metamodel author to declare an optional display **label** per enum
value. The label is display-only; the stored/wire value stays the snake_case
identifier so validation, badge color lookup, transitions, option verdicts, and
existing entity data are unaffected.

## Scope

**In scope**
- Metamodel: optional per-value labels for both named custom types (`types:`) and inline `type: enum` properties.
- Backend serialization to the frontend (v1 `_schema` API) carries labels.
- Frontend: `SelectWidget`, `MultiSelectWidget`, `Badge` render the label when present, value otherwise.
- Backwards compatibility: label-less enums render the raw value exactly as today.

**Out of scope**
- Changing the stored value or wire identity of any enum value.
- i18n / multi-locale labels.
- Labels for non-enum property types.
- OpenAPI `enum` output (stays value-only — JSON Schema enums are values, not labels).

## Acceptance criteria

1. A named custom type can declare labels; the data-entry select shows the label, submits the value.
2. An inline `type: enum` property can declare labels with the same behavior.
3. A badge (display mode, single and multi) shows the label while color styling still keys on the value.
4. An enum with no labels behaves identically to today (raw value shown).
5. Existing metamodels (string-list `values:`) load without error and without migration.
6. Validation error messages remain correct (value-based).

## Open design decision

Label shape — sidecar map vs. object list — to be settled in planning. See the
technical approach.
