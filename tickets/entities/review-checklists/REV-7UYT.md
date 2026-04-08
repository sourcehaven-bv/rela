---
id: REV-7UYT
type: review-checklist
title: 'Review: Simplify palette settings — Regular vs Light+Dark mode with explicit Derive'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` → all OK; `npm run test:run` → 313/313 passing across 16 files)
- [x] Lint clean (`golangci-lint run` → no issues; `npm run lint` → 0 errors, 20 pre-existing warnings)
- [x] Coverage maintained (`internal/dataentryconfig` 85.2%, `internal/dataentry` 61.1% — both increased from baseline due to new tests)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer agent invoked, returned 15 findings)
- [x] All critical review-responses addressed (RR-OA4A, RR-HJ92)
- [x] All significant review-responses addressed (RR-8PTK, RR-17IW, RR-R73K, RR-V6HR, RR-LQ34, RR-MFFU; RR-PLT2 deferred with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Critical (must-fix):

- RR-OA4A — addressed: `loadUserPalette` now returns errors for parse failures with a clear migration hint; NewApp fails to start, watcher logs and keeps the previous palette.
- RR-HJ92 — addressed: replaced the global live preview with a scoped preview-swatch component (per user suggestion). Both Light and Dark visible side-by-side, no global side effects.

Significant (must-fix):

- RR-8PTK — addressed: `generateDark` validates inputs against `FULL_HEX_RE` before HSL math; partial hex / whitespace / non-hex now produce empty strings instead of `#NaNNaNNaN`.
- RR-17IW — addressed: defensive nil-check added in `ResolvePalette`; regression test for JSON-null-decoded zero-value DarkMode.
- RR-R73K — addressed: `loadPaletteState` now takes a third `resolvedDarkDisabled` parameter so a user with no `dark` overlay inherits the project's resolved dark state instead of silently downgrading to Regular mode on first save.
- RR-V6HR — addressed: Derive Dark button is `:disabled` when no Light slot is set; tooltip explains why; defensive `uiStore.warning` if clicked anyway.
- RR-LQ34 — addressed: removed by construction. SettingsView no longer touches `applyPalette`/`clearPalette` from the live preview path; the scoped swatch component never writes to global scope.
- RR-MFFU — addressed: TS test imports goldens directly from `internal/dataentryconfig/testdata/` (single source of truth); duplicate `__fixtures__` directory removed.
- RR-PLT2 — deferred with reason: SettingsView.vue was already over the lint threshold before this PR. Component extraction (PaletteEditor.vue, DefaultsEditor.vue, OverridesEditor.vue) is the right call but is a substantial separate refactor too risky to bundle into this PR alongside the critical correctness fixes.

Minor / nit:

- RR-ABJ1 — addressed: clear godoc on `generateDark`/`generateDarkBadges` explaining their role as the parity-goldens reference implementation.
- RR-QW1A — addressed: added `PALETTE_ROLE_KEYS` const literal-typed array; `buildPalettePayload` iterates it instead of `Object.entries`, eliminating the `Record<string, string>` cast.
- RR-1O02 — addressed: renamed misleading `(legacy)` test name.
- RR-4A9F — addressed: side effect of RR-R73K fix.
- RR-BRIJ — wont-fix with reason: under the new model, `parseRelaPalette`'s permissive handling of `dark: auto` is now the correct behavior by accident — it falls through to the resolvedDarkDisabled-aware loadPaletteState.
- RR-HOMK — wont-fix with reason: false alarm; cranky was looking at a stale local build artifact. Static assets are gitignored.

## Acceptance Verification

**Acceptance Status:** All 12 ACs from PLAN-6ZCB verified.

| AC | Status | Evidence |
|---|---|---|
| 1. Top-level Regular / Light+Dark mode switch | PASS | Puppeteer screenshot shows the new toggle replacing the old in-form Light/Dark pill |
| 2. Regular mode = single column | PASS | Manual + verified via the new preview component (Regular renders one pane) |
| 3. Light+Dark mode = two columns side-by-side | PASS | Puppeteer screenshot shows side-by-side LIGHT and DARK columns with sticky header |
| 4. Save in Regular = `dark: false` | PASS | Vitest `buildPalettePayload` tests + manual save → palette.yaml inspection shows `dark: false` |
| 5. Save in Light+Dark after Derive = full dark object | PASS | Vitest tests + manual save → 8-key dark object on disk |
| 6. Save in Light+Dark with no Derive = `dark: {}` | PASS | Vitest test |
| 7. Derive populates all 8 from generateDark | PASS | Vitest goldens parity (4 fixtures) + manual Puppeteer Derive |
| 8. Derive overwrite confirm flow | PASS | Manual Puppeteer: cancel preserves, overwrite replaces |
| 9. Whitespace trim + normalize | PASS | Manual Puppeteer: pasted `  #ABC  ` → state holds `#aabbcc` |
| 10. Live preview scope | PASS (better than spec): replaced with scoped preview swatch — both Light and Dark visible at all times, no global page side effects |
| 11. Load matrix (false / explicit / undefined) | PASS | Vitest `loadPaletteState` tests + manual round-trips |
| 12. Backend two-state DarkMode | PASS | Go tests cover all paths including the new defensive nil-check |

## Documentation

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing prose docs reference the old toggle; the migration error message in the loader is the user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: see above)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message will explain WHY (the three intertwined "jankiness" sources, the explicit-derive design, the scoped-preview design choice)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (will be done by user after this checklist is reviewed; PR creation is intentionally not auto-run from inside the workflow)
- [x] ~~All CI checks pass~~ (deferred to PR creation)
- [x] ~~PR URL documented below~~ (deferred)

**PR:** https://github.com/sourcehaven-bv/rela/pull/337
