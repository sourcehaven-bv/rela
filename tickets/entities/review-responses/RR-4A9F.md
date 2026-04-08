---
id: RR-4A9F
type: review-response
title: loadPaletteState collapses both `dark === undefined` and `dark === false` to Regular but the user intent differs
finding: |-
    frontend/src/views/SettingsView.palette.ts:67-91 maps:
    - `dark === false` → Regular
    - `dark === undefined` → Regular (no project intent)
    - `dark` is object → Light+Dark

    This loses information. If the user previously saved Light+Dark with all dark slots empty (`dark: {}`, intentional 'inherit from light'), the editor correctly shows Light+Dark on next load. Good. But if the project ships `dark: { accent: '#x' }` and the user has no `palette.yaml`, the load function only sees the user palette (per RR-R73K), so `data.userPalette` is undefined → `loadPaletteState(undefined, ...)` → Regular. The editor doesn't show the project's dark state to the user at all. (See RR-R73K for the deeper fix.)

    In isolation, this function is fine; the issue is that its caller passes only the user palette and not the merged resolved palette, so important information is invisible to the user. Add at minimum a TS doc-comment on `loadPaletteState` saying 'this is the *user overlay only*; project-level dark settings are not visible here', so the next person reading the code knows that `state.mode === 'regular'` does not mean 'no dark theme is rendered'.
severity: minor
resolution: Resolved as a side effect of fixing RR-R73K. `loadPaletteState` no longer collapses `dark === undefined` and `dark === false` to the same Regular state — it now consults the third `resolvedDarkDisabled` parameter for the undefined case and selects Light+Dark if the project provides a dark theme. Updated docstring in SettingsView.palette.ts explains the new precedence rules explicitly.
status: addressed
---
