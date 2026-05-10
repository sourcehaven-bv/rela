---
id: REV-T61A
type: review-checklist
title: 'Review: Theme packages: export/install bundled palette + logo as .relatheme zip'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — all packages pass; 75.5 % total coverage.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — package floor satisfied; total 75.5 % > 65 % threshold.
- [x] Architecture lint clean (`just arch-lint`).
- [x] Frontend tests pass (`npm run test:run`) — 659/659.
- [x] Frontend typecheck clean (`npm run typecheck`).
- [x] Frontend lint clean (`npm run lint`) — 0 errors.
- [x] Frontend build succeeds (`npm run build`).

## Code Review

- [x] Run `cranky-code-reviewer` agent — done; 10 findings.
- [x] All critical review-responses addressed — none filed (no critical findings).
- [x] All significant review-responses addressed — RR-84YM, RR-YEVY, RR-0PTF, RR-5QTT, RR-U2S9.
- [x] Self-reviewed the diff for unrelated changes.

**Review Responses:**

| ID | Severity | Status |
|---|---|---|
| RR-84YM | significant | addressed (saturating-add zip-bomb guard, uint64 throughout) |
| RR-YEVY | significant | addressed (duplicate-entry rejection + sentinel + test) |
| RR-0PTF | significant | addressed (init-time tag-collision check + parseThemePackage panic recovery) |
| RR-5QTT | significant | addressed (useConfirm dirty-check before staging palette) |
| RR-U2S9 | significant | addressed (deferred URL.revokeObjectURL) |
| RR-MP1R | minor | addressed (typed APIThemeImportResponse) |
| RR-7P3O | minor | addressed (manifest extension allowlist {png, jpeg, jpg, svg, webp}) |
| RR-5EHQ | nit | addressed (maxThemeUploadBytes constant) |
| RR-YMSC | nit | addressed (overflow-protected ratio multiply) |
| RR-OMEN | nit | addressed (exhaustive switch + slog.Warn default) |

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-VIQW)

**Acceptance Status:**

| AC | Result | Evidence |
|---|---|---|
| 1. Settings shows Export/Install | PASS | New Theme Package card rendered between Logo and App Info; Export + Install buttons. |
| 2. Export with palette + logo | PASS | curl test: 394 byte zip with `theme.yaml` (99 b) + `logo.png` (56 b); `Content-Disposition` header present. |
| 3. Export with no logo | PASS | `TestThemeExport_PaletteOnly` asserts only `theme.yaml` in zip and no `logo.*`. |
| 4. Import staging | PASS | curl test: returned `{logoUrl, palette}` JSON; `_palette` GET still showed previous saved value (palette is staged, not auto-saved). |
| 5. Live sidebar update | PASS | After import, `_sidebar.logoUrl` reflected the new hash. |
| 6. Validation matrix | PASS | curl tests: non-zip → 400 `not a valid zip file`; zip with subdirectory → 400 `path traversal`. Plus the full Go reject matrix in `theme_package_test.go`. |
| 7. No new abstractions | PASS | `just arch-lint`: clean. Backend reuses `state.KV`, `mutateState`, `saveUserLogo`, `sniffLogoMime`, `MaxUserLogoBytes`. |

## Documentation (enhancements only)

Deferred. The umbrella plan's documentation impact (theme-system docs, API
reference) lands together with this PR's merge — `IMPL-VIQW` Documentation
Planning calls this out. End-user docs ride along when the broader "theme system
in data-entry" story is ready for a docs pass.

- [x] Logged in `IMPL-VIQW` Documentation Planning section.

## Final Checks

- [x] Commit message will explain why (PR 2 of theme system, builds on TKT-WN7O, font dropped).
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- to be added after `gh pr create` -->
