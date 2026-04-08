---
id: PLAN-6ZCB
type: planning-checklist
title: 'Planning: Replace palette Light/Dark editing toggle with Regular vs Light+Dark mode switch'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Context — manual testing findings**

Drove the current Settings → Appearance section through Puppeteer with a
Lospec palette and a rela palette.yaml. The current UX is "janky" because
of several compounding issues:

1. **Backend bug** (`internal/dataentryconfig/palette.go:306-315`): when
   the user saves a partial `dark: { accent: "#xxx" }` override, the
   "explicit" branch in `ResolvePalette` fills the unset dark fields with
   `defaultLightColors` instead of `generateDark(light)`. So setting one
   dark override resets every other dark color (sidebar, surface, badges)
   to the framework defaults. Confirmed via `/api/v1/_config`.
2. **Live preview writes to the wrong CSS scope**: editing a Dark color
   pushes the dark hex into the same `:root` CSS variables used for
   light, so the visible page reverts to "light" mid-edit even when
   `html.dark` is set. Editing Light while the global toggle is dark
   does the inverse. The whole live-preview-while-editing-dark flow is
   broken.
3. **Auto-derivation is invisible**: dark inputs show placeholders that
   are visually indistinguishable from real values, and the
   `<input type="color">` swatch shows `#808080` even when the
   placeholder is a real auto-derived color. The user can never tell
   "what is mine" vs "what is derived".
4. **Whitespace fussy** (`SettingsView.vue:86`): `setColor` /
   `setBadge` write the raw `e.target.value` straight into state with
   no `.trim()`. Pasting `  #ffcd75 ` into a per-role input gets
   rejected by the backend `hexColorRe` (`^#...$`).
5. **No way to disable dark mode** from the UI even though backend
   `DarkMode.IsDisabled()` (`dark: false`) supports it.

**Design direction (decided with user)**

Make the data model **simple and explicit**: there is no "auto" mode at
runtime. The palette is either Light-only (`dark: false`) or has a
fully-explicit dark palette (`dark: { all 8 colors }`). Auto-derivation
is a one-shot user action ("Derive Dark from Light" button) that fills
in the dark column from the current light values. After clicking
Derive, the user can hand-tweak any individual dark color and the
others stay put. No silent merging, no hidden state.

This collapses the current three-state `DarkMode` (`auto` / `false` /
`Explicit`) into a two-state union (`false` / `Explicit`).

**Scope:**

IN scope:

- **Backend** (`internal/dataentryconfig/palette.go`):
  - Drop the `auto` mode from `DarkMode`. `UnmarshalYAML` /
    `UnmarshalJSON` accept only `false` or an object. Empty / missing
    `dark` field means "not yet configured" and resolves the same way
    as `dark: false` (no dark theme rendered).
  - `ResolvePalette`: `dark: false` → `DarkDisabled = true`. Object →
    use as-is, do not merge with anything. (UI guarantees all 8 fields
    are set when an object is sent.)
  - Delete `generateDark`, `generateDarkBadges`, and the `darkBaseDelta`
    / `darkSurfaceTarget` / `darkTextTarget` / `darkBrightenDelta`
    constants. The "auto-derivation" lives only in the frontend now,
    invoked by the Derive button.
  - Update `palette_test.go` for the new two-state semantics.
- **Frontend utility** (`frontend/src/utils/palette.ts`):
  - Add `generateDark(light: PaletteColors): PaletteColors` ported from
    the Go version (HSL adjust/invert with the same constants).
  - Add `generateDarkBadges(badges: Record<string, string>):
    Record<string, string>` similarly.
  - Vitest goldens locked against the current Go output before deletion
    to confirm parity.
