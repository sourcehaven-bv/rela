---
id: FEAT-8OTJVW
type: feature
title: Color property type with data-entry color picker
description: A built-in `color` metamodel property type storing a CSS hex string (#RRGGBB), validated against ^#[0-9A-Fa-f]{6}$, rendered in data-entry as a native color picker in edit mode and an accessible swatch (with the hex value as text) in display mode. Motivated by CalDAV calendar-color and category/tag colors, which today can only be modelled as an unvalidated string.
status: proposed
---

## Summary

A built-in `color` property type storing a CSS hex string (`#RRGGBB`), rendered
in data-entry as a native color picker with a swatch in read-only surfaces.

## Motivation

Several surfaces want a user-chosen color as *data*, not as configuration:

- **CalDAV / calendar feeds** — the `COLOR` / Apple `calendar-color` property on a
published calendar, so a feed subscribed in a client shows in the operator's
chosen color.
- Category / tag / status colors that today can only be faked with an `enum`
plus a hand-maintained style map.

Today the only way to model this is `type: string`, which gives no validation (a
typo yields an invalid CSS color that fails silently downstream) and a bare text
input.

## Shape

```yaml
properties:
  calendar_color:
    type: color
    default: "#3B82F6"
```

Stored verbatim in YAML frontmatter as a string; validated against
`^#[0-9A-Fa-f]{6}$`. No alpha channel — `#RRGGBB` only, keeping every consumer
(CSS, CalDAV, the picker element) on one representation.

## Out of scope

- Alpha (`#RRGGBBAA`), named CSS colors, `rgb()`/`hsl()` notation.
- A curated palette / theme-aware swatch set — that is an `enum` with a style
map, a different feature.
- Consuming the color in the calendar feed — a separate ticket depends on this.
