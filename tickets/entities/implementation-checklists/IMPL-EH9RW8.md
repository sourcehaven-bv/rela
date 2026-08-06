---
id: IMPL-EH9RW8
type: implementation-checklist
title: 'Implementation: Render admin-authored header/footer markdown on kanban boards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What changed:**

- `internal/dataentryconfig/config.go` — `Kanban.Header`/`.Footer`, same
yaml/json tags as `List`, with a doc comment on why there is no `Description`
fallback.
- `frontend/src/types/config.ts` — `listHeaderMarkdown`/`listFooterMarkdown`
replaced by `viewHeaderMarkdown`/`viewFooterMarkdown` taking a **structural**
param, so `ListConfig` (has `description`) and `KanbanConfig` (does not) both
satisfy them from one implementation. `KanbanConfig` gains `header?`/`footer?`.
- `frontend/src/styles/view-info.css` — **new**; `.list-info` moved here as
`.view-info` and imported in `main.ts`, following the existing `back-button.css`
shared-stylesheet convention. `:deep()` wrappers dropped (inert in a global
sheet).
- `frontend/src/components/lists/EntityList.vue` — migrated to the shared
resolvers + `.view-info`; local style block removed.
- `frontend/src/views/KanbanView.vue` — header region after the filter bar,
footer as the last child of `.kanban-view` (after every board branch), both
`v-if` + `v-html` with the eslint-disable comment.
- **Scroll containment:** `overflow-x` moved off `.kanban-view` onto
`.kanban-board` (`overflow-x: auto`) and `.kanban-swimlane-board` (`overflow:
auto hidden` — the two-value form preserves the vertical clipping that keeps
cells inside the 8px border radius).
- `docs/data-entry.md`, `tickets/data-entry.yaml` — docs + example board.

## Manual Verification (REQUIRED)

- [x] Feature tested end-to-end manually
- [x] EACH acceptance criterion verified
- [x] Verification evidence documented

Verified against `rela-server --project tickets` (real binary, production SPA
bundle) driven through a real browser (Puppeteer/Chromium), not jsdom.

- **AC1 — header renders sanitized HTML above the board.** PASS.
`/kanban/idea_board` `.view-info--top` renders "Ideas move left to right…" with
a real `<strong>` element (markdown parsed, not escaped).
- **AC2 — footer renders below the board.** PASS. `.view-info--bottom` renders
the italic note with an `<em>` element; screenshot confirms placement under the
columns.
- **AC3 — sanitized via renderMarkdown.** PASS (component test). See the
environment caveat under Quality below — the assertion is script-element
stripping, deliberately not inline handlers.
- **AC4 — nothing rendered when unset.** PASS. `/kanban/idea_by_category` (no
header/footer configured) has neither `.view-info--top` nor `.view-info--bottom`
in the DOM.
- **AC5 — `_config` serves the fields, omits when unset.** PASS.
`curl -H "Origin: …" /api/v1/_config` → `idea_board.header` and `.footer` both
present; the five other boards (`bug_board`, `feature_board`,
`future_concept_board`, `idea_by_category`, `ticket_board`) have neither key
(omitempty). No `description` key on any kanban.
- **AC6 — page furniture stays put while columns scroll.** PASS, measured.
Simple board: `.kanban-view` computed `overflow-x: visible`, `.kanban-board`
`auto`, content genuinely overflows (scrollWidth 1760 vs clientWidth 812).
Setting `scrollLeft = 900` moved the first column's `left` from 264 → **-636**
while header, footer, and `<h1>` all stayed at **264**. Footer is not a
descendant of `.kanban-board`. Swimlane board: computed `overflow-x: auto` /
`overflow-y: hidden` with `border-radius: 8px` intact, scrollWidth 1084 vs
clientWidth 810, scrolls to 274 with the title unmoved — the corner-clipping
risk flagged in planning did not materialize.
- **AC7 — list behavior unchanged.** PASS. `/list/all_ideas` still renders both
regions with markdown intact; no `.list-info` node remains anywhere. Shared
stylesheet confirmed applied: `font-size: 14px`, `line-height: 22.4px` (14 ×
1.6), and `margin-top: -12px` — the value planning flagged as "verify, don't
assume" — transferred correctly to the new sheet. Full frontend suite (1441
tests) green.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)
- [x] Lint clean
- [x] Tests pass

- `go build ./...` clean; `go test ./internal/dataentryconfig/
./internal/dataentry/` green.
- `npm run test:run` — 87 files, **1441 tests**, all passing.
- `npm run typecheck` (vue-tsc) — clean.
- `npm run lint` — **0 errors**. The 90 warnings are all pre-existing
(`max-lines`, non-null assertions in `stress/`); `EntityList.vue` got *shorter*
(1446 lines, styles moved out) and `KanbanView.vue` (941) was already over the
500-line warn threshold before this change.

**Sanitization test — scope note (deliberate, not an oversight):** the AC3 test
asserts that `<script>` elements never survive into the DOM. It does NOT assert
that inline handlers (`onerror=`) are stripped, because under this suite's
**happy-dom** environment DOMPurify leaves `onerror` on an `<img>` when the
input parses to multiple top-level nodes (e.g. `<p>x</p>\n<img onerror=…>`).
This was investigated rather than assumed: the identical payload through
DOMPurify under **jsdom** returns `<img src="x">` with the handler removed, and
DOMPurify sanitizes the payload correctly in isolation under happy-dom too — so
it is a happy-dom DOM defect confined to the test environment, not a
`renderMarkdown` bug and not a production exposure. Asserting no-`onerror` would
have failed for an environment reason while proving nothing about shipped
behavior. Handler stripping is DOMPurify's own contract; what is ours to prove
is that the sanitizer sits in the path at all, which the test does.

Worth noting for a future ticket: this happy-dom weakness means **no** frontend
test in this repo can meaningfully assert inline-handler sanitization, including
for entity content rendered by `EntityDetail.vue` / `api/views.ts`. That is a
pre-existing gap in test-environment fidelity, not something this ticket
introduced.
