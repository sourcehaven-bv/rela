---
id: FEAT-YMNB
type: feature
title: User-customizable branding (logo + font) for data-entry apps
summary: Lets users upload a custom logo and a custom UI font to brand their data-entry app, complementing the existing color palette customization (FEAT-4OEJ).
description: Lets users upload a custom logo and a custom UI font to brand their data-entry app, complementing the existing color palette customization (FEAT-4OEJ). Implemented incrementally by TKT-WN7O (logo) and TKT-KE0C (font); both are bundled into portable theme packages by TKT-WPKW.
priority: medium
status: proposed
---

Sister feature to **FEAT-4OEJ** (Customizable color palette). FEAT-4OEJ covers
colors; this covers the other two visual customization vectors:

- A custom logo image displayed in the sidebar header (replacing or
supplementing the text `app.name` from `data-entry.yaml`).
- A custom UI font applied via `@font-face` as the default for the SPA.

Together with FEAT-4OEJ, these are the three independent inputs the theme
packaging feature (TKT-WPKW / `.relatheme` zip) bundles for portability.

## Why two assets in one feature

Logo and font are *different* asset types but share a single design pattern:

- Both stored in `.rela/theme/<name>.<ext>` via the existing `kv` abstraction.
- Both served via `GET /api/v1/_theme/<name>` with cache-busting query params.
- Both uploaded via `PUT /api/v1/_theme/<name>` (multipart) and removed via `DELETE`.
- Both validated by mime/magic-byte allowlist + size cap.
- Both applied by SPA bootstrap code on page load.

Splitting feature ↔ ticket: this single feature is implemented by two
independent tickets (TKT-WN7O for logo, TKT-KE0C for font) so PR 1 and PR 2 ship
in isolation.

## Out of scope (for this feature)

- Multi-resolution raster bundles for the logo.
- External web fonts (Google Fonts, etc.).
- Custom CSS overrides — captured separately (no ticket yet).
- Bundling logo + font into a portable archive — that is the theme-packages
feature (TKT-WPKW).
