---
id: IMPL-OE71
type: implementation-checklist
title: 'Implementation: Inline backtick-triggered entity-reference autocomplete in markdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via Playwright + Puppeteer)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **Composable unit tests:** `useBacktickAutocomplete.test.ts` — 22 cases
covering: trigger detection (prose opens; fenced/url/closing-backtick/
non-backtick all suppress); open-delay grace (non-id char during delay cancels;
Escape during delay cancels); phase transitions (typed prefix → phase id, search
called with correct type arg); phase-1 filter by substring; keyboard navigation
(ArrowDown wraps, Escape dismisses, passes-through non-nav keys); auto-dismiss
(space, cursor move, blur); setHighlight clamping; pick (prefix insert;
manual-id jumps to phase 2 without inserting); dispose cleanly removes all
listeners.
- **Playwright e2e:** `markdown-editor-backtick-autocomplete.spec.ts` —
6 cases all green:
  - typing a backtick in prose opens the popup (phase 1)
  - prefix transitions to phase 2, Enter inserts the `id`
  - typing a backtick inside a fenced code block does NOT open
  - typing ` `flag-name` ` quickly does NOT show the popup
  - Escape dismisses without inserting
  - round-trip: inline insert produces a titled link on the detail page
- **Frontend full suite:** 736 tests / 41 files pass (was 714; +22 new).
- **Full e2e suite:** 197 pass, 1 skipped (was 191; +6 new).
- **Go suite:** all packages green (no Go changes).
- **`just lint` / `just arch-lint`:** clean.
- **`just coverage-check`:** 75.5% total, all floors satisfied.
- **`just build`:** all three binaries build cleanly.

**Real-browser validation via Puppeteer** confirmed the composable's behavior
end-to-end against the production CodeMirror in EasyMDE: typing `see \`` opened
phase-1 popup with prefix list; typing `FEAT-` after that transitioned the popup
to phase id with the correct type filter.

**Design tweaks discovered during implementation:**

- The open-delay timer's pending → prefix transition also needs to
immediately apply any prefix text typed during the delay window — without that
fast typists land in phase 1 with the full prefix list even though the
typed-after-trigger text already matches a prefix exactly. Fixed by running
`filterPrefixList` + `tryExactPrefixMatch` at the end of `transitionToPrefix`.
- The metamodel records prefixes with or without the trailing `-`
(e.g. `TKT-` vs `FEAT`). `tryExactPrefixMatch` now accepts both: exact match OR
prefix-followed-by-`-` initial substring. Phase 2's `partial` slice strips the
optional separator dash too.
- `setHighlight` was added to the controller so the popup component
could move the highlight to a clicked/hovered row in O(1), replacing an earlier
hacky `while (moveHighlight)` loop in MarkdownEditor.

## Quality

- [x] Code follows project patterns (mirrors `EntityPickerModal` for
the popup rendering; uses CodeMirror v5 events the way EasyMDE expects)
- [x] No security issues introduced (insertion path routes through
`insertEntityRef` which already does denylist validation)
- [x] No silent failures (popup errors render to the `errorMsg` field;
store errors propagate from `searchEntities`)
- [x] No debug code left behind
