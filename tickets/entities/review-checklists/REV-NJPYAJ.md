---
id: REV-NJPYAJ
type: review-checklist
title: 'Review: Render admin-authored header/footer markdown on kanban boards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test` passes — `internal/dataentryconfig`, `internal/dataentry` green
- [x] `npm run test:run` — 87 files, **1443 tests**, all passing
- [x] `npm run typecheck` (vue-tsc) — clean
- [x] `npm run lint` — **0 errors** (90 warnings, all pre-existing: `max-lines`,
non-null assertions in `stress/`)
- [x] `just arch-lint` — OK, no warnings
- [x] `just coverage-check` — package (50%) and total (65%) thresholds satisfied;
total 77.2%
- [x] `go build ./...` clean

## Code Review

Reviewed by the cranky-code-reviewer agent, which independently verified the CSS
in a real browser engine and the sanitizer under both happy-dom and jsdom rather
than trusting the comments. Verdict: no critical findings, two significant, one
minor. All three addressed.

- **RR-GNWJFO (significant) — addressed.** Comments in three places claimed the
TypeScript *type* kept the `description` alias away from kanban. False: types
erase at runtime, and configs arrive from the `/_config` HTTP response, so a
future Go `Kanban.Description` field would have silently switched the alias on
while the comment said it could not. Replaced the claim with an actual mechanism
— `viewHeaderMarkdown(view, { allowDescriptionAlias })`, opted into by
EntityList only — plus two regression tests including the exact drift scenario.
- **RR-G80SDH (significant) — addressed.** `overflow-x: auto` on `.kanban-board`
coerces `overflow-y` to `auto` per spec, making it a vertical clip container it
was not before, on every existing board. Benign today and inherent to the scroll
fix, so documented as an accepted constraint (mirroring the comment already on
the swimlane rule) rather than worked around — `overflow-y: visible` cannot opt
out.
- **RR-AKQRNB (minor) — addressed.** Sanitization-test comment named only the
`onerror` happy-dom gap; extended to include `javascript:` hrefs, and added an
environment-independent assertion that markdown was processed. Found in the
process that happy-dom also keeps a `<script>` following inline content, so the
two payloads are kept on separate fields with the reason documented.

Findings deliberately NOT actioned (discretionary, logged for future work):

- *Extract a `<ViewInfo>` component* — the reviewer's leverage suggestion. Both
views now share resolvers and styles, but each still wires its own computeds,
`v-html` div, and eslint-disable. Real duplication, but component extraction is
scope beyond this ticket and better done when a third view wants info regions.
- *`.view-info` naming / `mountBoard` positional `{}` filler* — cosmetic.
- *`-12px` coupled to two headers with no test* — pre-existing coupling
(RR-PUIE0H), now documented in `view-info.css`. The reviewer notes a filter-bar
interaction worth an eyeball; the example board `idea_board` has filter controls
and was visually verified, showing correct spacing.

## Acceptance Verification

All seven criteria verified end-to-end against the real `rela-server` binary
serving the production SPA bundle, driven through a real browser — see
IMPL-EH9RW8 for full evidence. Summary:

1. **AC1 header renders** — PASS (`<strong>` present, markdown parsed).
2. **AC2 footer renders** — PASS (`<em>` present, below the columns).
3. **AC3 sanitized** — PASS (script element stripped; scope caveat documented).
4. **AC4 unset → nothing** — PASS (`idea_by_category` has neither region).
5. **AC5 `_config` serves/omits** — PASS (1 board with both keys, 5 with neither).
6. **AC6 furniture stays put** — PASS, measured: columns moved 264 → -636 on a
900px scroll while header, footer, and `<h1>` all held at 264. Swimlane branch
keeps `overflow-y: hidden` + `border-radius: 8px`.
7. **AC7 lists unchanged** — PASS, re-verified after the RR-GNWJFO refactor:
`active_ideas` (only `description`, no `header`) still renders its region,
proving the alias opt-in works; shared stylesheet applies (`14px`/`22.4px`/
`-12px`).

## Ready

- [x] All critical/significant review responses resolved
- [x] Acceptance criteria verified with evidence
- [x] Docs updated (`docs/data-entry.md` — kanban field table + info-regions
subsection noting the absent `description` alias)
