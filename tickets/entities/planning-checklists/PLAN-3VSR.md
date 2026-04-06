---
id: PLAN-3VSR
type: planning-checklist
title: 'Planning: Add customizable color palette to data-entry apps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Add optional `palette` section to `data-entry.yaml` with up to 8 named color roles (all optional, defaults fill gaps)
- Optional `badges` subsection to customize the 7 badge colors (blue/purple/green/gray/red/orange/yellow)
- Auto-derive remaining 6 CSS variables from the 8 base colors
- Auto-generate dark mode from light palette (darken surfaces, lighten text)
- Support explicit dark mode override or `dark: false` to disable dark mode entirely
- Serve resolved palette (all 14 CSS vars + 7 badge vars for light + dark) via `/api/v1/_config`
- Frontend applies palette overrides to `:root` CSS variables at runtime
- Refactor Badge.vue to use CSS custom properties instead of hard-coded hex
- Palette customization in Settings page (persisted to `.rela/palette.yaml`)
- User palette overrides project palette (resolution: user > project > defaults)
- Hot-reload palette on config change via file watcher

OUT of scope:
- Custom fonts or typography
- Theme "presets" or palette sharing
- Lospec API integration (users copy-paste colors manually)
- Additional badge color names beyond the existing 7

**Acceptance Criteria:**

1. A `palette` section with named colors overrides the default CSS variables
   - Test: configure `palette.accent: "#e11d48"`, verify accent changes in UI
2. Partial palette works — unset fields fall back to built-in defaults
   - Test: only set `accent`, verify all other vars use defaults
3. Remaining 6 CSS vars (card-bg, input-bg, hover-bg, border-color, muted-text, sidebar-text) auto-derived
   - Test: set `surface: "#f8fafc"`, verify card-bg/input-bg/hover-bg are derived shades
4. Optional `badges` section customizes badge colors
   - Test: set `badges.blue: "#1e40af"`, verify blue badges use new color
5. Dark mode auto-generated from light palette when not explicitly set
   - Test: set only light palette, toggle dark mode, verify dark variant is generated
6. `dark: false` disables dark mode toggle entirely
   - Test: set `dark: false`, verify toggle hidden, always light
7. Explicit dark palette overrides auto-generation
   - Test: set both light and dark palettes, verify explicit dark values used
8. Invalid color values rejected at config load time
   - Test: set `accent: "not-a-color"`, verify validation error
9. User palette (`.rela/palette.yaml`) takes priority over project palette
   - Test: set accent in both files, verify user palette wins
10. Settings page allows palette customization with live preview
    - Test: change accent via Settings, verify live update and persistence
11. Config hot-reload picks up palette changes without restart
    - Test: edit palette in data-entry.yaml, verify frontend updates via SSE refresh

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Current CSS variables**: `App.vue:60-92` defines 14 variables for light/dark — these are the override targets
- **Settings page pattern**: `SettingsView.vue` + `handlers_api.go:702-859` — same pattern extends to palette
- **User defaults persistence**: `dataentryconfig/config.go:280-291` with `.rela/user-defaults.yaml` — palette uses same approach
- **Config serving**: `api_v1.go:109-119` `V1Config` — add resolved palette field
- **YAML union types**: `metamodel/types.go:229-293` HeaderCheck and InverseDef — pattern for `dark` field
- **Color derivation**: No external library needed — HSL manipulation via `math` is sufficient

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Color Model

Up to 8 named input colors (all optional, defaults fill gaps) → 14 CSS variables
per theme:

| Input Role | CSS Variable | Description |
|-----------|-------------|-------------|
| `base` | `--sidebar-bg` | Darkest surface (sidebar, nav) |
| `surface` | `--bg-color` | Main background |
| `accent` | `--accent-color` | Primary action/link color |
| `text` | `--text-color` | Main text color |
| `success` | `--success-color` | Success indicators |
| `error` | `--error-color` | Error indicators |
| `warning` | `--warning-color` | Warning indicators |
| `info` | `--info-color` | Info indicators |

6 auto-derived variables:

| Derived | From | Method | Clamping |
|---------|------|--------|----------|
| `--card-bg` | `surface` | Lighten ~2% | If L >= 0.98, equal surface |
| `--input-bg` | `surface` | Same as card-bg | Same |
| `--hover-bg` | `surface` | Darken ~3% | If L <= 0.03, equal surface |
| `--border-color` | `surface` + `text` | Mix at ~15% text | N/A |
| `--muted-text` | `text` | Lighten ~30% | Clamp L to [0,1] |
| `--sidebar-text` | `base` | High contrast | If base L < 0.5 → #e8e8e8, else #1e293b |

