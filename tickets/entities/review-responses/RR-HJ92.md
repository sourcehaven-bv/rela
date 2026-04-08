---
id: RR-HJ92
type: review-response
title: Live preview applies inline styles to <html> — Regular×global-dark renders Frankenstein theme
finding: |-
    frontend/src/views/SettingsView.vue:242 `buildPreviewPalette` correctly *picks* the right palette per (mode, global toggle), but the underlying `uiStore.applyPalette` (stores/ui.ts:127) writes the result via `document.documentElement.style.setProperty(...)` — i.e. inline styles on `<html>`. Inline styles defeat both `:root { ... }` and `:root.dark { ... }` selectors in App.vue.

    Concrete failure case: user has the global dark toggle ON, navigates to Settings, paletteMode is `regular`. The watch fires, `visibleDark` is false, the LIGHT palette is pushed as inline styles. The `<html>` element still has class `dark` (the global toggle was not changed), so any non-custom-property dark CSS rules still apply (e.g. `.dark .sidebar { background: #x }`) but the custom-property values that THOSE rules consume are forced light by the inline styles. Result: the user sees a half-light, half-dark page mid-edit — exactly the 'jankiness' the ticket set out to fix, just in a slightly different shape.

    A second case: user is in Light+Dark mode, global dark OFF, edits a Dark column field. The watch fires with `visibleDark=false`, recomputes the LIGHT preview (which is unchanged), and re-applies it. So the dark edit is invisible — fine for the user, but it means the only way to see your dark preview is to flip the global toggle, which is a discoverability issue.

    Fixes: (a) when applying a 'light' preview, also force-remove the `.dark` class on `<html>` (and restore on unmount); (b) write CSS custom properties into a `<style>` tag with `:root { ... }` and `:root.dark { ... }` blocks instead of inline; (c) at minimum, render the Dark column edits into a scoped preview *inside* the Settings view (a small color-swatch box) instead of trying to live-preview the whole app.
severity: critical
resolution: Replaced the global live preview with a small scoped preview-swatch component embedded in SettingsView. Each swatch pane has the resolved CSS-variable map applied as inline `style` on its own root element, so descendants read via `var(--xxx)` and the preview is fully scoped to that subtree. The rest of the application keeps using whatever palette is actually saved. There is no longer any code path in SettingsView that writes to `<html>` inline styles or injects global `<style>` rules. Both Light and Dark themes are visible side-by-side at all times in Light+Dark mode, regardless of the global dark toggle. Manually verified through Puppeteer in all four mode combinations.
status: addressed
---
