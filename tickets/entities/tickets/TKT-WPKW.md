---
id: TKT-WPKW
type: ticket
title: 'Theme packages: export/install bundled palette + logo + font as .relatheme zip'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Bundle the existing palette + (eventual) logo + (eventual) font into a portable
`.relatheme` zip so users can share themes between rela data-entry apps.

This is **PR 3** of the three-PR theme system split. It depends on:

- Logo support (PR 1) — adds `.rela/theme/logo.<ext>` and `/api/v1/_theme/logo` endpoints.
- Font support (PR 2) — adds `.rela/theme/font.<ext>` and `/api/v1/_theme/font` endpoints + `@font-face` injection.

**A theme package** is a zip with extension `.relatheme` containing:

- `theme.yaml` — manifest (name, version, author, palette fields, optional `logo` / `font` references)
- `logo.<ext>` — optional, bundled by PR 1's storage
- `font.<ext>` — optional, bundled by PR 2's storage

**Export flow:** From Settings, click Export — backend reads palette + logo +
font and returns a `.relatheme` zip download.

**Install flow:** From Settings, upload a `.relatheme` zip. Backend persists
logo/font bytes (atomic, since they're binary), validates and returns the
palette JSON. Frontend stages the palette into the existing palette editor; user
clicks Save to persist the colors — matching the current palette UX where colors
only persist on explicit Save.

## Acceptance criteria

1. Settings page exposes Export and Install buttons in the Appearance section.
2. Export produces a `.relatheme` zip containing `theme.yaml` plus whichever of `logo.<ext>` / `font.<ext>` are set.
3. Install accepts a `.relatheme` zip, validates manifest, persists logo/font, and stages palette into the editor for user-confirmed Save.
4. Invalid / corrupt theme files surface a clear toast error and leave existing settings unchanged.
5. Theme persistence reuses existing storage patterns — palette stays in `palette.yaml`, logo/font in `.rela/theme/`, plus a new `.rela/theme/manifest.yaml` for the metadata fields (name, version, author, font.family).
6. Custom CSS, multi-resolution rasters, multi-theme libraries, drag-and-drop, theme registries, and signing are explicitly **out of scope** for this PR (see PLAN-KZ5H).
