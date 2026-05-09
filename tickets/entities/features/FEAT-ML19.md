---
id: FEAT-ML19
type: feature
title: Refreshed default theme
summary: Better default light + dark palette for the data entry UI, with self-hosted Open Sans.
description: 'Replaces the old default palette with a calmer, more legible one — cream paper bg + harbor-navy sidebar + #4772fb accent for light, deep harbor-navy + lifted blue for dark. Open Sans is bundled via @fontsource (SIL OFL 1.1) so the SPA has zero runtime dependency on Google Fonts. Default dark mode is now on out-of-the-box; projects that prefer light-only can still opt out with `dark: false`.'
priority: low
status: implemented
---

## Implementation

- `internal/dataentryconfig/palette.go` — new `defaultLightColors`, `defaultDarkColors`, and matching badge maps. `ResolvePalette` now seeds dark mode from the new defaults instead of `Disabled: true`; explicit `dark: false` opt-out still wins.
- `frontend/src/App.vue` — `:root` and `:root.dark` CSS vars synced to the same palette so the SSR fallback matches.
- `frontend/src/main.ts` — imports `@fontsource/open-sans` weights 400/500/600/700 instead of pulling from fonts.googleapis.com.

## Why bundle the font

Operating rela should not depend on a working connection to Google Fonts. SIL
OFL 1.1 explicitly permits redistribution.

## Verification

- `just ci` passes locally.
- Verified at runtime via puppeteer: zero off-origin requests on page load, Open Sans woff2 served from rela's `/assets/`.

Shipped via PR #670.
