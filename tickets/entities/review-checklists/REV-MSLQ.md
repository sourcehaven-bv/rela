---
id: REV-MSLQ
type: review-checklist
title: 'Review: Analyze page warning count out of sync with visible tables (gaps + duplicates hidden)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: frontend-only
change exercised by full vitest suite which passes; no Go production code
changed)

Evidence:

- `npm run test:run` (frontend): 45 test files, 787 tests, all pass.
Including 4 new vitest cases for the new section rendering and the badge-sum
invariant.
- `npm run typecheck` (frontend, vue-tsc): clean.
- `npm run lint` (frontend, eslint): 0 errors (75 pre-existing warnings,
none in changed files).
- `go test ./internal/dataentry/`: pass — including new
`TestRunAnalysisSectionNames` pinning the section-name wire contract.
- `go test -race ./internal/dataentry/`: pass.
- `just lint` (golangci-lint): 0 issues.
- `just arch-lint`: OK — no warnings.
- `npx tsc --noEmit` (e2e): clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none flagged)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- [[RR-3QL5]] (significant): addressed — added
`TestRunAnalysisSectionNames` Go test pinning the ordered section names and
pointing at the SPA + e2e consumers in its failure message.
- [[RR-BZNO]] (significant): addressed — replaced fragile
`text().includes(...)` card selector with a `findCard` helper that matches
`.check-title` via `startsWith`.
- [[RR-W6LX]] (significant): addressed — Duplicates test now asserts
message content fidelity and the `.clickable` class.
- [[RR-XWNK]] (minor): addressed — the badge-sum invariant test
implicitly proves correct key lookup across all six cards together.
- [[RR-ZX60]] (minor): addressed — rewrote the comment above
`ANALYSIS_CHECKS` in `e2e/tests/fixtures.ts`.
- [[RR-6GAP]] (nit): addressed — expanded the comment above
`CHECK_TYPES` in `AnalyzeView.vue` to spell out the three-way contract.
- [[RR-F3AD]] (nit): addressed — moved the three new tests into a
sibling `describe('AnalyzeView section rendering (GH#785)')` block.
- [[RR-Z4JA]] (nit): addressed — badge-sum test now derives both sides
from the rendered DOM and asserts equality.
- [[RR-506Q]] (minor): deferred — `section.Name` doubling as both UI
label and wire key is a legitimate smell. The fix is an architectural refactor
(separate `section.Key` from `section.Label`) that is well out of scope for the
GH#785 visibility patch. Recorded as tech debt.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (Duplicates card + rows render) — **PASS**. Vitest
`'renders a Duplicates card with clickable rows showing duplicate messages'`
asserts card presence, count badge, row count, message-content, and `.clickable`
class. Manually verified at `/analyze` against the `tickets/` project: 2
duplicate rows for FEAT-004 / FEAT-007.
- AC2 (ID Gaps card + inert rows) — **PASS**. Vitest
`'renders an ID Gaps card with inert rows for each missing ID'` asserts card,
count, row count, `.clickable` absent, and gap message in row text. Manually
verified: 45,522 missing-ID rows rendered with em-dash placeholders, none
clickable.
- AC3 (badge total = sum of card counts) — **PASS**. Vitest
`'summary badge total equals sum of visible card counts'` derives both sides
from the rendered DOM and asserts equality, plus a sanity check on the seeded
count. Manually verified: `3 errors / 45526 warnings` badge = `0+0+5+0+2+45522 =
45529` card-count sum.
- AC4 (existing four sections unchanged) — **PASS**. Original 5 vitest
cases in `'AnalyzeView click discrimination'` continue to pass. Manually
verified: Properties, Cardinality, Validations, Orphans render unchanged at the
top of the page.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: UI
consistency fix; no user-facing docs reference the analyze page's hidden
categories)
- [x] ~~User-facing documentation updated~~ (N/A: the new card
descriptions are self-documenting; no doc-site mentions of the analyze page's
check-type list)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred to a
separate user action; the `/ticket` workflow completes at the `done` transition.
The user will run `/pr` when ready to merge.)
- [x] ~~All CI checks pass~~ (will be checked at PR creation time)
- [x] ~~PR URL documented below~~ (N/A until `/pr` is invoked)

**PR:** *Pending — `/pr` will be run separately by the user.*
