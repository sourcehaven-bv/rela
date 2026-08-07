---
id: IMPL-DJ9Y71
type: implementation-checklist
title: 'Implementation: Replace emoji with an SVG icon set; add icon: to kanban columns and swimlanes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Commit `37b4ed46`. `lucide-vue-next` (ISC); `utils/icons.ts` as the single
config-string→component boundary; `ValidIconNames` + `validateIconName` Go-side;
`Icon` on `KanbanColumn` and `KanbanSwimlane`; sidebar and kanban rendering;
prototype YAML migrated; docs authored in `docs-project/`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`TestIconAllowlistMatchesFrontend` compares the two REAL sources rather than
either against a third literal list in the test — that is what makes it a
contract test instead of a restatement.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

**AC 1 — self-hosted.** Lucide imported by name; the build produces no runtime
fetch for icon assets.

**AC 2 — all nine sidebar emoji gone.** Counted before (9, via a unicode-range
grep) and after (0). That includes the five outside the old `getIconEmoji`
switch, which RR-09N4MN flagged as easy to miss.

**AC 3 — icons follow the theme.** Verified in-browser, and this took two
attempts: toggling `.dark` on `<html>` directly did NOT change the tokens, so my
first measurement wrongly suggested icons ignored the theme. Using the real UI
toggle showed the truth — in light mode the kanban icon stroke is
`rgb(25,25,25)`, exactly matching its label's colour; in dark it is
`rgb(236,233,224)`. The icon inherits `currentColor`, which is the entire
justification for the swap.

Worth noting the sidebar icons legitimately do NOT change colour: the sidebar is
deep navy in both themes by design (PR 1's `tokens.css`), so its icons correctly
hold one colour.

**AC 4 — kanban icon field.** 15 icons render as real `<svg>` elements (12 nav
+ 3 column). Confirmed by tag name, not by eye.

**AC 5 — prototype migrated.** Emoji moved out of `label:` into `icon:` for all
three columns; server loads with no config error.

**AC 6 — unknown name.** `validateIconName` rejects with an indexed message
listing the valid set; `resolveIcon` falls back without throwing.

**AC 7 — emoji in a label renders verbatim.** No parsing of label text exists,
by design.

**AC 8 — no regression.** Frontend 1499/1499 (93 files), Go tests ok,
`golangci-lint` 0 issues, ESLint 0 errors, typecheck clean, `just docs-check`
passes.

**A real bug the tests caught.** `resolveIcon` originally used `ICONS[name] ??
DEFAULT_ICON`. A bare index finds INHERITED `Object.prototype` members, so a
config naming `toString` or `constructor` would have returned a function that is
not a component and crashed the render. The test I wrote for prototype keys
failed on first run; fixed with an own-property check.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows `TestAppTokensCSSInSyncWithFrontend` for the cross-language pin, and the
`span:` precedent from PR 2 for the config-surface shape (one optional field,
loud load-time validation, frontend fallback as defence-in-depth).

Security: icon names select a component, so the lookup is an allowlist by
construction — no dynamic resolution from a config string, and the own-property
check closes the prototype-chain hole. Icons are bundled, so no new network
surface.
