---
id: RR-PLT2
type: review-response
title: SettingsView.vue is 1559 lines — well above the 500-line lint warning, the palette extraction barely dented it
finding: |-
    The frontend CLAUDE.md says `max-lines: 500` warning is on for Vue files specifically to catch god components. After this PR SettingsView.vue is 1559 lines (the prompt mentioned 1347; my checkout shows 1559). The extraction of `SettingsView.palette.ts` only pulled out two pure functions — meanwhile the file still mixes:

    - Property defaults grid (load/save/UI)
    - Relation defaults grid (load/save/UI)
    - Override groups grid (load/save/UI)
    - Palette editor (eight refs, eight setters, derive flow, live preview watch, file import, drag/drop, swatch picking)
    - Application info card

    At least three obvious extractions:
    1. **PaletteEditor.vue** as a child component receiving the editor state via v-model. Owns: paletteColors, paletteBadges, paletteDarkColors, paletteMode, derive button, file import, swatch picking, the live preview watch. Easily 500-700 lines on its own. SettingsView orchestrates and wires the save call.
    2. **DefaultsEditor.vue** for property + relation defaults (currently ~150 lines of nearly-duplicated grid markup with subtle widget switches per type).
    3. **OverridesEditor.vue** for the override groups (another ~200 lines, also duplicated grid logic).

    Benefits beyond line count: each piece becomes independently testable with `@vue/test-utils` (today the only thing tested is the extracted pure helper); the live-preview watch can be properly scoped to the PaletteEditor's lifecycle (and unmounted when navigating to another tab in Settings, instead of relying on the full SettingsView unmount); the duplicated grid widget switches become a single FieldRenderer.

    This isn't a 'while you're here' suggestion — at 1559 lines a single Vue component is genuinely hard to reason about, and the live preview bugs in this same review (RR-HJ92) are direct consequences of that complexity.
severity: significant
reason: 'SettingsView.vue was already over the lint warning threshold (~1395 lines) before this PR — the file has been a known cleanup target for a while. This PR added the new preview-swatch component and side-by-side layout (~150 net lines), bringing it to ~1742. Extracting PaletteEditor.vue / DefaultsEditor.vue / OverridesEditor.vue is the right call but it''s a substantial refactor that touches state ownership, prop drilling, and slot composition — too risky to bundle into this PR alongside the critical correctness fixes. Filing as a follow-up: SettingsView component extraction (split palette editor, defaults editor, overrides editor into focused subcomponents). The lint warning continues to fire (suppressed at the file level).'
status: deferred
---
