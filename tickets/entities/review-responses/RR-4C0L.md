---
id: RR-4C0L
type: review-response
title: z-index 10000 vs EasyMDE 9999 is a fragile magic-number race
finding: |-
    `EntityPickerModal.vue` hardcodes `z-index: 10000` with a comment explaining it sits one above EasyMDE's fullscreen 9999. This is brittle for three reasons:

    1. **External dependency tracking.** The constant lives in CSS in *our* code. The number it relies on lives in `MarkdownEditor.vue`'s fullscreen rules AND inside EasyMDE's own CSS. A future EasyMDE upgrade that bumps to 10000 (or any value ≥ 10000) silently puts the editor toolbar back on top of the picker overlay. There's no compile-time or runtime check.

    2. **No SSOT.** `MarkdownEditor.vue` declares 9999 in five `!important` rules; the picker compensates with 10000. If someone tunes one without the other, they break in different fullscreen sub-modes (side-by-side, preview, etc.).

    3. **Teleport stacking context surprise.** The picker is `<Teleport to="body">`, so it lives in the root stacking context. EasyMDE's fullscreen also uses position: fixed at the root. They share a stacking context today, but if someone wraps the app in a `<div style="position: relative; z-index: 0">` (common pattern), the picker's z-index will be clamped to that wrapper's context while EasyMDE's fullscreen — because it's `position: fixed` — may still escape to the root, inverting the stacking order.

    Fix options:
      - Define `const Z = { editor_fullscreen: 9999, modal_overlay: 10000, modal_overlay_above_editor: 10001 }` in a shared `src/styles/z-index.ts` so the relationship is captured in one place.
      - More robust: exit fullscreen on picker open. `editor.codemirror.toggleFullScreen()` (EasyMDE API) flips the mode. The picker user is going to need the page back anyway. Saves the whole class of stacking bugs.
      - Document the assumption in a top-of-file comment plus an e2e test that opens the picker WHILE the editor is in fullscreen mode and asserts overlay visibility. The current e2e suite does not cover this path.
severity: minor
reason: Single hardcoded z-index with a comment explaining the EasyMDE 9999 ceiling. Adopting a project-wide z-index constants table is a broader refactor (the codebase has multiple ad-hoc z-index values today). The auto-exit-fullscreen alternative is a UX choice that should not change without explicit design input.
status: deferred
---
