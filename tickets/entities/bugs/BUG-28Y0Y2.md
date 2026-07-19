---
id: BUG-28Y0Y2
type: bug
title: Badge colors never resolve when a property's name differs from its custom-type name
description: Badge.vue looks up styling by property name, but the server emits the styles map keyed by custom-type name. They only coincide when a property happens to be named identically to its type, so most enum/status badges render gray regardless of the configured styles.
priority: medium
effort: s
why1: Badge.vue indexes the /_config styles map by property name, but buildStyleMap keys it by custom-type name (validated so by validateStyles), so the lookup misses whenever property name ≠ type name and falls back to gray.
why2: The styles wire field is an untyped Record<string, Record<string, string>> on both sides; the outer key's semantics live only in Go comments and server-side validation, so the SPA consumer assumed property-name keys.
why3: Badge's unit tests seed schemaStore.styles keyed by property names, encoding the wrong assumption, and no test or e2e wires a real type-keyed _config payload through to a rendered badge color, so the mismatch shipped as silent cosmetic gray.
why4: 'No test wires a realistic server payload through to a rendered badge: frontend unit tests hand-construct store state instead of deriving fixtures from the wire contract, and no e2e asserts badge colors, so a key-semantics mismatch had no failing check anywhere.'
why5: Cross-boundary wire contracts between the Go server and the TS SPA are informal duck-typed JSON maps whose key semantics live in comments and server-side validation only — nothing shared or generated makes a consumer's wrong assumption a compile- or CI-time failure.
prevention: 'Regression tests (badge-style-type-key-test) pin the type-keyed resolution end-to-end: schema.test.ts covers stylesForProperty''s resolution/fallback/disambiguation and Badge.test.ts renders realistic type-keyed store state to CSS classes, so a future consumer assuming property-name keys fails CI. The styles-key contract is now documented at both ends (stylesForProperty comment references buildStyleMap/validateStyles).'
status: done
---

## Summary

Badge.vue looks up styling by **property name**, but the server emits the styles
map keyed by **custom-type name**. They only coincide when a property happens to
be named identically to its type, so in practice most enum/status badges render
gray regardless of the configured `styles:`.

## Where

- **Server** (`internal/dataentry/app.go`, `buildStyleMap`) builds `StyleMap` keyed by type name (`cfg.Styles` is keyed by type; it also auto-assigns colors per custom type). Emitted on `/api/v1/_config` as `styles`.
- **SPA** (`frontend/src/stores/schema.ts`) stores it verbatim: `styles.value = configData.styles` — a `Record<typeName, Record<value, cssClass>>`.
- **SPA** (`frontend/src/components/common/Badge.vue`, ~L36): `const propStyles = schemaStore.styles[props.property]` — indexes by property name, not type name. Miss → `badge--gray`.

## Repro

Metamodel: `types: { ticket-status: {values: [todo, doing, ...]} }`, entity
property `status: { type: ticket-status }`. Config: `styles: { ticket-status: {
todo: yellow, ... } }` (valid — must be keyed by a defined type). GET any entity
→ the status badge is gray. Confirmed the same in the in-tree `tickets/` project
(`styles: ticket_status:`, property `status`) — its badges are gray too.

## Impact

Every badge across lists, kanban, detail view, and the new status control.
Purely cosmetic (no data/security effect), but the `styles:` config is
effectively dead for any type ≠ property naming.

## Fix direction

Resolve property → its custom-type name, then key styles by that type (falling
back to the property name for inline enums the server may key by property). The
schema store already has the exact pattern in `labelsForProperty`
(`customTypes.value.get(def.type)`); add a sibling `styleFor(property, value,
entityType)` and point Badge at it. One shared component, ~15 lines, plus a
`Badge.test.ts` case where property ≠ type.
