---
id: REV-SQEOA6
type: review-checklist
title: 'Review: Document view flashes empty state and scrolls to top on any unrelated entity write'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: frontend-only change;
`go-test-coverage` floors cover Go packages, and the frontend has no coverage
enforcement by project policy)

Frontend suite 2662 tests / 164 files pass. `npm run typecheck` clean, `npm run
lint` 0 errors (remaining warnings are pre-existing: `v-html` and one non-null
assertion, both predating this change). `just comment-lint` clean across 14838
comments; `just arch-lint` OK. Both Go gates are unaffected by a frontend-only
diff but were run rather than assumed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7DHGWF (critical), RR-ZPPRIH (critical), RR-UVW8YL
(significant), RR-QMSWKL (significant) — all `addressed`.

The two critical findings were both real and both independently reproduced
before fixing:

- RR-7DHGWF: the equality-guard test asserted on `.document-body`, the one
element `v-html` never replaces, so removing the guard killed zero tests. The
first round's "mutation-verified" claim covered only the cold-load half.
Investigating the fix surfaced a further correction: an identical assignment to
a Vue `ref` does not trigger reactivity at all, so the guard never was the
mechanism preventing the repaint. The code comment asserted the opposite and was
wrong in both files; corrected.
- RR-ZPPRIH: no request sequencing. Removing the unconditional blank took away
the thing that made an out-of-order render visible, so the pre-existing race
became able to paint one document's body under another's title.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- No flash / no empty state on an SSE re-render — **PASS**. Browser-measured on
the final build: 1 real `entity:changed` frame from an unrelated `label` write,
0 body unmounts, 0 empty states, scroll held at 900px. The unfixed control on
the same project reproduced 2 unmounts, 1 empty state and a reset to 0. Re-run
after the review fixes with a stricter child-node probe.
- Unchanged re-render performs no DOM write — **PASS** (firstChild identity
preserved; `renderMermaidDiagrams` not re-invoked).
- A real content change still lands — **PASS**.
- Superseded render cannot paint — **PASS** (3 tests; removing the fence fails
two of them).
- Cold load still blanks — **PASS**.

## Documentation (enhancements only)

Skipped: bug fix, no user-facing behaviour to document beyond the fix itself.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no API, config or CLI surface
changed)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on
the bug already being `done`, so it necessarily runs after this checklist closes
- see the note below)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M.
-->
