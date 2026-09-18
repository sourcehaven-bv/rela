---
id: REV-5O5Y5M
type: review-checklist
title: 'Review: Markdown tables in the detail body render cramped: full body width + horizontal overflow'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `npm run test:run`: **2740 passed / 168 files**
- [x] Lint clean — `npm run lint`: **0 errors** (125 warnings, all pre-existing
in unrelated files), `npm run typecheck` clean
- [x] ~~Comment lint gate~~ (N/A: `commentlint` checks Go comments; this change
touches no Go files)
- [x] ~~Coverage~~ (N/A: `go-test-coverage` enforces Go package floors; the
frontend has no coverage enforcement — `frontend/CLAUDE.md`. Frontend tests run
plain, and 4 new cases were added.)

**`just ci` fails, and it is not this change.** Go lint dies on
`cmd/rela-desktop/main.go:26` with `could not import C (cgo preprocessing
failed)` importing Wails. Verified by stashing the entire diff and re-running on
a clean tree — **identical failure**. This diff touches zero Go files, and CI
runs Linux. Local macOS toolchain issue, not a merge blocker.

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer, on the actual diff)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5SURSS, RR-B22351 (design review) · RR-MDH0B1,
RR-AYBZ7W, RR-27MW0U, RR-4R3XQP, RR-DWO2KJ, RR-TDELP8 (code review). All
`addressed`.

The code review found **two real bugs I introduced**, both from the same root
cause — using a descendant query where a scoped one was needed:

- **RR-MDH0B1 (critical).** `querySelectorAll('table')` wrapped nested tables,
stacking two `role="region"` tab stops, and `tableLabel` named the OUTER region
from the INNER table's headers. Reproduced (`count: 2`, labels `["Table: OUTER,
INNER", "Table: INNER"]`), fixed, re-verified (`count: 1`, `["Table: OUTER"]`).
My JSDoc had claimed "top-level" while the code did not. The first fix attempt
(`table.closest('table') !== table`) was **wrong** — `closest` starts at the
element itself — and was caught only by re-running the probe rather than
re-reading the code.
- **RR-AYBZ7W (critical).** The new scroll container invalidated
`BlockCommentOverlay`'s host-relative position math, so a comment affordance on
an image inside a table would drift on scroll and never re-measure
(`ResizeObserver` does not fire on scroll). Fixed with passive scroll listeners,
cleaned up on re-observe and unmount.

**One finding was already fixed before the review landed** (RR-4R3XQP, the
EasyMDE preview) — found while re-checking that surface, so the reviewer read a
slightly stale tree. Noted rather than double-counted.

**Self-review:** no unrelated changes, no debug code, no TODO/FIXME (grepped).
The demo entity used for browser verification was created and deleted, and is
not in the diff.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status** — all measured in Chrome at an 1800px viewport / 1200px
body column against a live `rela-server`, re-run after the review fixes:

| AC | criterion | result |
|---|---|---|
| 1 | no mid-word breaking | **PASS** — Niveau 60px → **103px**, header 68px → **43px** |
| 2 | wide table scrolls, prose does not | **PASS** — wrapper 1236 > 1200; paragraph fixed while the table's first cell moves 264 → 229 (`proseMoved: false`) |
| 3 | long URL wraps, no page scrollbar | **PASS** — table stays 1200px, `pageScrolls: false` |
| 4 | narrow 2-col table still fills | **PASS** — `narrowFills: true` (RR-5ZVPC5 held) |
| 5 | scroll region keyboard-reachable | **PASS** — all 4 wrappers carry `tabindex="0"`, `role="region"`, and a scoped label |
| 6 | light + dark correct | **PASS** — no new colour literals; only the existing `--focus-ring` token pair |

Also confirmed: `.md-body` computed `overflow-x` is now `visible`, and the
table's computed `display` stays `table` (RR-5SURSS).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — **DOCS-P5QN1O**
- [x] User-facing documentation updated (N/A with reasons recorded there; the
load-bearing documentation is the CSS comment that replaced a **false** one)
- [x] Docs-checklist marked as done

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**Known gaps, recorded rather than hidden:**

- `TextSelectionComment` shares the drift math but is transient and needs a
selection made inside a scrolled cell; left alone deliberately (RR-AYBZ7W).
- The EasyMDE preview has no scroll wrapper by design, so a very wide table
wraps there rather than scrolling (RR-4R3XQP).
- Milkdown column-resize was excluded via `:not(.milkdown-prose)` but not
exercised by dragging a handle (IMPL-M27BGY).
- `.table-wrapper` / `.table-scroll-wrapper` (app layout tables) have the same
missing-`tabindex` defect; out of scope, left for a follow-up (RR-B22351).

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
