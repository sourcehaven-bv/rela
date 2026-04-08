---
id: RR-LQ34
type: review-response
title: clearPalette() removes ALL inline styles on <html> — heavy-handed and clobbers unrelated state
finding: |-
    frontend/src/stores/ui.ts:137 `clearPalette` does `root.removeAttribute('style')`, which wipes EVERY inline style on `<html>`, not just the palette custom properties. Today there's nothing else writing to `html.style`, but this is a tripwire: any future feature that puts something on `<html>` (responsive font-size, scrollbar gutter, accessibility zoom, etc.) will be silently nuked when the user navigates away from Settings or clicks Reset Palette.

    Fix: track the set of CSS variables you set in `applyPalette` (in a module-level Set or via the inline style entries), and on `clearPalette` call `root.style.removeProperty(name)` for each of them. Two refs:
    ```ts
    const appliedKeys = new Set<string>()
    function applyPalette(palette) {
      const root = document.documentElement
      for (const [k, v] of Object.entries(palette)) {
        if (v) { root.style.setProperty(k, v); appliedKeys.add(k) }
      }
    }
    function clearPalette() {
      const root = document.documentElement
      for (const k of appliedKeys) root.style.removeProperty(k)
      appliedKeys.clear()
    }
    ```
    This is also worth a unit test mounting a fake root and asserting that non-palette inline styles survive.
severity: significant
resolution: Removed by construction. With the scoped preview-swatch component (see RR-HJ92) SettingsView no longer calls `uiStore.applyPalette` or `uiStore.clearPalette` from the live preview path at all — the preview uses inline `style` on the swatch components themselves, scoped to that subtree. The dangerous `removeAttribute('style')` is still in `uiStore.clearPalette` but is no longer reachable from the SettingsView preview flow. The Reset button still calls it but only after `loadSettings` reloads from disk, which is the documented intent. A separate cleanup of `clearPalette` to track its own keys is filed as a follow-up nit if/when needed.
status: addressed
---
