---
id: REV-299VJY
type: review-checklist
title: 'Review: Ctrl/Cmd-click (and middle-click) should open data-entry rows and cards in a new browser tab'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

| Gate | Result |
|---|---|
| Frontend unit | **2067 pass** / 127 files |
| E2E (Playwright) | **272 pass**, 8 skipped, 0 fail |
| `vue-tsc --noEmit` | clean |
| `eslint src/` | 0 errors |
| `just comment-lint` | no unresolvable doc links across 11426 comments |
| `just coverage-check` | PASS — package 50% and total 65% thresholds satisfied (78.8% total) |
| `just arch-lint` | OK — no warnings |
| `go build ./...` | clean |

One e2e flake (`forms-id-controls.spec.ts`, unrelated to this change) failed
once under parallel load; passed in isolation and on a clean full re-run.

**Comment findings:** none introduced.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** two rounds, 13 total, all addressed.

*Design review (pre-implementation)* — RR-6PBTF1, RR-I4WQPX, RR-NPDW9A
(critical); RR-0PEI9F, RR-UHNHX4 (significant); RR-Z0AHAY (minor). These
reshaped the plan: the shared-target refactor became mandatory, the
stretched-link was chosen over a title-only anchor, and a security claim was
withdrawn as incorrect.

*Code review (post-implementation)* — RR-BRVI7N, RR-88510Z (critical);
RR-HIYJHR, RR-SYFX1B, RR-WEJM54, RR-PB8DO8 (significant); RR-5Z83S0 (minor).

The two critical ones were real bugs the implementation had missed:

- **RR-88510Z** is the most serious: `EntityDetail.vue:1182/:1245` were real
`<a href>` anchors carrying unconditional `@click.prevent`, suppressing the
browser default on EVERY click — **this ticket's own bug, still live in a file
this ticket edited.** Missed because the survey looked for `@click` on
NON-anchor elements. Confirmed live in a browser before the fix
(`defaultPrevented=true` at bubble phase, current tab navigated) and after (zero
`preventDefault` calls, tab opens). The entity-detail table-display section had
NO e2e coverage at all, which is *why* it survived — a fixture and a test now
exist.
- **RR-BRVI7N**: the palette guarded only the inner `RouterLink`, so an entity
with no resolvable route rendered an empty zero-height `<li role="option">` —
invisible, still counted by the arrow-key highlight, nameless to a screen
reader. Worse than the inert row it replaced.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| 1 — cmd/ctrl-click opens a background tab | **PASS** | e2e `context.waitForEvent('page')` on list row, kanban card and entity-detail section cell; original tab URL unchanged |
| 2 — middle-click opens a background tab | **PASS** | e2e `click({ button: 'middle' })` |
| 3 — shift-click opens a new window | **PASS** | browser-native once the element is a real anchor; guard covers `shiftKey` (unit) |
| 4 — right-click offers "Open in new tab" | **PASS** | `a[href]` asserted present per surface |
| 5 — plain click unchanged | **PASS** | pre-existing `list.spec.ts:187` still green; component tests assert unchanged push args |
| 6 — nested controls unaffected | **PASS** | `crud.spec.ts` delete-from-list + cancel, `checkboxes.spec.ts`, `kanban.spec.ts` 15/15 incl. drag, `relation-cards.spec.ts` 12/12 |
| 7 — href carries the same query as the push | **PASS** | live browser: href and popup URL match character-for-character incl. `from=`/`scope=`; unit test derives expectation from the actual push payload |

**Mutation-verified** (each reverted after): dropping the query from the href
fails the two scope tests; removing the modifier guard fails all five
modifier-click tests; removing the palette's `v-else` fallback fails the
empty-option test. The C2 e2e test was confirmed to fail against the unfixed
bundle before the fix landed.
