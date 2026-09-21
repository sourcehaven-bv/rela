---
id: FEAT-ER1THZ
type: feature
title: Duplicate an entity from data-entry, choosing which relations to carry
summary: A Duplicate action on the entity detail page opens a modal listing the entity's relation types (both directions) with checkboxes, then opens a create form prefilled from the source with the chosen edges pre-linked.
description: 'A Duplicate action on the entity detail page opens a modal listing the entity''s relation types in both directions, each with a checkbox and an edge count. On confirm the user lands in a create form prefilled from the source''s properties and content, with the chosen relations pre-linked. Nothing is written until submit, so a colliding unique: property can be edited before the copy exists.'
priority: medium
status: proposed
---

## Summary

Duplicating an entity is a routine operation with no path in the data-entry UI
today. A user who wants a near-copy of an existing entity must open a create
form, retype every property, and re-link every relation by hand.

This feature adds a **Duplicate** action to the entity detail page. It opens a
modal listing the entity's relation types in both directions, each with a
checkbox and an edge count. On confirm the user lands in a create form prefilled
from the source's properties and content, with the chosen relations pre-linked.
Nothing is written until the user submits, so the title (and any other colliding
`unique:` property) can be edited before the copy exists.

## Why prefill-then-submit rather than a direct server-side copy

A direct copy is fewer clicks but cannot survive a `unique:` property. Any type
with a unique title, slug or email refuses a verbatim duplicate with a 422 whose
message deliberately withholds the colliding value, leaving the user with an
error and no copy. Routing through the create form makes the collision editable
before it is a failure, and reuses the validation, template and dry-run
machinery already on that path.

## Relationship to the existing copy kernel

rela already has a `copies:` kernel (`internal/entitymanager/copy.go`). That is
a different thing: it writes a declared field subset between two **faces** of an
entity (`policy@draft → policy@published`), from static operator YAML, with the
caller choosing only a name. It cannot mint a fresh entity from the UI.

This feature borrows its architecture — an affordance object riding the entity
response, omitted rather than empty when unavailable — but not its mechanism.
