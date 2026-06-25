---
id: REV-40KNQQ
type: review-checklist
title: 'Review: PlantUML diagram rendering in data-entry (remote server)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go htmlutil/dataentry/dataentryconfig + 62 frontend unit tests
- [x] Lint clean (`just lint`) — frontend 0 errors; `go vet` clean; `just arch-lint` OK
- [x] Coverage maintained (`just coverage-check`) — new code covered by added Go + frontend tests

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none remain critical (RR-F6XG58 downgraded to minor after analysis, addressed)
- [x] All significant review-responses addressed — RR-CIAI73, RR-3OY8XW, RR-Q3U5YW all addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-F6XG58 (addressed), RR-CIAI73 (addressed), RR-3OY8XW
(addressed), RR-Q3U5YW (addressed), RR-V0X7FJ (addressed), RR-JTBTUU
(addressed), RR-21O6D4 (deferred — proxy follow-up), RR-PT28IE (deferred —
pipeline convergence)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 (URL set → diagram renders in entity body, sections, documents): PASS — `renderPlantUMLDiagrams` wired into EntityDetail/DocumentView/DocumentsPanel; unit tests assert `<img>` emitted for both source forms.
- AC2 (URL empty/absent → plain code block, no network call): PASS — disabled/empty-string/unsafe-scheme no-op tests assert no `<img>` and the source block left intact.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: feature is config + render parity with mermaid; no user-facing docs written this PR — `app.plantuml_server_url` documented inline in config godoc)
- [x] ~~User-facing documentation updated~~ (N/A: see above)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** none (deferred — mermaid itself has no dedicated user doc;
parity maintained)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — all code/build/test/lint green; the Rela Tickets gate clears once this checklist + the ticket move to done
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1034