- **Frontend Settings UI** (`frontend/src/views/SettingsView.vue`):
  - Replace `editingDark: Ref<boolean>` with
    `paletteMode: Ref<'regular' | 'light-dark'>`.
  - Top-of-section toggle: **Regular** | **Light + Dark** (reuses
    `.toggle-pill` styles).
  - In **Regular** mode: render exactly today's single-column layout
    for the 8 roles + 7 badges. Save sends `palette.dark = false`.
  - In **Light + Dark** mode: render two columns side-by-side per row.
    Each row has `Light` input on the left, `Dark` input on the right.
    Above the grid, a sticky two-column header shows `Light` | `Dark`
    so the layout is self-explaining at any scroll position.
  - **Derive Dark from Light** button at the top of the Dark column.
    Clicking it calls `generateDark(paletteColors)` and
    `generateDarkBadges(paletteBadges)` and writes the result into
    `paletteDarkColors` / `paletteDarkBadges`. If any dark slot is
    already non-empty, show an inline confirm ("Overwrite all dark
    colors?") before applying. Always overwrites all 8 + all badges
    when confirmed — no per-slot merging.
  - **Whitespace fix** in `setColor` / `setBadge`: trim the value;
    if it matches `HEX_RE`, store the normalized form; otherwise store
    the trimmed raw (so the user can keep typing partial hex).
  - **Live preview rewrite**: only apply the *active* mode's palette to
    the visible CSS variables. In Regular mode the page is always
    rendered as light regardless of `uiStore.darkMode`. In Light+Dark
    mode the page reflects `uiStore.darkMode` — editing the Light
    column while the global toggle is dark must NOT clobber the dark
    rendering. Implementation: pick the source palette per the
    *current* `uiStore.darkMode` and `paletteMode` and pass that to
    `uiStore.applyPalette`.
  - Drop now-dead helpers `autoDarkColor` / `autoDarkBadge` /
    `activeColors` / `activeBadges` (no longer needed because both
    columns are bound directly to their own refs in light+dark mode).
- **Frontend types** (`frontend/src/api/settings.ts`):
  - Tighten `PaletteConfig.dark?: PaletteColors | false` (drop
    `'auto'` from the union).

OUT of scope (file as separate bugs after this lands):

- Backend serves stale palette after `.rela/palette.yaml` is deleted
  from disk (file watcher doesn't catch deletions).
- Bulk-import textarea doesn't accept rela palette.yaml format (only
  the `Browse File` path does — discoverability issue).
- "Reset" button scrolls page to top.
- Color contrast checks / accessibility validation on the derived
  palette.

**Acceptance Criteria:**

1. The Appearance section shows a top-level **Regular** / **Light +
   Dark** mode switch.
   - Test: Vitest mount, assert two `.toggle-pill` buttons with those
     labels exist.
2. In **Regular** mode, exactly one column of color inputs is rendered
   per role and per badge.
   - Test: set mode=regular, count `.color-input-group` per `.color-row`
     → 1.
3. In **Light + Dark** mode, two columns are rendered per row (`Light`
   / `Dark`), with a sticky two-column header.
   - Test: set mode=light-dark, count `.color-input-group` per row →
     2; assert `.column-header` element with `Light` and `Dark` text.
4. Saving in **Regular** mode writes `palette.dark = false` to the API.
   - Test: Vitest — fill light values, set mode=regular, click Save,
     assert payload `palette.dark === false`.
5. Saving in **Light + Dark** mode after Derive writes a fully-populated
   `palette.dark` object (all 8 base colors).
   - Test: fill light, click Derive, click Save, assert
     `Object.keys(payload.palette.dark)` includes all 8 role keys.
6. Saving in **Light + Dark** mode with no Derive click and no manual
   dark values writes `palette.dark = {}` (empty object). The backend
   then renders dark mode with the unset-color fallbacks (currently
   `defaultLightColors` — acceptable because the UI promotes Derive
   for any user who actually wants dark).
   - Test: assert payload.palette.dark === {} (or omitted).
7. Clicking **Derive Dark from Light** with no existing dark values
   populates `paletteDarkColors` with the output of `generateDark(...)`
   for all 8 roles, and `paletteDarkBadges` for all 7 badges.
   - Test: Vitest — click derive, assert each `paletteDarkColors[k]`
     is non-empty and equals `generateDark(paletteColors)[k]`.
8. Clicking **Derive Dark from Light** with at least one existing dark
   value shows an inline confirm; clicking Confirm overwrites all 8 +
   7 badges; clicking Cancel leaves the existing values intact.
   - Test: pre-set `paletteDarkColors.accent = '#abc'`, click derive,
     assert confirm appears, click cancel, assert accent unchanged;
     repeat with confirm, assert accent overwritten.
9. Pasting `  #abc  ` into a per-role text input stores `#aabbcc`
   (trimmed and normalized) in state, and the value survives a Save
   round-trip without a backend validation error.
   - Test: Vitest — fire input event with `'  #abc  '`, assert state
     value === `'#aabbcc'`.
10. Live preview honors mode + global toggle: in Light+Dark mode with
    `uiStore.darkMode === true`, editing the Dark column updates the
    visible CSS vars; editing the Light column does NOT change them.
    In Regular mode the page renders as light regardless of
    `uiStore.darkMode`.
    - Test: Manual Puppeteer verification (the watch logic has too
      many DOM dependencies for clean unit testing).
11. Loading an existing palette with `dark: false` selects Regular
    mode; loading with an explicit `dark: { ... }` object selects
    Light+Dark mode and pre-fills the dark inputs.
    - Test: Vitest — mock `getSettings` for both shapes, mount, assert
      `paletteMode` resolves correctly.
12. Backend `ResolvePalette` no longer references `defaultLightColors`
    or `defaultBadgeColors` from the dark resolution path. `dark: {}`
    or `dark: false` are the only two states the resolver handles for
    the dark side; old `auto` mode is removed.
    - Test: Go unit test asserting `DarkMode` only accepts `false` or
      object; `auto` string returns an error.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The HSL color math is already in
  `internal/dataentryconfig/palette.go:443-600` (`hexToHSL`,
  `hslToHex`, `adjustLightness`, `invertLightness`). The TypeScript
  port mirrors that exact algorithm so the Derive button produces
  byte-identical output to today's auto-derivation. Vitest goldens
  pinned before deleting the Go version guarantee parity.
- Frontend already has `deriveTheme` in `frontend/src/utils/palette.ts`
  for the 6 computed CSS variables (border, hover, etc.) — that stays
  untouched; only the 8 base-color dark generation moves to TS.
- Three-state union elimination follows the same pattern as the
  `editingDark` removal — both are state-explosion sources where the
  user only ever cared about two of the three states.
- Prior tickets: TKT-K8UQ added the palette feature, TKT-EPB5 added
  smart import, TKT-ULCZ added file/drag/dark editing. The current
  ticket consolidates and simplifies all three.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Step 1 — Lock parity goldens. Before any deletions, write a Go test
that runs `generateDark` against ~6 representative input palettes (the
8 default colors + a few Lospec imports) and dumps the output as JSON
goldens to a fixtures file. This is the contract the TS port must
match.

Step 2 — TS port. Add `generateDark` and `generateDarkBadges` to
`frontend/src/utils/palette.ts`. Port `hexToHSL`, `hslToHex`,
`adjustLightness`, `invertLightness` (the existing TS file already has
`hexToHsl` for `deriveTheme`, but check if it's the same algorithm or
needs to be replaced). Vitest spec consumes the same JSON goldens
generated by the Go test (write a tiny script or copy them in by hand).

Step 3 — Backend simplification. Update `DarkMode` to a two-state
union. Update `ResolvePalette` accordingly. Delete dead code. Update
Go palette tests.

Step 4 — Frontend `PaletteConfig` type narrowing.

Step 5 — Settings UI rewrite. New `paletteMode` ref, two-button mode
toggle, conditional template for single-column vs side-by-side
layout, Derive button with confirm-on-overwrite, whitespace
trim/normalize in setters, live preview scope fix.

Step 6 — Tests. Vitest spec for SettingsView covering load/save
matrix, Derive button behavior, whitespace handling, mode toggle.

Step 7 — Manual Puppeteer pass to verify the live preview fix and the
overall feel of the new flow.

**Files to modify:**

- `internal/dataentryconfig/palette.go` (delete + simplify)
- `internal/dataentryconfig/palette_test.go` (update for new
  semantics)
- `internal/dataentryconfig/palette_parity_test.go` (NEW — pin
  goldens before deleting `generateDark`)
- `frontend/src/utils/palette.ts` (add `generateDark`,
  `generateDarkBadges`, possibly `hexToHsl`/`hslToHex` if not already
  there in the right form)
- `frontend/src/utils/palette.test.ts` (add Derive parity tests vs Go
  goldens)
- `frontend/src/api/settings.ts` (narrow `dark` type to
  `PaletteColors | false`)
- `frontend/src/views/SettingsView.vue` (the bulk of the work)
- `frontend/src/views/SettingsView.palette.test.ts` (NEW — Vitest
  spec)

**Alternatives considered:**

- *Backend Derive endpoint instead of TS port*: rejected. The math is
  ~50 lines, easy to keep in sync via goldens, and a network round
  trip per click degrades the feel of the UI. Single-source-of-truth
  argument is real but the goldens give us strong parity guarantees.
- *Auto-call Derive on first toggle into Light+Dark mode*: rejected
  per user feedback — "rather give the user a bit more control than
  accidentally doing the wrong thing." The Light+Dark column starts
  empty; user clicks Derive when they're ready.
- *Per-role Derive button (one per row)*: too noisy. The 8 colors are
  derived from the same base palette anyway; one button is enough.
- *Keep `auto` mode for backward compat with existing palette.yaml
  files*: rejected — user said "no need for legacy stuff." If anyone
  has an existing `dark: auto` in their palette.yaml, the YAML
  decoder will return an error and they'll need to either delete the
  field or run the UI to set it explicitly. Acceptable migration cost
  for a project still in active development.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over
  blocklist)
