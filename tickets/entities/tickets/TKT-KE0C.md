---
id: TKT-KE0C
type: ticket
title: Custom UI font upload for data-entry
kind: enhancement
priority: medium
effort: s
status: backlog
---

Add an optional user-uploaded UI font to the data-entry app, applied as the
default font-family for the SPA via `@font-face`.

This is **PR 2** of the three-PR theme system split. PR 3 (TKT-WPKW) depends on
this for the theme-package zip format.

## Scope

**In scope:**

- Backend endpoints under `/api/v1/_theme/font`: `GET` (serve bytes), `PUT` (multipart upload, includes font-family in form field), `DELETE` (clear font).
- Storage: `.rela/theme/font.<ext>` for bytes; `.rela/theme/manifest.yaml` for the `font.family` metadata (the only metadata captured for v1 — name/version/author live with theme packages).
- Mime / magic-byte validation: WOFF2, WOFF, TTF, OTF. Max 2 MiB.
- Frontend: Settings → Appearance gains a Font upload control + family-name input + Remove button + preview text.
- `applyFont(url, family)`: injects an `@font-face` `<style>` element into `<head>` and sets `:root { --ui-font: "<family>", -apple-system, ... }`.
- `App.vue`: existing global `font-family` becomes `var(--ui-font, <existing system stack>)` so the system stack remains the fallback.

**Out of scope:**

- Web fonts loaded from external URLs (Google Fonts, etc.). Local upload only.
- Multiple bundled font weights / styles. Single font file = single weight.
- Font as part of a theme package (covered by PR 3 / TKT-WPKW).
- Per-component font choices (mono vs body, etc.).

## Acceptance criteria

1. Settings → Appearance shows a Font upload control with a family-name input, preview text in that font, and a Remove button.
2. Uploading WOFF2 / WOFF / TTF / OTF within size limits stores the file under `.rela/theme/font.<ext>`, persists the family name, and immediately applies it across the SPA via `@font-face` + `--ui-font` CSS var.
3. Removing the font deletes the file and reverts to the system font stack.
4. Files larger than 2 MiB or that fail magic-byte validation are rejected with a clear toast error; existing font (if any) is unchanged.
5. Family-name input is validated (1–64 chars, `[A-Za-z0-9 _-]`) — used inside the `@font-face` declaration.
6. A licensing reminder is shown next to the upload control: "Only upload fonts you are licensed to redistribute / use locally."
