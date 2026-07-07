---
id: REV-EUTS6V
type: review-checklist
title: 'Review: Relation filter_controls render as target selector (select → typeahead), not free text'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend `npm run test:run` 1188 pass (74 files); e2e `list.spec.ts › Filtering` 5/5; `go build ./...` clean
- [x] Lint clean — `npm run lint` 0 errors (changed files eslint+prettier clean); `just arch-lint` OK (no Go change)
- [x] ~~Coverage~~ — frontend has no coverage enforcement (per CLAUDE.md); no Go package changed, so Go floors unaffected

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer) — 8 findings surfaced, 6 filed as review-responses (2 were "verified-correct, not defects": reactivity of the candidate ref, click-outside listener pairing)
- [x] All critical review-responses addressed (RR-5GU270)
- [x] All significant review-responses addressed (RR-TE8HA6, RR-L78S8H, RR-33NE96)
- [x] Self-reviewed the diff for unrelated changes (none — CLAUDE.md accidental touch was reverted; working tree is scoped to the feature + tickets/ workflow entities)

**Review Responses:**

Design review (all addressed): RR-3MDVZD, RR-X4QWBF, RR-NH8B6D, RR-A51QQ2,
RR-3TJVQJ. Code review (all addressed): RR-5GU270 (critical), RR-TE8HA6,
RR-L78S8H, RR-33NE96 (significant), RR-0TY8MA, RR-SFRV0T (minor).

No open critical/significant responses remain.

## Acceptance Verification

- [x] Each acceptance criterion tested (see IMPL-BJSQWA mapping)

**Acceptance Status:**

- AC1 (select ≤10) — PASS: FilterBar unit "renders select mode at or below the threshold" + e2e `<select>`.
- AC2 (typeahead >10) — PASS: "renders typeahead mode above the threshold (11 candidates)" + mode-flip test.
- AC3 (value = bare title, list narrows) — PASS: EntityTargetSelect "option VALUE is the bare title, not 'Title (ID)'" + e2e narrows tasks to TASK-001 by "User Authentication".
- AC4 (empty clears) — PASS: "empty commit clears the filter" + e2e clear restores list.
- AC5 (property filters unchanged) — PASS: "property filters still render as before" + e2e Filtering group green.
- AC6 (direction from/to) — PASS: incoming→`from` / outgoing→`to` tests.
- AC7 (deep-link) — PASS: "deep-linked value passed through" + "placeholder option for a committed value absent from candidates" (covers the truncation/pre-load case from RR-5GU270).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-OIUDYH)
- [x] User-facing documentation updated (`docs/data-entry.md` Filter Controls section)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-OIUDYH

## Final Checks

- [x] Commit message will explain the why (target selector replaces guess-the-title free text; title-not-id is the load-bearing fact)
- [x] No TODOs or FIXMEs left unaddressed (the original `FilterBar.vue` "use text input for now" TODO is removed)
- [x] Ready for another developer to use (`EntityTargetSelect` is a documented reusable component in `common/`)

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->
