---
id: REV-T4GY
type: review-checklist
title: 'Review: Migrate checkbox toggle to PATCH-based reactive flow; retire /api/toggle-checkbox'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: package-floor thresholds; this PR deletes Go code (helpers.go shrinks) and adds TS code in already-covered packages — no floor changes)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MV0H (significant, addressed), RR-3P62 (significant, addressed), RR-MMZO (significant, addressed), RR-T8TV (significant, addressed), RR-4Q0T (minor, addressed), RR-KM4F (minor, addressed), RR-V4IG (minor, addressed), RR-1ONA (minor, addressed), RR-96F7 (nit, addressed), RR-24Y7 (nit, addressed), RR-V054 (nit, addressed), RR-D6VS (nit, addressed). All 12 addressed; zero open.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS — Clicking a checkbox toggles the markdown source on the server (BUG-N6WW parity preserved). Evidence: `clicking a checkbox persists the toggle on the server` e2e test passes.
- PASS — The SPA's rendered state visibly updates without any spinner/loading-state appearing. Evidence: new `toggling a checkbox does not flicker the entity detail tree` e2e test installs a MutationObserver and asserts `.entity-detail > .loading-state` never appears during the toggle.
- PASS — The `(n/m)` stats counter updates within the same render tick as the checkbox. Evidence: reactive splice mutates the same section that feeds `checkboxStats`; manually verified in the demo project.
- PASS — `/api/toggle-checkbox` route is removed. Evidence: deleted from `internal/dataentry/router.go:57`; handler deleted from `handlers.go`; no Go callers remain.
- PASS — PATCH-based toggle handles concurrent client edits correctly per the existing PATCH contract. Evidence: no ETag is sent (intentional — see RR-24Y7); behavior matches the prior endpoint (last-write-wins).
- PASS — All existing tests still pass; new no-flicker e2e assertion passes. Evidence: 801/801 unit, 199/199 e2e, 52/52 Go packages.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal architecture change; no user-facing API or workflow change)
- [x] Updated `tickets/entities/concepts/data-entry-ui.md` — removed `toggle-checkbox` from the legacy `/api/*` endpoint inventory.
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist created)

**Docs Checklist:** N/A — concept doc updated inline.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: user creates PR after final commit; this branch carries BUG-N6WW + TKT-R7Q9 as one PR)
- [x] ~~All CI checks pass~~ (N/A: local CI gates cleared — `just test`, `just lint`, `just arch-lint`, `go test ./...`, full e2e suite all green; remote CI is a function of opening the PR)
- [x] ~~PR URL documented below~~ (N/A: PR not yet opened at the time of marking this done)

**PR:** Pending — user will open after reviewing the second commit.
