---
id: TKT-WPKW
type: ticket
title: 'Theme packages: export/install bundled palette + logo as .relatheme zip'
kind: enhancement
priority: medium
effort: m
status: review
---

Bundle the existing palette + logo into a portable `.relatheme` zip so users can
share themes between rela data-entry apps.

This is **PR 2** of the theme system. Depends on:

- **Logo support** (TKT-WN7O, merged) — provides `.rela/theme/logo` storage and `/api/v1/_theme/logo` endpoints.
- **Color palette** (FEAT-4OEJ, already in `develop`) — provides `palette.yaml` storage and `/api/v1/_palette` endpoints.

Custom UI fonts are **out of scope** (the font ticket TKT-KE0C is parked
indefinitely; the `.relatheme` format does not include a font slot).

**A theme package** is a zip with extension `.relatheme` containing:

- `theme.yaml` — manifest (name, version, author, palette fields, optional `logo` reference)
- `logo.<ext>` — optional, bundled by TKT-WN7O's storage

**Export flow:** From Settings, click Export — backend reads palette + logo and
returns a `.relatheme` zip download.

**Install flow:** From Settings, upload a `.relatheme` zip. Backend persists
logo bytes (atomic, since they're binary), validates and returns the palette
JSON. Frontend stages the palette into the existing palette editor; user clicks
Save to persist the colors — matching the current palette UX where colors only
persist on explicit Save.

## Acceptance criteria

1. Settings page exposes Export and Install buttons in the Appearance section.
2. Export produces a `.relatheme` zip containing `theme.yaml` plus `logo.<ext>` if a logo is set.
3. Install accepts a `.relatheme` zip, validates manifest, persists the logo (if present), and stages palette into the editor for user-confirmed Save.
4. Invalid / corrupt theme files surface a clear toast error and leave existing settings unchanged.
5. Theme persistence reuses existing storage patterns — palette stays in `palette.yaml`, logo stays at `.rela/theme/logo` + sidecar. The `theme.yaml` manifest is created at export time only and is not persisted on the receiving side outside the zip.
6. Custom CSS, custom fonts, multi-resolution rasters, multi-theme libraries, drag-and-drop, theme registries, and signing are explicitly **out of scope** for this PR (see PLAN-KZ5H).
