---
id: IMPL-O7F1T8
type: implementation-checklist
title: 'Implementation: Global `/` search shortcut fires while typing in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the
change replaces a boolean guard expression; there is no error path)

Two one-line guard swaps plus three test files:

- `Sidebar.vue:77` — `isInputFocused()` replaces the inline tagName check.
- `SearchView.vue:181` — same, for the `f` filter shortcut.
- `utils/dom.test.ts` — 8 unit tests for the shared guard itself.
- `Sidebar.shortcut.test.ts` / `SearchView.shortcut.test.ts` — mount the REAL
components and assert the shortcut does not fire with focus in a contenteditable
host. These are the integration half: the defect was never in the guard, it was
a handler not calling it, which only a mount can see.
- `utils/inputGuardConvention.test.ts` — the source scan.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Every new test was **mutation-verified** rather than assumed:

| Mutation | Result |
|---|---|
| Reintroduce tagName guard in Sidebar | 3/6 behavioural + scan fail |
| Remove Sidebar guard entirely | scan fails |
| Remove SearchView guard entirely | scan fails |
| Pre-fix SearchView guard | 2/3 behavioural fail |

The "remove guard entirely" mutation initially PASSED. The scan's
`BARE_PRINTABLE` pattern matched only `e.key === 'x'`, but the Sidebar
early-returns with `e.key !== '/'`. Pattern widened to `[!=]==?`, which is the
exact shape this bug had. Caught because the mutation was actually run.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` (as
`alice@example.com`, the editor principal) with the real embedded SPA bundle, on
the `edit_ticket` form for TKT-001, which renders a live Milkdown editor
(`.ProseMirror`, `contenteditable="true"`).

| Build | Focus | `/` result |
|---|---|---|
| Pre-fix | Milkdown editor | path → `/search`, `defaultPrevented: true` — **bug reproduced** |
| Fixed | Milkdown editor | path unchanged, `defaultPrevented: false` — key reaches the document |
| Fixed | body (no text surface) | path → `/search`, `defaultPrevented: true` — **shortcut still works** |

The last row is the one that matters beyond the fix: the guard narrows the
shortcut without disabling it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The source scan follows the existing `styles/focusRing.test.ts` precedent (walk
source, skip comments, reasoned exemption map) rather than inventing a
mechanism. The DRY opportunity is the fix itself: two copies of a rule collapsed
onto the one shared helper.

One judgement call worth recording. The scan's positive half was first written
as "any file with a document-level keydown listener must import the guard." That
flagged five files — four Escape-only handlers and DynamicForm's Cmd+Enter save.
All five are correct as they stand: Escape and Cmd+Enter are *supposed* to work
from inside a text field, and guarding them would be a regression. The assertion
was narrowed to bare printable keys, which is the actual risk surface, instead
of exempting the five.

Verification: 2680/2680 frontend tests pass, `vue-tsc` clean, 0 lint errors,
production build reproducible (no `internal/` diff).
