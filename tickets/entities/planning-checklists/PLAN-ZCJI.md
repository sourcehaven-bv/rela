---
id: PLAN-ZCJI
type: planning-checklist
title: 'Planning: Add file import and dark mode editing to palette settings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- File picker button to load .gpl, .hex, .txt, .yaml files into the import textarea
- Drag & drop zone on the import area for dropping palette files
- Parse rela `palette.yaml` files (YAML with base/surface/accent/etc + badges + dark)
- Dark mode editing: toggle between Light/Dark in the Appearance section
- When editing dark, show dark palette colors; user can override individual values
- Dark mode control: auto (default), disabled, or explicit overrides
- **Backend: Add JSON serialization to DarkMode type** (design review finding)

OUT of scope:
- Palette export/download (future ticket)
- Dark mode preview (would need to toggle the entire page theme)

**Acceptance Criteria:**

1. File picker button opens native file dialog accepting .gpl, .hex, .txt, .yaml
2. Drag & drop a palette file onto the import area populates the textarea
3. Importing a rela palette.yaml populates both light colors and badges (and dark if present)
4. Light/Dark toggle in Appearance section switches between editing light and dark palettes
5. Dark palette edits are saved as explicit dark overrides in palette.yaml
6. When dark palette has no overrides, auto-generation is used (default behavior)

## Research

- [x] All items checked

**Existing Solutions:** parsePalette, PaletteConfig, backend DarkMode type
(needs JSON methods)

## Approach

- [x] All items checked

**Technical Approach:**

### 0. Backend: DarkMode JSON Serialization (design review fix)

Add `UnmarshalJSON`/`MarshalJSON` to `DarkMode` in `palette.go`:
- `MarshalJSON`: returns `"auto"`, `false`, or the explicit PaletteColors object
- `UnmarshalJSON`: try string → try bool → try object (same pattern as YAML)
- Add tests in `palette_test.go`

### 1. File Picker & Drag/Drop

- Hidden `<input type="file" accept=".gpl,.hex,.txt,.yaml,.yml">` triggered by button
- Drag & drop zone wrapping textarea with `dragover`/`drop` handlers
- Check `file.size < 102400` (100KB) before reading; toast error if too large
- `FileReader.readAsText()` → populate `importText` → auto-trigger import

### 2. YAML Palette Parsing

Extend `parsePalette()` in `palette.ts`:
- Detect rela YAML by checking for `base:` or `surface:` or `accent:` keys
- New `parseRelaPalette()` function:
  - Regex parser: `key:\s*"?#?([0-9a-fA-F]{6})"?` handles quoted/unquoted hex
  - Track indentation to detect `badges:` and `dark:` sections
  - Returns `{ colors, badges, dark }` structured result
- For swatch display: extract all hex values into flat array
- For form population: directly set `paletteColors`, `paletteBadges`, and dark overrides

### 3. Dark Mode Editing

**State:**
- `paletteDarkColors` ref for dark theme overrides
- `paletteDarkBadges` ref for dark badge overrides
- `editingDark` boolean ref — toggles light vs dark editing

**UI:**
- Light/Dark toggle pill above Theme Colors
- When `editingDark=true`: pickers show/edit dark refs; empty fields show auto-generated as placeholder
- When `editingDark=false`: pickers show/edit light refs (current behavior)

**Saving:**
- If any dark colors set → include `dark` object with PaletteColors in API call
- All dark empty → omit `dark` field (defaults to auto)

### 4. API Type Update

```typescript
export interface PaletteConfig {
  // ...existing 8 color fields + badges...
  dark?: PaletteColors | 'auto' | false
}
```

**Files to modify:**

Backend:
- `internal/dataentryconfig/palette.go` — add `UnmarshalJSON`/`MarshalJSON` to DarkMode
- `internal/dataentryconfig/palette_test.go` — JSON serialization tests

Frontend:
- `frontend/src/utils/palette.ts` — add YAML parsing
- `frontend/src/utils/palette.test.ts` — YAML parsing tests
- `frontend/src/views/SettingsView.vue` — file picker, drag/drop, dark mode toggle
- `frontend/src/api/settings.ts` — add dark field to PaletteConfig

## Security Considerations

- [x] All items checked

File picker/drag-drop sandboxed by browser. YAML parsing is regex-based, no
eval. File size capped at 100KB.

## Test Plan

- [x] All items checked

| AC | Test | Type |
|----|------|------|
| 0 | DarkMode JSON marshal/unmarshal round-trips | Unit (Go) |
| 1 | File picker triggers import | Manual |
| 2 | Drag & drop triggers import | Manual |
| 3 | Parse rela palette.yaml | Unit (TS) |
| 4 | Light/Dark toggle swaps palette | Manual |
| 5 | Dark edits saved as explicit section | Manual |
| 6 | Empty dark → auto mode | Unit (TS) |

## Risk Assessment

- [x] All items checked

Effort: **M**

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| RR-IJG9 | critical | DarkMode needs JSON serialization | addressed |
| RR-6383 | significant | Plan incorrectly said no backend changes | addressed |
| RR-1LRP | minor | YAML parser must handle quoted hex | addressed |
| RR-LL33 | minor | File size check before FileReader | addressed |
