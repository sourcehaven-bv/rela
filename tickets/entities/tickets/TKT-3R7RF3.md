---
id: TKT-3R7RF3
type: ticket
title: Widget override for view section fields (`widget:` on ViewSectionField)
kind: enhancement
priority: medium
effort: m
status: review
---

## Goal

Let view config authors override which widget renders a given property in a view
section, independent of the type-based default.

Split out of TKT-HOIX1, which now covers only the `render: input | display`
axis. This ticket is the orthogonal *which widget* axis and lands after it
(TKT-HOIX1 merged as #1364).

## Scope corrections (verified against develop @ 3a735757)

The original scope was written before TKT-HOIX1 landed and is stale in three
material ways. Verified, not assumed:

1. **`ViewCell.Widget` is LIVE, not dead** (corrected by RR-YTC4W5; my
original claim here was wrong). `cell.Widget = resolveWidget(pd, s.Meta)` at
`internal/dataentry/sections.go:296` populates it, and `v1.ViewCell(cell)`
(`views_handler.go:611,629`) ships it on every table cell. My original grep
(`'Widget:'`) could not match a field *assignment*. Tables remain out of scope,
but on **surface** grounds — a table cell is a different renderer with no
inline-edit path — not because the field is dead. Do not delete it.

Corollary: a **second** server-side type→widget resolver already exists
(`Metamodel.ResolveWidgetFromType`, `schema_output.go:117`), whose godoc claims
to be "the single source of truth". That claim is already false — `registry.ts`
`defaultWidgetFor` is the authority on the section path — and the two have
drifted (`file` → `text` in Go, `'file'` in TS).

2. **The frontend hook is not `registry.ts`.** `defaultRegistry.resolve(name,
propertyDef)` **already** honours an explicit name and falls back to the type
dispatch — that code is written and tested. The gap is purely that the only
view-side caller, `SectionEditForm.vue:147-152` (`widgetRows`), hardcodes
`resolve(undefined, field.propertyDef)`. So this is a call-site change plus
carrying `widget` on `SectionEditField`, exactly parallel to how `render` is
carried. `registry.ts` needs no change.

3. **The `hint` arm must ignore the override — but not for the reason first
given** (corrected by RR-2GBB0V). An unschema'd field is **not** display-only:
`isFieldWritable` (`frontend/src/utils/affordances.ts:12-18`) returns `true` for
a *missing* verdict, and neither `widgetRows` (`SectionEditForm.vue:164`) nor
the edit branch (`:288`) inspects `field.kind` — so a hint-arm field on `render:
input` renders an editable widget and PATCHes. What the hint arm actually lacks
is a `PropertyDef` (`:298`, `:314` pass `undefined`), so the server has
**nothing to type-check a widget name against**. Honouring `widget:` there would
ship an unvalidated widget into a live edit control. Ignore + warn.

## Scope (corrected)

- `ViewSectionField` (`config.go:847-852`) gains an optional `Widget string`,
beside `Render`.
- `SectionFieldData` (`sections.go:63`) + `apiwire/v1.SectionField`
(`responses.go:355`) gain `Widget` in the SAME position — the unnamed conversion
`v1.SectionField(f)` is the compile-checked sync, so position and type must
match.
- `buildSectionFieldData` (`sections.go:82`) carries it through. Note there is
**no section-level inheritance** for widget (unlike `render`): a widget is
inherently per-property, so a section-wide default is meaningless.
- `internal/dataentryconfig/validate.go` validates the name against the
registered set and against the property's type. Note `FormField.Widget` is
currently **not validated at all** — this ticket should not silently inherit
that gap for the new field.
- Frontend: `ViewSectionField` TS gains `widget`; `sectionEditFields.ts` carries
it onto `SectionEditField`; `SectionEditForm.vue` `widgetRows` passes it to
`defaultRegistry.resolve`.
- Omitting `widget` preserves today's type-based selection exactly.

## Why

The payoff case: the Daily-Notes "click checkbox to mark task done" interaction
becomes a config line rather than custom code. Needs `render: input` from
TKT-HOIX1 to be useful, since a checkbox you cannot click is just an icon.

## Non-goals

- No new widget types — only overriding selection among registered ones.
- The `render: input | display` axis (TKT-HOIX1, done).
- The table-cell path (`ViewCell.Widget`) — live and type-derived, different
surface, no inline edit. Not dead; not to be removed.
- Closing the `FormField.Widget` validation gap — its own breaking change.
- Plumbing `_attachments` onto cards/list rows (RR-NGY84F) — follow-up.
- Markdown and relation widgets, which do not exist in the registry.
