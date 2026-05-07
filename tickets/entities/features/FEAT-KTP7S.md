---
id: FEAT-KTP7S
type: feature
title: Editable list widgets and auto-save in data-entry forms
summary: 'Adds the primitives needed to build a daily-notes UX: form auto-save, a relation-list widget (sibling of cards), an `order` property type with declarative `order_by`, in-item interactive fields, and a top-bar nav primitive.'
description: |-
    Background: building a daily-notes / journal UX (per-day entity with notes textarea + curated, tickable, reorderable list of tasks) revealed the data-entry app needs several primitives before such a screen is expressible as a form. Design discussion captured in .ignored/daily-notes-plan.md.

    This feature aggregates the supporting tickets:

    1. Auto-save for forms (debounced per-field PUT, dirty-field reconciliation with SSE, opt-in per form).
    2. New `order` property type (sparse-int or fractional indexing internally, declarative on the metamodel).
    3. `order_by` config on FormRelation, with reorderable rendering in the existing `cards` widget.
    4. New `relation-list` widget (sibling of `cards`): rows with accent / title / subtitle / drag handle.
    5. Interactive fields inside relation-list items (e.g., checkbox to flip status of a target entity from inside the parent's form).
    6. Top-bar nav primitive with a tiny declarative date-expression language ({{today}}, {{entry.date - 1 day}}).

    Each is independently shippable and individually useful; together they unlock the daily-notes UX without inventing a parallel "editable view" rendering pipeline. The screen IS a form — forms are the editable surface, and widgets are render-everywhere.
priority: medium
status: proposed
---
