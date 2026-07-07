---
id: FEAT-JIBWQP
type: feature
title: Human-readable labels for enum property values
summary: Allow the metamodel to declare a display label/title for each enum value so snake_case values render as human-friendly text in data-entry UI while keeping the value as the stored identity.
description: Allow the metamodel to declare an optional display label per enum value so snake_case values render as human-friendly text in data-entry select widgets and badges, while the stored value remains the snake_case identity.
status: proposed
---

## Feature: Human-readable labels for enum values

Enum property values are snake_case identifiers (e.g. `in_progress`,
`wont_fix`). Today the data-entry UI renders these raw in select dropdowns and
badges, which is poor UX.

This feature lets the metamodel author declare an optional human-readable
**label** for each enum value. The label is display-only: the stored value (and
everything keyed on it — validation, badge color lookup, transitions, option
verdicts, existing entity data) stays the snake_case identifier.

Applies to both enum declaration forms:
- Named custom types (`types:` section)
- Inline `type: enum` properties

Backwards compatible: enums without labels continue to render the raw value.
