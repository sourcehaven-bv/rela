---
id: RR-5QTT
type: review-response
title: Import silently overwrites unsaved palette draft
finding: 'SettingsView.vue:159-179 + applyImportedPalette:181-192 — User with edited-but-unsaved palette colors clicks Install; editor refs are stomped without warning. Mirror the existing showDeriveConfirm flow: detect dirtiness against schemaStore.paletteLight/Dark and prompt ''Replace your unsaved palette draft?'' before applying. At minimum, surface a Reset link in the success toast so the user can recover.'
severity: significant
resolution: 'Theme install now calls isPaletteEditorDirty() and, when dirty, prompts via the project''s useConfirm composable: ''Replace your unsaved palette draft? — Installing a theme will replace the colors in the palette editor. Your previously saved palette on disk is not affected, but any unsaved edits in the editor will be lost.'' If user cancels, the file input is reset and no import occurs. Saved palette in palette.yaml is untouched either way.'
status: addressed
---
