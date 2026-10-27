---
id: REV-UZIMGM
type: review-checklist
title: 'Review: Palette: UI kit shades, teal, distinct orange/yellow and operator colours'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (exit 0; frontend `npm run test:run` 333 files / 4948 tests pass; rela-components `npm run check` passes)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues on changed packages after US-spelling fix; frontend lint 0 errors)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links; arch-lint OK; plimsoll OK)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** No new advisory findings were introduced on purpose; nil
and parameter contracts in new code are typed (ParseColor takes a nil-able map,
documented).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7W7TM5, RR-7J0JQ1, RR-YZWLGS (design review);
RR-T2SSKC, RR-ER2WCZ, RR-1KTZJI, RR-AEW30O, RR-M59LZH, RR-4K7AGA, RR-UKME5V,
RR-UWLOPL, RR-5HOA1F (wont-fix with reason), RR-N8PB97, RR-RV3O8Z (wont-fix with
reason), RR-QECZCL, RR-8LSDU0, RR-LPEU7T.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Hues incl. grey/gray, teal, distinct orange and yellow: PASS (TestParseColor, styleColors.test.ts, screenshots).
- Shades `hue/50..900` from the kit: PASS (TestKitRampsMatchTokens, Badge.test.ts).
- Dark mirror and `palette.dark.colors` override: PASS (TestKitRampsMatchTokens, TestResolvePalette_DarkKitOverride, dark screenshot).
- Operator `palette.colors` with optional dark: PASS (TestResolvePalette_OperatorColors, served palette checked over HTTP).
- Automatic AA foreground: PASS (TestForegroundForReachesAA, tokenContrast.test.ts).
- Validation rejects unknown names, bad hex, reserved names: PASS (TestValidatePalette_Colors, TestValidateConfig_StyleColors).
- Valid in styles, badges and kanban dots: PASS (TestBuildStyleMap, Badge/Kanban tests, screenshots). Matrix use lands with TKT-2SGPJZ.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-22MYX5)
- [x] User-facing documentation updated (GUIDE-data-entry Styles; docs/data-entry.md regenerated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-22MYX5

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)
