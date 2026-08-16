---
id: TKT-HOIX1
type: ticket
title: 'View section fields render as display by default; opt in to inline edit with `render: input`'
kind: enhancement
priority: medium
effort: m
status: done
---

## Goal

Give view-config authors per-field control over whether a detail-page property
renders as a **view-oriented display value** or an **editable inline input** —
and flip the default to display.

Before this, there was no config surface at all: `ViewSectionField` was just
`{property, label}`. Whether a field rendered as an input was decided entirely
by the ACL verdict — a *permission*, not a presentation choice.

## Config surface

```yaml
views:
  ticket:
    sections:
      - heading: Details
        display: properties
        render: input              # optional section-wide default
        fields:
          - property: title
            render: display        # overrides the section
          - property: status       # inherits `input` from the section
```

- `ViewSectionField.render: input | display`, defaulting to `display`.
- `ViewSection.render` sets a section-wide default that contained fields override.
- Resolution order: field → section → `display`, resolved **server-side** by the single
`dataentryconfig.ResolveFieldRender` helper.

## BREAKING CHANGE (accepted, no migration)

Inline edit on view sections is now **opt-in**. Existing `data-entry.yaml` files
lose inline editing until they add `render: input`. Deliberate; no
`internal/migration` step.

In-repo configs were migrated as part of this ticket (28 sections gained a
section-level `render: input`): `tickets/data-entry.yaml` (23 sections / 57
fields) and `prototypes/data-entry/project` (5 / 24).
`prototypes/data-entry/catalog` has no `views:` block.

**Synthesized default views also render display.** `buildDefaultViewConfig`
(`internal/dataentry/default_view.go:33`) generates a view for every entity type
with no `views:` entry — most types, including `ticket` in this repo's own
tracker. Those set no `Render`, so they resolve to display like any unset
config. This was a deliberate choice for consistency (found during
implementation, not at design review): an operator who wants inline edit on such
a type must author an explicit view. Pinned by
`TestBuildDefaultViewConfig_RendersDisplayByDefault`.

`render: input` yields an input only when the ACL also permits the write. Config
can downgrade an editable field to display; it can never upgrade a read-only
one.

## Key finding: the display rendering already existed

`SectionEditForm.vue` already branched **per field**, three ways: `transitions`
→ `StatusControl`; `row.writable` → `FieldShell` + widget `mode="edit"`; else →
bare widget `mode="display"`. That third arm is genuine view-oriented rendering
with no form chrome, already in production use via ACL verdicts. This ticket
feeds config into the same branch.

NOT the `FormField.readonly` path (`FieldRenderer.vue:82`), which renders a
**disabled input**.

## What shipped

1. `internal/dataentryconfig/config.go` — `Render` on `ViewSection` and `ViewSectionField`;
`RenderDisplay`/`RenderInput` constants; `ResolveFieldRender`.
2. `internal/dataentryconfig/validate.go` — `validateSectionRender` runs **outside** the
source-resolution guards (RR-4ICH8M), covering both section- and field-level
values; `inertSectionRenderWarnings` warns for `display: table` / `content`
(RR-675AA0).
3. `internal/dataentry/sections.go` — `Render` on `SectionFieldData`, populated in both
builders via the shared helper; `buildSectionEntityData` gained a
`sectionRender` parameter.
4. `internal/apiwire/v1/responses.go` — `Render` on `SectionField`, same position. The unnamed
`v1.SectionField(f)` conversion exists at **four** sites —
`views_handler.go:147, 161, 562` and `api_v1.go:2321` (the cards/list row path,
RR-1V04ZD). The compiler flagged all four.
5. `frontend/src/api/views.ts` + `sectionEditFields.ts` — `render` threaded through as its own
property, never folded into `verdict`.
6. `SectionEditForm.vue` — `writable: field.render === 'input' && isFieldWritable(field.verdict)`
at the `widgetRows` site only; `StatusControl` gated on `render`; long-text
`.property-long` handling ported from `PropertyDisplay`.
7. `e2e/tests/fixtures.ts` — `render: input` on the `Implements` list section so the #997
regression guard keeps mounting a `SectionEditForm` (RR-UQ2MIV).
8. `docs/data-entry.md` — new "Field Render Modes" section, section/field tables, and the
breaking-change note. Also corrected the stale line calling views "read-only
detail pages".

## Deviation from the plan

**`sectionShouldRouteToInlineEdit` DID change**, contrary to the plan's
instruction to leave it untouched. The plan's reasoning (RR-8EISWO) was that an
all-display section must not mount an autosave host — that conclusion stands and
is unchanged. But leaving the predicate keyed on the verdict alone would have
produced exactly the outcome RR-8EISWO argued against: an all-display section
would still satisfy "some field is writable" and mount a `SectionEditForm` whose
every field then renders display. The predicate now applies the same `render ===
'input' && isFieldWritable(verdict)` conjunction as `widgetRows`, so the mount
decision and the per-field rendering agree. Pinned by AC 9 (`returns false when
fields omit render, even with writable verdicts`).

## Traps (verified against source — do not skip)

- **Keep `render` separate from `verdict`, and do NOT pass it as `isFieldWritable`'s second
`fieldReadonly` parameter** (RR-PGGRBD). It is called at both the widgetRows
site and the flip-watcher; applying it at both makes a display field look like a
revoked permission and fires the spurious "Permission changed — your unsaved
edit was discarded" toast.
- **The ACL conjunction order is load-bearing.** Never weaken it to `||` or a ternary that lets
config win.
- **Two builders.** `sections.go` constructs `SectionFieldData` in two places — both must use
the shared resolution helper.

## Non-goals

- **Widget override** (`widget:` on a view section field) — TKT-3R7RF3.
- Side-panel inline edit. Side panels reuse `ViewSection` configs so they receive `render` on
the wire, but `SidePanel.vue` has no inline-edit renderer and ignores it
(RR-4O96FZ).
- The pre-existing `mapFieldsToProperties` staleness for mixed sections (RR-8EISWO).
- Markdown and relation display rendering — no widgets in the registry.
- No new widget types; no content-section inline edit; no bulk/row-level actions.

## Design review

Eight findings, all addressed in PLAN-6RDYUL: RR-1V04ZD, RR-8EISWO, RR-UQ2MIV,
RR-4ICH8M (significant); RR-4O96FZ, RR-9S63SJ, RR-675AA0, RR-PGGRBD (minor).
