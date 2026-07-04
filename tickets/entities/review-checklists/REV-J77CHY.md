---
id: REV-J77CHY
type: review-checklist
title: 'Review: Reposition Properties auto-save indicator inline in the section heading, hidden when idle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `npm run test:run`: 71 files, 1150 tests pass (9 AutoSaveIndicator + 16 SectionEditForm, incl. new review-driven cases)
- [x] Lint clean — `npx eslint src`: 0 errors (39 pre-existing warnings in unrelated files, none in changed files)
- [x] Typecheck clean — `npm run typecheck` (vue-tsc): 0 errors. (Frontend has no coverage enforcement per CLAUDE.md; unit tests run plain.)

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer invoked on commit b1638002
- [x] All critical review-responses addressed (RR-32ARO9)
- [x] All significant review-responses addressed (RR-4SN00Y, RR-ZE29PY)
- [x] Self-reviewed the diff for unrelated changes (only the 3 components + 2 test files + ticket entities; reverted accidental TKT-VZB9O title edit from manual testing)

**Review Responses:** RR-32ARO9 (critical, addressed), RR-4SN00Y (significant,
addressed), RR-ZE29PY (significant, addressed), RR-95OACT (minor, addressed),
RR-DNWKY0 (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (planning PLAN-PYE29T AC1-5)
- [x] Test evidence documented in implementation checklist (IMPL-C0LALM)

**Acceptance Status:**

- AC1 (idle hidden): **PASS** — real app: `autosave-hidden` at idle; no floating check.
- AC2 (inline placement while saving): **PASS** — real app geometry: indicator right edge = header right edge (1376px), same top as heading.
- AC3 (hold ~1s then fade out): **PASS** — real app poll: idle→saving→saved(held >1s)→idle; 0.3s opacity fade.
- AC4 (error persists): **PASS** — unit test: error stays visible, not hidden.
- AC5 (no cards/list regression): **PASS** — slot-override test + full suite green.

Post-review additions verified in real app: `data-visible` removed; sr-only live
region empty at idle, announces "Saving…"→"Saved" on edit; visual wrapper
`aria-hidden`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (see below)
- [x] ~~User-facing documentation updated~~ (N/A: cosmetic indicator micro-behaviour, no documented feature/API surface)
- [x] Docs-checklist marked as done

**Docs Checklist:** created below

## Final Checks

- [x] Commit message explains the why (float removal + hidden-until-needed + a11y)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->
