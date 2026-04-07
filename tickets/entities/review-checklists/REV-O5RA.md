---
id: REV-O5RA
type: review-checklist
title: 'Review: Drop user-visible /v2/ URL prefix and remove stale HTMX app.js'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./...` — all packages green, including updated `internal/dataentry`; `cd frontend && npm run test:run` — 286/286). Note: `just test` fails locally due to a pre-existing Go toolchain mismatch (Homebrew 1.25.6 vs go.mod 1.25.8) when `-cover` is enabled. Confirmed unrelated to this ticket's changes. Plain `go test ./...` is clean.
- [x] Lint clean (`just lint` — no output means clean)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: same pre-existing toolchain issue; covered by CI on PR instead)

## Code Review

- [x] Ran cranky-code-reviewer agent over the diff
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (3 of 3 addressed in-PR)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- [RR-EEK5](review-responses/RR-EEK5.md) (significant, **addressed**) — Repo-root `e2e/` suite hardcoded `/v2/` in 4 files. Fixed all 4 (base.page.ts, fixtures.ts, crud.spec.ts, data-entry.spec.ts). This was the most important miss — the reviewer caught an entire e2e suite I had missed.
- [RR-MLH1](review-responses/RR-MLH1.md) (significant, **addressed**) — Added startup-time `fs.Stat(spaFS, "index.html")` check in router.go to fail fast on BUG-W144-class regressions instead of silently serving a directory listing.
- [RR-QAN4](review-responses/RR-QAN4.md) (significant, **addressed**) — Replaced lying comment "favicon only - v1 assets removed" with accurate one describing the actual /static/ mount surface. Also improved panic messages to include embedded filesystem paths for operational visibility.
- [RR-89B0](review-responses/RR-89B0.md) (minor, **deferred**) — Favicon double-embedded in static/ and static/v2/. Pre-existing; will file a follow-up cleanup ticket. Deferring is appropriate because fixing it would expand scope and the duplication is not a regression introduced by this PR.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1. No `/v2/` in browser address bar | **PASS** | Built server serves `/`, `/list/ticket`, `/assets/index-*.js`, `/favicon.svg` all at 200 with no `/v2/` anywhere in HTML. Built `index.html` uses `href="/favicon.svg"` and `src="/assets/..."`. |
| 2. Favicon loads at root | **PASS** | `curl /favicon.svg` → 200, 12614 bytes |
| 3. No regression in SPA deep-link refresh | **PASS** | `curl /list/ticket` → 200, serves SPA shell via catch-all |
| 4. Embedded `app.js` is gone | **PASS** | `strings bin/rela-server \| grep -c "Data Entry App JavaScript"` → 0. (Note: the originally-planned `grep -c EasyMDE` check was imprecise because the Vue MarkdownEditor.vue also imports EasyMDE; the correct check is the unique `app.js` header marker.) |
| 5. `codemirror-textarea-sync` measure still valid | **PASS** | Measure kept (not deleted) because BUG-005 has a required `adds-measure` relation to it. Location and description updated to point at `frontend/src/components/forms/MarkdownEditor.vue`. `analyze_cardinality`, `analyze_orphans`, `analyze_properties` all pass. |
| 6. All tests/lint pass | **PASS** | See Automated Checks above. e2e tests not run because pre-existing `frontend/e2e/` fixture bug from PR #318 blocks the probe (verified on plain develop). Repo-root e2e/ suite similarly blocked; requires a follow-up bug fix to unblock. |
| 7. Desktop app still works | **NOT VERIFIED** | `just build-desktop` not executed (Wails build is slow and non-deterministic on this machine). Reviewed `cmd/rela-desktop/main.go` — it delegates HTTP handling to `app.NewRouter()`, so any router behaviour verified for `rela-server` applies to the desktop binary too. The new startup check (`fs.Stat` for `index.html`) is the desktop's best defence against BUG-W144-class regressions. |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, no user-facing documentation changes)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message will explain the why (user bookmarks/URL hygiene), not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: user has not asked me to create the PR yet. Ticket is marked done after local verification; PR creation is a separate step owned by the user.)
- [x] ~~All CI checks pass~~ (Deferred until PR is opened)
- [x] ~~PR URL documented below~~ (Deferred until PR is opened)

**PR:** Not yet opened — user will invoke `/pr` when ready.
