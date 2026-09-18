---
id: IMPL-M27BGY
type: implementation-checklist
title: 'Implementation: Markdown tables in the detail body render cramped: full body width + horizontal overflow'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: pure presentation change — CSS plus a
DOM transform with no failure mode. `wrapTablesForScroll` returns its input
unchanged when there is no table; there is no error state to surface.)

**What was built.**

1. `frontend/src/styles/markdown-content.css` — cell bounds
(`min-width: 12ch`, `max-width: 40ch`, `overflow-wrap: break-word`) and the
table rule (`width: auto; min-width: 100%`), both scoped
`:not(.milkdown-prose)`. `overflow-x` moved off `.md-body` onto
`.md-table-scroll`, which also gets the app's standard focus ring.
2. `frontend/src/utils/markdown.ts` — `wrapTablesForScroll` + `tableLabel`,
applied inside `renderMarkdown`.
3. `frontend/src/views/DocumentView.vue`, `frontend/src/components/entity/DocumentsPanel.vue`
— both call the helper on their sanitized server HTML, which is why the
server-rendered surfaces get the fix rather than only the client-rendered one.

**Edge cases from planning, all covered:** narrow 2-column table (must still
fill), 12+ columns (must scroll), 300-char unbroken URL (must wrap), empty cell,
links inside cells, already-wrapped table (no double wrap), table-free content,
and a table with no `<thead>` (generic label fallback).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: the tests assert on
HTML-string output of a pure function; inputs are literal markup fragments by
design, which is how the surrounding `markdown.test.ts` cases are written.)
- [x] Property comparisons use original object, not hardcoded strings

9 new cases in `src/utils/markdown.test.ts`, following the existing file's
style. jsdom has no layout engine, so these pin **structure and the
accessibility contract**; pixel widths are verified in a real browser (below) —
the same split the repo already uses for pending-indicator widths.

**The tests were mutation-checked, not just observed passing.** Deleting the
`scroller.setAttribute('tabindex', '0')` line produces:

```
FAIL  src/utils/markdown.test.ts > wrapTablesForScroll > wraps a table in a
      focusable, labelled scroll region
AssertionError: expected '<div class="md-table-scroll" role="re…' to contain
      'tabindex="0"'
```

So the keyboard-accessibility fix cannot be silently removed later.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence.**

Live `rela-server` (`prototypes/data-entry/project`) + Vite, Chrome at a 1800px
viewport, 1200px body column, against a demo entity holding all four table
shapes. Measured via `getBoundingClientRect` / `scrollWidth`, not by eye:

| AC | scenario | before | after |
|---|---|---|---|
| 1 | reported 5-col table | Niveau **60px**, header **68px** (3 lines) | Niveau **103px**, header **43px** |
| 2 | 12-col table | squeezed to 1200, never scrolled | wrapper **1236 > 1200**, scrolls |
| 2 | prose beside a scrolled table | body scrolled as a whole | paragraph **fixed at 264px** while the table's first cell moves 264→229 |
| 3 | 300-char URL cell | — | stays 1200px, wraps, no scroll |
| 4 | narrow 2-col table | fills | still fills (RR-5ZVPC5 held) |
| 5 | all four wrappers | — | `tabindex="0"`, `role="region"`, e.g. `aria-label="Table: Competentie, Niveau, Verworven via, …"` |

Also confirmed post-fix: `.md-body` computed `overflow-x` is now `visible` (it
was the scroll container before), the table's computed `display` stays `table`
(RR-5SURSS), and `documentElement.scrollWidth === clientWidth` — no page-level
horizontal scrollbar.

**AC6 (light/dark):** no new colour literals were introduced; the only colour
added is the existing `--focus-ring` / `--focus-ring-gap` token pair, so both
themes follow by construction.

**Known gap, recorded rather than hidden:** the Milkdown column-resize
interaction was reasoned about and excluded via `:not(.milkdown-prose)`, but was
not exercised by dragging a column handle in the running editor. The exclusion
means the new bounds cannot apply there at all, so the risk is that the
exclusion is *unnecessary*, not that it is insufficient.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Patterns followed.** The wrapper mirrors the existing post-render DOM
transform in the same file (`plantuml-diagram-wrapper`), and the focus ring uses
the mandated two-shadow token pattern from `frontend/CLAUDE.md` rather than a
hand-written colour.

**DRY.** `wrapTablesForScroll` is one shared helper used by all three call
sites, specifically so the client-rendered and server-rendered paths cannot
drift. `tableLabel` is split out because it is a distinct decision (caption →
headers → generic) rather than to avoid repetition.

**Security.** The helper runs on **already-sanitized** HTML and builds the
wrapper with DOM APIs inside an inert `<template>` — never string-splicing
markup around DOMPurify's output, which is the way this kind of helper usually
becomes a sanitizer bypass. No new attacker-controlled attribute is introduced
(the class is static, and the `aria-label` is derived from text content that
DOMPurify has already cleaned). A lint hook flagged the `innerHTML` write; the
reason it is safe is documented at the call site rather than suppressed
silently.

**Full local run:** `npm run test:run` 2736 passed / 168 files; `npm run
typecheck` clean; `npm run lint` 0 errors (125 pre-existing warnings in
unrelated files).

One real failure was found and fixed during the run, not worked around:
`DocumentView.rerender.test.ts` mocks `@/utils/markdown`, so the new export was
missing from the mock and broke 9 tests. An identity stand-in was added with a
comment explaining why the wrapper is a no-op there.
