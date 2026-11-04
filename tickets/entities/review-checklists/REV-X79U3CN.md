---
id: REV-X79U3CN
type: review-checklist
title: 'Review: Place a channel type above the relations by configuration'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Also run: `just arch-lint`, `just plimsoll` (after making the source field
unexported on `SectionData`), `just lint-md`, `just docs` (no drift), frontend
`typecheck`, `lint` (0 errors) and `test:run`, the channel, presence,
relation-order, checkboxes and open-new-tab e2e specs, and `archplan ci`: 0
ratchet metrics worse. Its 5 "new" findings are unused SQLite FTS tables
unrelated to this diff. The archplan baseline was not tightened: CI notes the
static model reads better than the traced weekly run, so tightening here would
make that run read as worse.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CM5RY5L (significant, addressed), RR-FREVJV6,
RR-5NE8H03, RR-FYG88RG, RR-0SIJ3GW (minor, addressed), RR-C08IW4U (minor,
deferred), RR-FU9GPOC, RR-0LCVN67, RR-IW9GFGJ (nit, addressed). The security
review found nothing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC 1 (block before the first relation section): PASS, Go wire test,
`channelPlacement.test.ts`, `EntityDetail.channelPlacement.test.ts`, e2e.
- AC 2 (default unchanged): PASS, existing channel unit and e2e tests.
- AC 3 (unknown value is a load error): PASS, `TestChannelsPresentation_Validate`.
- AC 4 (deep links per block): PASS, `ChannelsSection.deeplink.test.ts`.
- AC 5 (budget): PASS, `ChannelsPanel.placement.test.ts` (single mode and
list plus chosen channel).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6FGNDQH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after the ticket is done, see TKT-UFV01M)