7 optional badge color overrides:

| Badge | CSS Variable | Default |
|-------|-------------|---------|
| `blue` | `--badge-blue` | `#3b82f6` |
| `purple` | `--badge-purple` | `#8b5cf6` |
| `green` | `--badge-green` | `#22c55e` |
| `gray` | `--badge-gray` | `#6b7280` |
| `red` | `--badge-red` | `#ef4444` |
| `orange` | `--badge-orange` | `#f97316` |
| `yellow` | `--badge-yellow` | `#eab308` |

Badge.vue will be refactored to use `var(--badge-blue)` etc. instead of
hard-coded hex.

### Dark Mode Generation

When `dark` is omitted or `dark: auto`:
- Invert surface lightness: light surfaces → dark, dark base → darker
- Lighten text colors for contrast on dark backgrounds
- Increase semantic color brightness for visibility
- Keep accent hue, shift lightness up slightly
- Badge colors: increase lightness slightly for dark bg visibility

When `dark: false`: disable dark mode entirely (hide toggle, force light).

When `dark:` is explicit palette object: use those 8 colors + optional badges
for dark mode.

### Config Format

```yaml
palette:
  # All fields optional — unset fields use built-in defaults
  base: "#1a1a2e"
  surface: "#f8fafc"
  accent: "#6366f1"
  text: "#1e293b"
  success: "#10b981"
  error: "#ef4444"
  warning: "#f59e0b"
  info: "#3b82f6"
  badges:
    blue: "#3b82f6"
    purple: "#8b5cf6"
    green: "#22c55e"
    gray: "#6b7280"
    red: "#ef4444"
    orange: "#f97316"
    yellow: "#eab308"
  # Dark mode: auto (default) | false | explicit object
  dark: auto
```

### Backend

1. **Config types** (`internal/dataentryconfig/config.go`):
   - `PaletteConfig` struct with 8 optional color fields + `Badges` map + `Dark` field
   - `Dark` field uses custom `UnmarshalYAML` (three-way: string "auto"/bool false/nested PaletteColors) — follows HeaderCheck/InverseDef pattern from metamodel/types.go
   - Add `Palette *PaletteConfig` to `Config` struct

2. **Color derivation** (`internal/dataentryconfig/palette.go`): New file.
   - Hex↔HSL conversion with clamping to [0,1]
   - `Derive(colors)` → produces the 6 derived CSS variables
   - `GenerateDark(lightColors)` → auto-generates dark variant
   - `Resolve(projectPalette, userPalette)` → merges and resolves full palette for both modes
   - `ValidatePalette(palette)` → hex regex validation on all provided colors

3. **User palette** (`internal/dataentryconfig/palette.go`):
   - Load from `.rela/palette.yaml`, save following `UserDefaults` pattern

4. **App integration** (`internal/dataentry/app.go`):
   - Call `Resolve()` during `NewApp()`, store resolved palette
   - Pass to frontend via config endpoint

5. **Hot-reload** (`internal/dataentry/watcher.go`):
   - Add palette resolution alongside `buildStyleMap()` in `onReload()`
   - Also reload user palette from disk

6. **API** (`internal/dataentry/api_v1.go`):
   - Add `Palette` field to `V1Config` (resolved light + dark, all 21 vars each)

7. **Settings API** (`internal/dataentry/handlers_api.go`):
   - Extend GET to include current user palette (8 colors + badges, not resolved)
   - Extend PUT to accept and validate user palette

### Frontend

8. **Badge.vue refactor** (`frontend/src/components/common/Badge.vue`):
   - Replace hard-coded hex with CSS custom properties (`var(--badge-blue)`, etc.)
   - App.vue gets new `:root` variables with current hex as defaults

9. **Schema store** (`frontend/src/stores/schema.ts`):
   - Receive resolved palette from config
   - Apply all CSS variables to `document.documentElement.style` on load

10. **UI store** (`frontend/src/stores/ui.ts`):
    - On theme toggle, swap palette variables (light↔dark)
    - If `dark: false` (darkDisabled flag), hide toggle and force light
    - Store `paletteLight` and `paletteDark` resolved maps

11. **Settings page** (`frontend/src/views/SettingsView.vue`):
    - "Appearance" section with color pickers for 8 roles + 7 badges
    - Dark mode control (auto/disabled/custom)
    - Live preview via CSS variable updates
    - Save via settings API

12. **Settings API** (`frontend/src/api/settings.ts`):
    - Add palette types and save/load functions

**Alternatives considered:**

