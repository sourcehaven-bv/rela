---
id: RR-F371WB
type: review-response
title: Vendor CSS (EasyMDE/CodeMirror/Font Awesome) swept into @layer, undermining a documented swap seam
finding: 'A generateBundle post-plugin wraps EMITTED assets, so all bundled third-party CSS lands inside
  @layer rela: easymde/dist/easymde.min.css (MarkdownEditor.vue:4), font-awesome/css/font-awesome.min.css
  (MarkdownEditor.vue:10), and four @fontsource/open-sans/*.css (main.ts:8-11). The plan covers this in
  half a sentence with the mitigation ''full visual/e2e pass; watch MarkdownEditor and Badge/TagSelect''
  — that is a hope, not a mitigation for a cascade change. Two unexamined consequences. (1) @font-face
  is now nested a level deeper; needs an explicit check that fonts still load, since a silently-missing
  icon font in the editor toolbar is a visible regression and vite.editor.config.ts:12-45 shows this project
  has been bitten by Font Awesome CSS structure assumptions before. (2) It hands operators zero-specificity
  override power over .CodeMirror / .editor-toolbar / .fa-* internals. internal/dataentry/CLAUDE.md:181-188
  is emphatic that the editor''s DOM and styling are explicitly NOT a contract precisely so the editor
  can be swapped later. Layering makes those internals trivially skinnable, operators will skin them,
  and a future CM6 swap breaks every such skin — quietly undermining a deliberate documented seam.'
severity: significant
status: addressed
resolution: 'Vendor CSS stays IN the flat layer (uniform, simpler). docs/customisation.md states explicitly
  that .CodeMirror/.editor-toolbar/.fa-* are tier 3, outside the stability promise, and will break on
  an editor swap. Verified no font/icon regression: full e2e suite (241 tests) green including the markdown-editor
  specs.'
---

## Recommended resolution

Decide explicitly whether third-party CSS is in the layer.

- **If in** (simpler, uniform): AC6 docs must state that non-rela-authored
selectors are explicitly *outside* the tier-2 stability promise — skinning
`.CodeMirror` is tier 3, no contract, breaks on any editor swap.
- **If out**: the plugin must distinguish vendor CSS, which is real complexity
the L estimate does not currently cover.

Interacts with [[rr-important-inversion]]: several of the 17 `!important` rules
exist only to beat EasyMDE, and layering the vendor CSS may make them removable.
