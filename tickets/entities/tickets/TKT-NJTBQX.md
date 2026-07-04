---
id: TKT-NJTBQX
type: ticket
title: display_property as a template (multi-property title)
kind: enhancement
priority: medium
effort: s
status: review
---

## TL;DR

Extend `display_property` to accept a template string with `{property}`
placeholders (e.g. `"{voornaam} {tussenvoegsel} {achternaam}"` → `"Jeroen
Vloothuis"`), in addition to the existing bare-property-name form. Backwards
compatible: a value with no `{` routes to the existing bare-name path. Extends
FEAT-9J42V.

## Why

`EntityDef.DisplayTitle` (`internal/metamodel/entity_def.go`) resolves a single
`GetPrimaryProperty()` name today. Works for a single "primary" field (`titel`,
`naam`); breaks the moment the natural human name is a concatenation (`voornaam`
+ `tussenvoegsel` + `achternaam`) — the list/dropdown/link shows only one field
or the ID.

```yaml
persoon:
  display_property: "{voornaam} {tussenvoegsel} {achternaam}"   # "Jeroen Vloothuis"
```

## Design

### Detection
A value containing `{` is a template; otherwise a bare property name (existing
behaviour verbatim). Property names can't contain `{`, so detection is
unambiguous. No new YAML key, no migration.

### Rendering (in `DisplayTitle`)
- `{propname}` → the property's stringified value (`fmt.Sprintf("%v", val)`; `nil` → empty).
- literal text passes through.
- Consecutive whitespace collapses to one space; result is trimmed (so an empty
`tussenvoegsel` yields a single space, not a double).
- Empty after trim → fall back to the entity ID (existing unset-display_property behaviour).

### `GetPrimaryProperty()` — returns `""` for templates
**Locked decision:** the display property is READONLY-derived. A template has no
single writable target, and (since TKT-CW96FU removed `create --title`) nothing
writes into `GetPrimaryProperty()` anymore. So it returns `""` when the value is
a template — `DisplayTitle` handles templates before consulting it.

### Load-time validation (in `validateDisplayProperty`, loader.go)
- Parse the template, extract all placeholder names.
- Each placeholder must reference a defined property (existing bare-name rule, per-placeholder).
- Existing type restriction (reject date/file/rrule/list) applies per referenced property.
- Malformed templates (unclosed `{`, empty `{}`) → error at load with the offending template + entity type.

### Dropped from scope
- **Inline `{prop:fallback}` syntax** — no compelling use case; whitespace-collapse already handles the empty-middle-field case.
- Full expression language, localised formatting, multi-line titles.

## Acceptance criteria

1. `"{voornaam} {achternaam}"` + `{voornaam:"Jeroen", achternaam:"Vloothuis"}` → `"Jeroen Vloothuis"`.
2. `"{voornaam} {tussenvoegsel} {achternaam}"` + tussenvoegsel empty → `"Jeroen Vloothuis"` (single space).
3. All placeholders empty → entity ID.
4. Bare name `"achternaam"` (unchanged) → renders the value verbatim.
5. `"{achternaam}, {voornaam}"` → `"Vloothuis, Jeroen"` (literal comma passes through).
6. `"{unknown_prop}"` → error at load.
7. `"{voornaam"` (missing `}`) → error at load.
8. `GetPrimaryProperty()` on a template-valued entity returns `""`.

## Files

- `internal/metamodel/entity_def.go` — `DisplayTitle`, `GetPrimaryProperty`, `renderDisplayTemplate` + parse helper.
- `internal/metamodel/loader.go` — `validateDisplayProperty` placeholder parsing.
- `internal/metamodel/entity_def_test.go`, `loader_test.go` — tests.
- `docs/metamodel.md` (via docs-project source) — document template syntax.
