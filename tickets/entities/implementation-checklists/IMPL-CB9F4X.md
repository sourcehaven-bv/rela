---
id: IMPL-CB9F4X
type: implementation-checklist
title: 'Implementation: CheckboxWidget is unstyled — the only widget with no design tokens'
status: done
---

<!-- @managed: claude-workflow v1 -->
## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: the integration surface here is
      *rendered pixels*, which no test runner in this repo asserts on — jsdom
      does not apply scoped SFC styles and the Playwright suite has no visual
      baseline. Verified in a real browser instead; evidence below)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: scoped CSS has no failure mode to
      surface — no I/O, no parsing, no user input)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation)
- [x] Property comparisons use original object, not hardcoded strings

The display-arm test loops `[true, false]` rather than duplicating a block per
value, so the assertion reads against the mounted widget in both states.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Run against a freshly built SPA **and** a freshly rebuilt `bin/rela-server`
(the stale-binary trap from TKT-3R7RF3: `just ci` does not rebuild it, so a
stale binary embeds a pre-change bundle and silently verifies nothing).
Confirmed the new rule reached the bundle before trusting the browser:
`registry-C6V4fbrl.css` contains `input[type=checkbox][data-v-...]{appearance:none`.

Server: `RELA_DATAENTRY_USER=alice@example.com ./bin/rela-server -project
prototypes/data-entry/project -port 8437`, on
`/entity/ticket/TKT-001` — the `is_blocked` field in the TKT-3R7RF3 demo
section, which is exactly the surface where the problem was reported.

- **AC1 — visual consistency.** Computed style on the edit arm:
  `appearance: none`, `width/height: 18px`, `border-radius: 4px`,
  `cursor: pointer`. Screenshot confirms it sits consistently beside the
  selects, text inputs and textarea in the same 12-column grid. PASS
- **AC2 — checked state.** After clicking: `background-color` and
  `border-color` both `rgb(111, 147, 255)` = `--accent-color` (dark theme);
  `::after` renders `5px × 10px`, `matrix(0.707…)` (a 45° rotation), border
  white. Re-checked in light theme: `rgb(71, 114, 251)` = the light
  `--accent-color`. Both themes PASS
- **AC3 — display arm stays muted.** Pinned by unit test + mutation (below).
  The display arm did not render on the pages exercised, so it is covered by
  test rather than by eye — stated plainly rather than claimed as observed.
  PASS (by test)
- **AC4 — no behaviour regression.** Toggling the live checkbox persisted:
  `TKT-001.md` gained `is_blocked: true` on disk via the autosave PATCH.
  Demo-data change reverted afterwards (`git checkout`). PASS

**Scope check performed live.** The same page renders 4 markdown task-list
checkboxes from the ticket body. All 4 kept `appearance: auto` at 13px,
confirming the scoped rule did not leak past the widget — and confirming the
planning decision to leave that surface alone.

**Mutation test of the new guard.** Removing `class="display-checkbox"` from
the SFC turns the suite red (`1 failed | 59 passed`); restoring it returns it
to green. The guard fails for the right reason, rather than merely existing.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**On DRY.** The rule set is deliberately a second copy of
`RelationCards.vue`'s `.inline-edit-checkbox`, not an extraction. Hoisting it
to a shared stylesheet would mean editing a working component to serve a
styling ticket, and the two sites differ in layout context. Per CLAUDE.md's
"three similar lines is better than a premature abstraction", two copies is
the honest state — recorded as a follow-up rather than silently tolerated.

**Token usage.** `--radius-sm` replaces the literal `4px`. The remaining
values (18px box, 2px border, the checkmark geometry) stay literal because
`scales.css` has no control-size or border-width ramp, and the checkmark
geometry is tuned to the 18px box rather than being a scale step. Inventing
tokens for them would violate the file's own "prefer an existing value in the
tree over a rounder number" rule.

**Verified against the frozen contracts.** The change adds no `--font-size-*`
and touches neither `tokens.css` nor `scales.css`, so the cross-boundary
typography/token contracts are untouched. Confirmed by running the Go-side
guards: `TestAppTokensCSSInSyncWithFrontend`, `TestFrozenTypographyContract­MatchesSPA`,
`TestAppCSSSource`, `TestTokensCSSNeverLayered` — all PASS.