- [x] Security-sensitive operations identified (file access, auth,
  crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- User-entered hex colors — same as today, validated by `hexColorRe`
  on the backend at save time. The new whitespace trim in the
  frontend setter is purely a UX layer; the backend remains the
  authoritative validator.
- The mode discriminator is frontend-only; on save it translates to
  `palette.dark = false` or an object. Backend validates the object
  against `validateColors` regardless.
- Derive button output is byte-output of the existing algorithm, no
  user input involved beyond the already-validated light palette.

**Security-Sensitive Operations:**

- None new. Palette save still goes through `/api/v1/_palette` which
  is behind the existing CSRF + localhost binding from TKT-COJF.
- Removing the `auto` branch closes a small attack surface: today an
  attacker who could get a malformed `dark` field through the YAML
  parser had three code paths to probe; after this change there are
  two.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1  | Vitest: mount SettingsView, assert two mode buttons present. |
| 2  | Vitest: set mode=regular, count `.color-input-group` per row → 1. |
| 3  | Vitest: set mode=light-dark, count → 2; assert column header. |
| 4  | Vitest: build save payload in regular mode, assert `dark === false`. |
| 5  | Vitest: light+derive+save, assert dark object has all 8 keys. |
| 6  | Vitest: light+no-derive+save, assert `dark === {}`. |
| 7  | Vitest: click derive, assert `paletteDarkColors` matches `generateDark` output for all 8. |
| 8  | Vitest: pre-set one dark value, click derive → confirm appears; cancel → unchanged; confirm → overwritten. |
| 9  | Vitest: fire input with `'  #abc  '`, assert state === `'#aabbcc'`. |
| 10 | Manual Puppeteer: verify live preview scope fix (light vs dark editing under both global toggle states). |
| 11 | Vitest: mock `getSettings` for `dark: false` and explicit object, assert mode resolves correctly. |
| 12 | Go test: `DarkMode.UnmarshalYAML('auto')` returns error; `false` and object work; `ResolvePalette` paths covered. |

**Edge Cases:**

- User clicks Derive in Light+Dark mode but the light palette is
  partially empty (e.g. only `accent` set): `generateDark` should
  still produce reasonable output for the empty fields (current Go
  behavior: `adjustLightness('')` → invalid; need to skip empty
  inputs in the port and only fill dark slots whose corresponding
  light slot is set).
- User toggles Regular → Light+Dark → Regular within a session: the
  in-memory `paletteDarkColors` is preserved across the toggle (no
  destructive side effect on toggle), but only persisted on Save.
- User loads an old `dark: auto` palette.yaml: backend YAML decoder
  returns an error; the API surfaces it and the Settings page shows
  the error. User has to manually edit the file or use the UI to
  override. Document this in the migration note (added to ticket
  body).
- Whitespace edge cases: `'  '` (only spaces) → store empty string
  (treat as cleared); `'#abc  '` → `'#aabbcc'`; `'  #1234567890'`
  (too long) → store the trimmed raw, validation fails on save.

**Negative Tests:**

- Invalid hex on save → backend returns error → frontend shows toast
  via `uiStore.error` (existing behavior).
- `DarkMode.UnmarshalJSON("auto")` returns error.
- `ResolvePalette` with `dark: { base: "" }` (empty single field):
  the dark base falls through to the existing fallback path. Document
  what that fallback is in the test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Goldens parity drift*: the TS port might not exactly match the Go
  version due to floating-point rounding differences between Go's
  `math.Round` and JS's `Math.round`. Mitigation: lock Go output to
  JSON, run TS against the exact same inputs, allow ≤1 in last place
  per channel as the test tolerance.
- *Backward incompatibility*: existing `palette.yaml` files with
  `dark: auto` break. Mitigation: error message tells the user what
  to do; document in the ticket body. Project is still in active
  development with very few external users.
- *Visual regression*: substantial layout change in Light+Dark mode
  (single column → two-column grid). Mitigation: Regular mode keeps
  the existing single-column markup byte-identical; only the
  Light+Dark branch is new. Manual screenshot verification.
- *Live preview rewrite is the highest-risk part*: easy to introduce
  subtle bugs around which CSS scope to write to. Mitigation:
  manual Puppeteer testing of all four cases (regular+light_global,
  regular+dark_global, ldmode+light_global, ldmode+dark_global) at
  the end.
- Effort: **s** (small Vue rewrite + small Go cleanup; the bulk of
  the diff is deletions).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: no user-facing docs reference the old toggle)
- [x] ~~CLI help text~~ (N/A: no CLI surface affected)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns introduced)
- [x] ~~README.md~~ (N/A: project-level concepts unchanged)
- [x] ~~API docs~~ (N/A: API contract narrowed (`PaletteColors | false`); the
  surface is internal between rela-server and the SPA)
- [x] Migration note: existing `palette.yaml` files with `dark: auto`
  will fail to load. The error message in `loadUserPalette` includes
  a clear hint pointing users to either delete the `dark` line or
  set it to `false` / an explicit object. No prose docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A:
  iterated extensively with user during planning, including manual
  Puppeteer testing that drove the rescope. Design is locked.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — design iterated live with user.
