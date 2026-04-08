---
id: TKT-HIAX
type: ticket
title: Simplify palette settings — Regular vs Light+Dark mode with explicit Derive
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

The palette Settings → Appearance section is "janky" for several compounding
reasons (confirmed by manual Puppeteer testing):

1. **Backend bug**: saving a partial `dark: { accent: "#xxx" }` override
causes `ResolvePalette` to fill the unset dark fields with `defaultLightColors`
instead of auto-derived dark, clobbering the sidebar / surface / badges.
2. **Live preview writes to the wrong CSS scope**: editing a Dark color
pushes the dark hex into the same `:root` CSS variables used by light mode, so
the page renders with garbage colors mid-edit.
3. **Auto-derivation is invisible**: dark inputs show placeholders that
look identical to real values; users can't tell what is theirs vs what is
derived.
4. **Per-role text input is whitespace-fussy**: pasting ` #ffcd75`
gets stored verbatim and rejected by the backend hex regex.
5. **No way to disable dark mode** from the UI even though backend
`DarkMode.IsDisabled()` (`dark: false`) supports it.

## Scope

Replace the in-form Light/Dark editing pill with a top-level palette mode switch
and make auto-derivation an explicit user action.

- **Regular** mode: single column of color inputs (today's layout
minus the dark machinery). Saves `dark: false`.
- **Light + Dark** mode: two columns side-by-side per role and per
badge. Above the Dark column, a **Derive Dark from Light** button populates all
8 dark base colors + 7 dark badges from the current light palette using the
existing HSL math. If any dark slot is already non-empty, an inline confirm
appears before overwriting. Saves an explicit `dark: { ... }` object.

Backend simplification:

- Drop `auto` from `DarkMode`. The union becomes two-state: `false`
(disabled) or an explicit object.
- Delete `generateDark` / `generateDarkBadges` from Go — auto-derivation
lives in the frontend now, invoked by the Derive button.
- Port `generateDark` / `generateDarkBadges` to TypeScript in
`frontend/src/utils/palette.ts`, with Vitest goldens locked against the current
Go output to guarantee parity before deletion.

UX fixes bundled in:

- **Whitespace trim + normalize** in `setColor` / `setBadge`.
- **Live preview scope fix**: only apply the active mode's palette to
the visible CSS variables (Regular mode always renders as light; Light+Dark mode
follows `uiStore.darkMode`).

## Out of scope

These get filed as separate bugs:

- Backend serves stale palette after `.rela/palette.yaml` is deleted
from disk (file watcher misses deletions).
- Bulk-import textarea doesn't accept rela palette.yaml format
(parity gap with `Browse File`).
- "Reset" button scrolls page to top.

## Migration note

Existing `.rela/palette.yaml` files with `dark: auto` will fail to load after
this change. Users need to either delete the `dark` field or open Settings and
re-save.
