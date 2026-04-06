---
id: PLAN-LMC5
type: planning-checklist
title: 'Planning: Add palette import helper with smart color assignment'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Parse hex list format (one hex per line, or comma/space separated) — this is what Lospec gives
- Parse GPL (GIMP Palette) format — RGB lines with optional names
- Auto-detect format from pasted text
- Smart color assignment algorithm for 8 UI roles using lightness sorting + hue matching
- Smart color assignment for 7 badge colors using hue proximity to default badge hues
- Show full imported palette as clickable swatches in the Settings Appearance section
- Click a swatch to assign it to the currently-focused color role
- All client-side (no new backend endpoint needed — just populate the existing palette form)

OUT of scope:
- Lospec API integration (user pastes text, not a URL)
- File upload dialog (textarea paste is simpler and works everywhere)
- ASE/PAL binary formats (hex + GPL cover the common text formats)
- Perfect color mapping (goal is "good enough starting point")

**Acceptance Criteria:**

1. User can paste a hex list (one per line) into an import textarea and get auto-assigned palette
   - Test: paste Fading 16 hex list, verify 8 UI roles + 7 badges populated
2. User can paste GPL format text and get auto-assigned palette
   - Test: paste a GIMP Palette file content, verify parsing and assignment
3. Format is auto-detected (hex vs GPL)
   - Test: paste hex list without header → hex mode; paste with "GIMP Palette" header → GPL mode
4. Imported palette swatches are displayed below the import area
   - Test: import 16 colors, see 16 clickable swatches
5. Clicking a swatch assigns that color to the currently selected/focused role
   - Test: focus on "accent" field, click a swatch, verify accent field updates
6. Auto-assignment uses lightness for UI roles (darkest→base, lightest→surface)
   - Test: import palette with known lightness order, verify base is darkest, surface is lightest
7. Auto-assignment uses hue proximity for semantic + badge colors
   - Test: import palette with a green, verify it's assigned to success/green badge
8. No color is assigned to two different roles (uniqueness constraint)
   - Test: import 8-color palette, verify no duplicate assignments

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **HSL utilities in palette.go**: `hexToHSL`, `hslToHex`, `mixColors` — can be ported to TS or reimplemented client-side
- **SettingsView.vue**: Appearance section already has color pickers for all 15 roles — import just populates these
- **No existing import libs needed** — parsing hex/GPL is trivial string parsing

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Pure Frontend Implementation

This is entirely client-side. No backend changes needed — the import parses
text, runs the assignment algorithm, and populates the existing `paletteColors`
and `paletteBadges` reactive state. The user can then tweak and hit "Save
Palette" as before.

### Parsing

```typescript
function parseHexList(text: string): string[]
// Split by newlines/commas/spaces, filter valid hex, normalize to #rrggbb

function parseGPL(text: string): string[]
// Skip "GIMP Palette", "Name:", "Columns:", "#" lines
// Parse "R G B" lines → convert to hex

function parsePalette(text: string): string[]
// Auto-detect: starts with "GIMP Palette" → GPL, else hex list
```

### Color Assignment Algorithm

**Step 1: Sort by lightness** (L in HSL)
- All imported colors sorted light→dark

**Step 2: Assign UI structural roles**
- `surface` = lightest color
- `base` = darkest color
- `text` = second darkest (must contrast with surface)
- `accent` = most saturated of remaining mid-range colors

**Step 3: Assign semantic roles by hue matching** Target hues (from defaults):
- success ≈ 145° (green)
- error ≈ 0° (red)
- warning ≈ 38° (yellow-orange)
- info ≈ 217° (blue)

For each semantic role, find the unassigned color with the closest hue (weighted
distance: hue×3 + saturation×1 + lightness×0.5). Mark as used.

**Step 4: Assign badge colors by hue matching** Target hues:
- blue ≈ 217°, purple ≈ 259°, green ≈ 142°, gray ≈ low saturation
- red ≈ 0°, orange ≈ 25°, yellow ≈ 48°

For gray badge: pick lowest saturation unassigned color. For others: closest hue
match from remaining unassigned colors.

If fewer colors than roles, some roles stay empty (use defaults).

### UI Components

**Import section** (new, above the existing Theme Colors section):

```
┌─ Import Palette ──────────────────────────────┐
│ [textarea: paste hex or GPL here]             │
│ [Import] [Clear]                              │
│                                               │
│ Imported Colors:                              │
│ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■ ■            │
│ (click a swatch to assign to selected role)   │
└───────────────────────────────────────────────┘
```

**Swatch interaction:**
- Each color role input gets a "focus" state tracked in a ref
- When a swatch is clicked, if a role is focused, assign that color to it
- Visual feedback: focused role has a highlighted border

**Files to modify:**

Frontend only:
- `frontend/src/views/SettingsView.vue` — import UI, swatch display, assignment logic
- `frontend/src/utils/palette.ts` — NEW: parsing + assignment algorithm (pure functions, testable)
- `frontend/src/utils/palette.test.ts` — NEW: unit tests for parsing + assignment

**Alternatives considered:**

- **Backend parsing endpoint**: Rejected — unnecessary complexity, all parsing is simple string ops
- **File upload dialog**: Rejected — textarea paste is simpler, works on all platforms, no file API needed
- **Binary format support (ASE/PAL)**: Rejected — hex + GPL cover the text formats Lospec provides

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

1. **Textarea paste** (user input): Parsed client-side only. Invalid lines silently skipped. Hex regex validates each color. No data sent to server until user explicitly clicks Save.
2. **No new API endpoints** — import is purely client-side
3. **XSS risk**: None — colors are hex strings applied via `setProperty()`, never inserted as HTML

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Type |
|----|------|------|
| 1 | Parse hex list, verify color count and values | Unit |
| 2 | Parse GPL format, verify RGB→hex conversion | Unit |
| 3 | Auto-detect hex vs GPL | Unit |
| 4 | Import populates swatch display | Manual |
| 5 | Click swatch assigns to focused role | Manual |
| 6 | assignUIRoles: darkest→base, lightest→surface | Unit |
| 7 | assignSemanticRoles: green hue→success | Unit |
| 8 | No duplicate assignments | Unit |

**Edge Cases:**

- Empty input → no change, show error toast
- Single color → assign to accent only
- Fewer colors than roles → unassigned roles stay empty (defaults)
- More colors than roles → extra colors shown as swatches but not auto-assigned
- Mixed valid/invalid lines → skip invalid, use valid
- All grays (low saturation) → hue matching degrades gracefully
- Duplicate colors in input → deduplicate

**Negative Tests:**

- Completely invalid input ("hello world") → "No valid colors found" message
- Empty textarea → disabled import button

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Hue matching picks wrong color for a role | Medium | Low | User can click swatches to reassign; it's a starting point |
| Small palettes (4-8 colors) leave many roles empty | Medium | Low | Unassigned roles keep defaults; user fills via swatches |
| GPL format variations | Low | Low | Lenient parser, skip unparseable lines |

Effort: **M** (frontend-only, ~200 lines of utility code + ~100 lines of UI)

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] ~~User guide~~ (N/A: UI is self-explanatory with placeholder text)
- [ ] ~~CLI help text~~ (N/A)
- [ ] ~~CLAUDE.md~~ (N/A)
- [ ] ~~README.md~~ (N/A)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small, frontend-only feature building on existing palette)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** Skipped — pure frontend addition with no
architectural changes