- **All 14+7 colors explicit**: Rejected — tedious, doesn't work with Lospec-style palettes
- **Numbered array of colors**: Rejected — ambiguous role assignment
- **localStorage-only**: Rejected — not shareable, lost on new machine
- **Derive badge colors from semantic colors**: Rejected — user wants explicit control (option C)

**Files to modify/create:**

Backend:
- `internal/dataentryconfig/config.go` — add PaletteConfig to Config
- `internal/dataentryconfig/palette.go` — NEW: types, derivation, resolution, validation
- `internal/dataentryconfig/palette_test.go` — NEW: tests
- `internal/dataentry/app.go` — palette resolution in NewApp
- `internal/dataentry/watcher.go` — palette rebuild on reload
- `internal/dataentry/api_v1.go` — add Palette to V1Config
- `internal/dataentry/handlers_api.go` — extend settings for palette

Frontend:
- `frontend/src/App.vue` — add badge CSS custom properties to :root
- `frontend/src/components/common/Badge.vue` — use CSS variables
- `frontend/src/stores/schema.ts` — receive and apply palette
- `frontend/src/stores/ui.ts` — palette application on theme toggle, darkDisabled flag
- `frontend/src/views/SettingsView.vue` — appearance section
- `frontend/src/api/settings.ts` — palette types

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

1. **`data-entry.yaml` palette section** (trusted config): Validate hex format via `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`. All fields optional.
2. **Settings API palette input** (user HTTP): Same hex validation. Reject unknown fields.
3. **CSS injection risk**: Mitigated by hex-only allowlist. `setProperty()` with validated hex values is safe.
4. **Badge color names**: Allowlist of 7 known names only. Unknown badge names rejected.

**Security-Sensitive Operations:**

- File write to `.rela/palette.yaml` — follows existing `saveUserDefaults` pattern
- No auth/crypto operations

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Type |
|----|------|------|
| 1 | Parse palette with accent, verify CSS var populated | Unit |
| 2 | Partial palette (only accent), verify defaults for rest | Unit |
| 3 | Verify derived vars from surface/text with correct values | Unit |
| 4 | Set badges.blue, verify --badge-blue in resolved palette | Unit |
| 5 | Auto-generate dark, verify contrast/inversion | Unit |
| 6 | Set `dark: false`, verify darkDisabled flag, no dark palette | Unit |
| 7 | Explicit dark palette overrides auto | Unit |
| 8 | Invalid hex rejected with error | Unit |
| 9 | User palette merges over project palette | Unit |
| 10 | PUT palette via API, GET returns updated | Integration |
| 11 | Config change triggers palette rebuild in watcher | Unit |

**Edge Cases:**

- Empty palette section (no overrides — all defaults)
- Partial palette (only 1-2 colors set)
- 3-digit hex shorthand (`#f00`)
- 8-digit hex with alpha (`#ff000080`)
- Case insensitivity (`#FF0000` = `#ff0000`)
- Surface = `#ffffff` — card-bg/input-bg clamp to surface, hover-bg darkens slightly
- Base = `#000000` — sidebar-text forced to `#e8e8e8`
- Badges partially set (only blue + red, others use defaults)

**Negative Tests:**

- Non-hex value (`"red"`, `"rgb(255,0,0)"`) → error
- Empty string → error
- CSS injection (`"#000; background: url(evil)"`) → rejected
- Unknown badge name (`badges.teal`) → error
- `dark: "invalid"` → error (must be auto/false/object)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Auto-derived colors look bad for some palettes | Medium | Medium | Sensible clamping, allow explicit dark override |
| Auto-dark generation produces poor contrast | Medium | Medium | Test with variety; users can set explicit dark |
| Flash of default colors on load | Low | Low | Apply palette in schema store init before render |
| Badge.vue refactor breaks existing styling | Low | Medium | Visual regression test with default palette |

Effort: **L**

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide — document palette config format with examples
- [x] API docs — document palette field in config endpoint
- [ ] ~~CLI help text~~ (N/A)
- [ ] ~~CLAUDE.md~~ (N/A)
- [ ] ~~README.md~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| RR-CQ7U | significant | Badge colors hard-coded hex, not CSS vars | addressed — add CSS custom properties |
| RR-QGOU | significant | All 8 fields required too strict | addressed — all fields optional |
| RR-MCNU | significant | Scope contradicts badges discussion | addressed — scope updated |
| RR-AA1V | minor | Dark union type needs UnmarshalYAML | addressed — follow existing pattern |
| RR-VXHI | minor | HSL derivation clamping unspecified | addressed — clamping rules added |
| RR-NUT7 | minor | Config reload missing palette rebuild | addressed — added to watcher path |
