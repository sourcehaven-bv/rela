---
id: DOCS-WGTOVR
type: docs-checklist
title: 'Docs: Widget override for view section fields'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `ViewSectionField.Widget` explaining it is passed through verbatim (the SPA's registry owns resolution) and why it is field-level ONLY, unlike `Render` — a section-level widget would be a config error on every field of a non-matching type
- [x] Godoc on `sectionFieldWidgetTypes` recording that it is deliberately NOT derived from `Metamodel.ResolveWidgetFromType` (that resolver serves table cells, has no `file` case, and has already drifted), and that it is only the TYPE half of the rule
- [x] Godoc on `widgetAcceptsProperty` documenting the precedence it mirrors (`list` → values → type) and, for each rule, the concrete failure it prevents — notably the list-flattening PATCH
- [x] Godoc on `inertWidgetWarnings` covering both warned cases and stating explicitly why the state-machine case is NOT warned (machine-ness is runtime, per-principal; unbuildable at config load)
- [x] Comment on `SectionEditForm.widgetRows` recording that the override applies on the schema arm only, and why the hint arm cannot validate one
- [x] Comment on `WIDGET_REGISTRATIONS` explaining it is exported data so the drift guard can assert the REAL registrations
- [x] ~~CLAUDE.md pattern update~~ (N/A: follows the existing `Render` thread and the `validRelationWidgets` allowlist pattern; introduces no new convention)

## Project Documentation

- [x] `docs/data-entry.md` — new **Widget Overrides** section: the accepted widget/type table, field-level-only rule with rationale, the `render:` pairing ("a checkbox you cannot click is just an icon"), the `widget: file` properties-only restriction, and the two ignored cases
- [x] `docs/data-entry.md` — **Field Render Modes** section added for TKT-HOIX1's `render:`, which shipped undocumented (its PR touched no docs file at all; `docs/` is generated from `docs-project/entities/`, so the edit was silently overwritten)
- [x] Section-field table extended with the `render` and `widget` keys; the per-field key table (`property`/`label`/`span`/`render`/`widget`) added, which did not previously exist
- [x] Edited the SOURCE (`docs-project/entities/guides/GUIDE-data-entry.md`) and regenerated, not the generated file

## External Documentation

- [x] ~~README / external docs~~ (N/A: a `data-entry.yaml` config key, covered by the data-entry guide)

**Docs verified:** `just docs` regenerated cleanly and `just ci`'s
docs-freshness gate passes. The documented widget/type table matches
`sectionFieldWidgetTypes` and the shared drift fixture; the documented
restrictions match the implemented errors and warnings.
