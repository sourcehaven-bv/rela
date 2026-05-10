---
id: TKT-WN7O
type: ticket
title: Custom logo upload for data-entry sidebar branding
kind: enhancement
priority: medium
effort: s
status: review
---

Add an optional user-uploaded logo to the data-entry app, displayed in the
sidebar header in place of (or alongside) the existing text `appName`.

This is **PR 1** of the three-PR theme system split. PR 3 (TKT-WPKW) depends on
this for the theme-package zip format.

## Scope

**In scope:**

- Backend endpoints under `/api/v1/_theme/logo`: `GET` (serve bytes), `PUT` (multipart upload), `DELETE` (clear logo).
- Storage: `.rela/theme/logo.<ext>` via the existing `kv` abstraction (same pattern as `palette.yaml`).
- Mime / size validation: PNG, JPEG, SVG, WebP. Max 256 KiB.
- SVG safety: rendered via `<img src>` only — browsers sandbox `<img>`-loaded SVGs (no script execution, no external requests, no DOM access). No server-side sanitizer library; a Go test fixture asserts a known-malicious SVG cannot break the sandbox via `<img>`.
- Frontend: Settings → Appearance section gains a Logo upload control + remove button + preview.
- `Sidebar.vue`: render `<img>` when a logo is set, fall back to text `appName` otherwise.
- Cache-busting via content-hash query parameter so updates are immediate.

**Out of scope (deferred):**

- Multi-resolution rasters (`srcset` / `<picture>` / `@2x`) — see PLAN-KZ5H reasoning. The manifest can grow `logo: string` → `logo: object` later without breaking themes.
- Logo as part of a theme package (covered by PR 3 / TKT-WPKW).
- Custom favicon (different problem; this only changes the sidebar branding).

## Acceptance criteria

1. Settings → Appearance shows a Logo upload control with an Image preview and a Remove button.
2. Uploading PNG / JPEG / SVG / WebP within size limits stores the file under `.rela/theme/logo.<ext>` and immediately replaces the sidebar text with an `<img>` element.
3. Removing the logo deletes the file and falls back to the text `appName`.
4. Files larger than 256 KiB or with unsupported mime are rejected with a clear toast error; existing logo (if any) is unchanged.
5. SVG bundled into a logo with `<script>`, `onload=`, or external `xlink:href` is served untouched but cannot execute any of those vectors when rendered via the sidebar `<img>` (browser sandbox).
6. Sidebar logo CSS sizes the image cleanly (`max-height` / `object-fit: contain`) for both expanded and collapsed sidebar.
