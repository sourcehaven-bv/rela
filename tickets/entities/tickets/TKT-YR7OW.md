---
id: TKT-YR7OW
type: ticket
title: Relation pickers should display name + id, not id alone
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

Relation pickers in data-entry forms (`RelationPicker.vue`, and the related
`RelationCards.vue` / `LinkExistingModal.vue`) currently fall back to showing
only the entity id when an entity has no `title` property, and the
selected-entity chip never shows the id. Users find it hard to identify the
linked entity in either case.

## Goal

In both the dropdown candidate list AND the selected chip, show the
human-readable display name (title) together with the id, formatted
consistently. Where the title is missing, the id alone is fine (no duplication).

## Scope

In scope: `RelationPicker.vue`, optional consistency fix in `RelationCards.vue`
/ `LinkExistingModal.vue`.

Out of scope: introducing a configurable display-property; entity list views;
non-form pickers.
