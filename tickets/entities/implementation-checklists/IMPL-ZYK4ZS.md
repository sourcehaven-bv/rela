---
id: IMPL-ZYK4ZS
type: implementation-checklist
title: 'Implementation: Design tokens: spacing, radius, typography and elevation scales for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Commit `9066f140` on `feat/design-tokens-TKT-8VVBRI`. 13 files, +158/-112.

New `frontend/src/styles/scales.css` (spacing on a 4px base; radius 3 steps +
pill + circle; type ramp; 3 elevation steps with a `:root.dark` override),
imported from `main.ts` after `tokens.css`. 112 declarations across 9 components
migrated from literals to tokens.

The "integration test" here is the cross-boundary contract assertion in
`TestAppCSSSource` plus in-browser verification of computed styles — a CSS token
layer has no runtime flow to integration-test beyond that.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: the contract test
asserts literal CSS name/value pairs on purpose — the whole point is that the
*literals* are frozen, so deriving them from the source under test would make
the assertion vacuous.)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the real server (`rela-server` on :8099, prototype project, editor role) and
drove it in headless Chrome.

**AC 1 — scales defined in a boundary-respecting location.** `scales.css`
created as a sibling of `tokens.css`, not an extension of it.
`TestAppTokensCSSInSyncWithFrontend` still passes, confirming the Go copy of
`tokens.css` is untouched.

**AC 2 — reuses the frozen names.** `--font-size-{sm,base,lg,xl}` =
12/14/18/22px, byte-matching `appTypographyCSS`. Added `--font-size-md` (13px)
as an SPA-only step inside the contract's 12->14 gap — documented in the file
header and in `frontend/CLAUDE.md` as not crossing the app boundary.

**AC 3 — Go-side contract test.** Strengthened `TestAppCSSSource` to assert
name/**value** pairs rather than names alone. **Negative-tested**: temporarily
set `--font-size-lg: 19px` in `apps_css.go`; the test failed with `frozen
typography contract --font-size-lg must be 18px / if you changed this, change
frontend/src/styles/scales.css to match`, then restored the file and
re-confirmed green. A contract test that cannot fail is worthless.

**AC 4 — token adoption, before/after census.**

| Scope | Metric | Before | After |
|---|---|---|---|
| Migrated surfaces | distinct radius literals | 9 | **0** |
| Migrated surfaces | distinct font-size literals | 15 | **7** |
| Migrated surfaces | declarations using tokens | 0 | **112** |

The 7 remaining font-size literals are the deliberately off-scale sizes
(9/10/15/16/17/20/21/24/48px, 1-3 uses each). Rounding them would change type
sizes an author chose on purpose, so they stay.

Repo-wide distinct-literal counts barely move, because unmigrated files still
use every value — that is expected for a scoped migration and is why the census
above is scoped to the target surfaces.

**Runtime resolution (not just "the CSS parses").** Via `getComputedStyle` in
the browser: all 15 sampled tokens resolve to their intended values.
Spot-checked real elements — `.badge` computes to `12px / 4px` (`--font-size-sm`
/ `--radius-sm`), `input` to `14px / 6px` (`--font-size-base` / `--radius-md`).

**AC 5 — both themes.** Screenshots of entity detail, kanban board and list
view. Dark and light both render unchanged, which is the intent: this is a
value-substitution PR, so *visual equivalence is the pass condition*. The
scannability change lands in PR 2. Confirmed the dark-mode elevation override
works: `--shadow-sm` = `#00000014` light vs `#0000004d` dark.

**AC 6 — no regression.** `npm run test:run` 1429/1429 pass (87 files); `go test
./internal/dataentry/... ./internal/dataentryconfig/...` ok; `npm run typecheck`
clean; `npm run lint` 0 errors (90 pre-existing warnings). `npm run
format:check` reports 123 files — **verified identical on a clean stash of the
branch**, so it is pre-existing and untouched.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the `styles/back-button.css` precedent for a shared stylesheet imported
from `main.ts`. The migration was done with a throwaway script restricted to
exact `property: <literal>;` matches — shorthand (`border-radius: 6px 6px 0 0`),
`50%`, `em`/`rem` and existing `var()` uses were left alone because they need a
human decision rather than a regex. The script was deleted after use, not
committed.

No new dependency, no config surface, no user input, no `v-html`.
