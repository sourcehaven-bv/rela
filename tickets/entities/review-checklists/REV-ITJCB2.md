---
id: REV-ITJCB2
type: review-checklist
title: 'Review: `_self` on a non-bare face 404s under a configured default_world'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** No new advisory findings introduced; the doclink gate is
clean across the branch.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XD6O6B, RR-VFFS9M, RR-UOHT8D (security); RR-DO86JP,
RR-TLWVZV, RR-4R4CU4, RR-WCXK13, RR-X9P7MW (addressed); RR-RN3YGR (wont-fix,
reasoned); RR-X2GBWJ (deferred to TKT-5SZG2L).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. `GET /policys/POL-1@published` with matching `_self` under `default_world` — PASS (atlas verify manual last section; `TestFacedAddress_GetServesTheNamedFaceUnderAnyWorld`).
2. SPA affordances from `_actions` under any world, stand-in note names the bare face — PASS (Chrome walk on the verify project; EntityDetail/EntityList/Kanban/Calendar world suites).
3. Edit from the concept page opens an editable form — PASS (Chrome walk; DynamicForm fetches the address in the default world).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-JPB9A0

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (PR #1527 was opened with `gh` directly, auto-merge enabled and reviewer requested; CI monitored by hand)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
