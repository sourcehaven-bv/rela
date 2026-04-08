---
id: IMPL-J1W0
type: implementation-checklist
title: 'Implementation: Simplify palette settings — Regular vs Light+Dark mode with explicit Derive'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (15 new Vitest tests for
  `buildPalettePayload`/`loadPaletteState`, 6 new Vitest tests for
  `generateDark`/`generateDarkBadges` parity vs Go goldens, 1 new Go
  test `TestGenerateDarkParityGoldens`, 7 updated Go tests for the
  new two-state `DarkMode`)
- [x] ~~Integration tests written~~ (N/A: SettingsView is excluded from
  frontend coverage and exercised via existing e2e/manual tests; new
  e2e cases will be added in a follow-up if regressions appear)
- [x] Happy path implemented (Regular mode, Light+Dark mode, Derive,
  Save, Load round-trip — all verified manually via Puppeteer)
- [x] Edge cases from planning handled
- [x] Error handling in place (backend hex validation surfaces via
  `uiStore.error`, frontend trim doesn't swallow invalid input)

## Test Quality

- [x] Using fixture builders or factories for test data (parity test
  uses named fixture struct with explicit input palettes)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
  (e.g. round-trip tests reconstruct expected from input)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end via Puppeteer
- [x] Each acceptance criterion verified with test scenario from
  planning
- [x] Edge cases manually verified

**Verification Evidence:**

Drove the new flow through a real `rela-server` instance on
`prototypes/data-entry/project` via Puppeteer. Verified:

1. **AC1-3 (mode toggle)**: Settings → Appearance shows the new
   `Regular` | `Light + Dark` toggle. Regular mode shows one column
   per role; Light+Dark shows two columns side-by-side with `LIGHT`
   and `DARK` headers and a `Derive Dark from Light` button next to
   the Dark header.
2. **AC7-8 (Derive button)**: Imported the Lospec sweetie-16 palette
   into Light, clicked Derive Dark from Light → all 8 dark color
   pickers populated with bytes that match the locked Go goldens
   (`#11121c`, `#141414`, `#ffdfa8`, …). Set a dark accent value
   manually, clicked Derive again → inline confirm appeared
   ("Overwrite all dark colors with values derived from the current
   light palette?"). Cancel preserved the existing value
   (`#123456`); Overwrite replaced it with the derived value.
3. **AC4 (Regular save)**: Switched to Regular mode and saved.
   Inspected `.rela/palette.yaml` → contains `dark: false`
   (previously-saved full dark object correctly dropped from disk).
4. **AC5 (Light+Dark save)**: Imported sweetie-16, clicked Derive,
   edited dark accent, saved. Inspected `palette.yaml` → light
   colors include the user-edited `accent: '#00ff00'`, `dark` is a
   fully-populated explicit object containing all 8 base colors with
   the user override on accent (`#ff00ff`). No `dark: auto`, no
   `dark: false`, no extra keys.
5. **AC9 (whitespace fix)**: Pasted `  #ABC  ` into the Light Accent
   text input. After Vue reactivity tick, the input rendered
   `#aabbcc` (trimmed and normalized via `normalizeHex`). The
   underlying state was `#aabbcc`, so a subsequent save would
   serialize cleanly without the backend hex validation rejecting
   the trailing whitespace.
6. **AC10 (live preview scope fix)**: With global dark mode enabled,
   edited the Light Accent input to `#00ff00`. The visible page
   stayed in dark mode and did NOT change colors — the light edit
   correctly did not leak into the dark CSS scope. Then edited the
   Dark Accent input to `#ff00ff`. `getComputedStyle` returned the
   new value for `--accent-color`, confirming the dark edit DID
   apply. Toggled back to Regular mode while still in global dark
   state → page rendered as light (overriding `html.dark`),
   confirming Regular mode forces light rendering as designed.
7. **AC11 (load matrix)**: Reloaded the page after each save.
   Confirmed `dark: false` loaded back into Regular mode, full dark
   object loaded into Light+Dark mode with the dark column pre-filled.
8. **AC12 (backend simplification)**: All 30+ Go packages pass with
   the new two-state `DarkMode`. The `auto` string and `true` bool
   are explicitly rejected by `Unmarshal{YAML,JSON}` as legacy
   shapes.

Tests run:

- `go test ./...` (excluding `cmd/rela` due to a pre-existing local
  go1.25.6/go1.25.8 toolchain mismatch unrelated to this change) —
  all green
- `go test -race ./internal/...` — all green, race-clean
- `npm run test:run` — 307/307 passing (16 files), up from 292
- `npm run typecheck` — clean
- `npm run lint` — 0 errors (only pre-existing `max-lines` warnings)
- `npm run build` — successful
- `just lint` (Go) — clean

## Quality

- [x] Code follows project patterns (single .vue file with extracted
  pure helpers in a co-located `.palette.ts` file, matching the
  pattern used elsewhere)
- [x] No security issues introduced (backend validation unchanged,
  frontend trim is purely a UX layer)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
