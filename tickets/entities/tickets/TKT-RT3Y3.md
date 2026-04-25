---
id: TKT-RT3Y3
type: ticket
title: Add display_property to entity-type metamodel
kind: enhancement
priority: medium
effort: s
status: review
---

## Problem

`EntityDef.GetPrimaryProperty()` (in `internal/metamodel/entity_def.go`)
auto-derives the entity's display-name property from a hardcoded priority list:
`title` → `name` → `label`, then "any required string property in alphabetical
order," falling back to the entity ID. There is no way for a metamodel author to
declare the display property explicitly.

Concrete failure mode (VWS gf-architectuur, 17 entity types in Dutch): every
type uses `naam` or `titel` as its display field. None match the English
priority list, so each falls through to the alphabetical fallback. It happens to
pick the correct property today because every type has exactly one required
string property — but if a second is ever added, the display silently flips. The
fragility is invisible in `metamodel.yaml`.

## Scope

**In scope**

1. Add `DisplayProperty string \`yaml:"display_property,omitempty"\``to`EntityDef`in`internal/metamodel/types.go`.
2. Update `GetPrimaryProperty()` to return `e.DisplayProperty` when
non-empty, before falling back to the existing auto-derivation.
3. Validate at metamodel-load time: if `display_property` names a
property that isn't defined on the entity, fail with a clear error citing the
entity type, the offending property name, and the list of defined properties.
4. Update `docs-project/.../GUIDE-metamodel.md` (and the derived
`docs/metamodel.md`) entity-types table with the new field.
5. Tests:
   - `GetPrimaryProperty` returns `display_property` when set
   - `GetPrimaryProperty` falls through to autoderivation when unset
   - Metamodel load errors when `display_property` names a missing
property; succeeds when name matches
   - `DisplayTitle` integrates correctly (uses the explicit property's
value, falls back to ID when value is missing)

**Out of scope**

- Multi-property formats (`display_format: "{naam} ({status})"`) — flag
for a future ticket if it becomes load-bearing.
- Per-context overrides (display vs. list vs. graph).
- Updating user metamodels (VWS, dogfood `tickets/`, etc.) — that's a
separate adoption step; this ticket only adds the capability.
- Frontend changes — the SPA reads `_title` (server-computed) which
flows through `Metamodel.DisplayTitle`, so no client work needed.

## Acceptance criteria

AC1. **Field accepted in YAML.** Adding `display_property: titel` to an
entity-type definition loads without error and is reflected in the parsed
`EntityDef`.

AC2. **Explicit override.** When `display_property` is set,
`GetPrimaryProperty()` returns its value, bypassing the priority list and the
alphabetical fallback. `DisplayTitle` then reads the property's value from the
entity (or ID fallback).

AC3. **Backwards compatibility.** Entity types without `display_property` return
the same primary property as today (priority list → alphabetical fallback).
Existing tests pass unchanged.

AC4. **Validation: missing property.** Loading a metamodel where an entity
declares `display_property: nonexistent` fails with a clear error referencing
the entity type and listing the available property names. The error must be
surfaced via the existing metamodel validation path (not a panic).

AC5. **Documentation.** `docs-project/.../GUIDE-metamodel.md` entity-types table
gains a `display_property` row; `docs/metamodel.md` rebuilt by `just docs`.

## Why now

Follows on from TKT-JIEKC's clarification of the data-entry rendering pipeline.
The user is starting display-name cleanup on a 17-type Dutch metamodel and the
implicit fallback is brittle. Adding the explicit field is a small change in
rela that unblocks the cleanup and guards every future non-English metamodel.
