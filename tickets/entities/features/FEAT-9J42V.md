---
id: FEAT-9J42V
type: feature
title: Explicit display property on entity-type definitions
description: 'Optional `display_property: <name>` field on entity-type definitions, declaring which property the runtime should use as the human-readable display name. When set, overrides the existing autoderivation (title/name/label priority + alphabetical fallback).'
status: proposed
---

## Problem

Today, rela's metamodel has no way to declare which entity-type property should
be used as the human-readable display name (the value rendered in lists, cards,
link text, breadcrumbs, etc.). The runtime auto-derives it from a hardcoded
priority list — `title`, `name`, `label`, then "any required string property in
alphabetical order" — falling back to the entity ID.

Two failure modes:

1. **Non-English schemas.** Dutch projects use `naam` / `titel`; the
priority list misses both, falling through to the alphabetical fallback. Today
this happens to pick the right property because every such type has exactly one
required string property; if a second one is later added (e.g. `samenvatting`),
the display silently flips.
2. **Implicit intent.** A reader of `metamodel.yaml` cannot tell which
property is the display name. Tooling (graph rendering, exports, MCP listing)
hardcodes the same auto-derivation logic, so a metamodel author can't override
it.

## Solution

Add an optional `display_property: <name>` field to the entity-type definition.
When set, `EntityDef.GetPrimaryProperty()` returns it without consulting the
priority list. When unset, the existing autoderivation continues unchanged.

Validation at metamodel-load time: error if `display_property` names a property
that isn't defined on the entity. (No type/required check — the metamodel author
has stated explicit intent.)

## Why now

A Dutch user-project (the VWS gf-architectuur metamodel, 17 entity types) has
hit the implicit-fallback case. Adding the explicit field is a 2-day change in
rela; it unblocks the user-project's display-name cleanup and guards every
future Dutch (or non-English-priority) metamodel from the same brittleness.

## Out of scope

- Templated display formats (e.g. `display_format: "{naam} ({status})"`).
Single-property reference only.
- Per-context overrides (display in lists vs. graph vs. export).
- Internationalisation of `label`/`label_plural`.
