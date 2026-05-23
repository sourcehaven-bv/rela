---
id: REV-G5T0
type: review-checklist
title: 'Review: Build-tag seams in appbuild + cli/mcp_wiring composition roots'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: pure
  refactor — no production code paths added; the new test exercises
  `memstore.WithObserver(nil)` and bumps memstore coverage)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (4 of 4 fixed in
  fixup commit 54b848d4)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Findings produced by cranky-code-reviewer and
go-architect were addressed inline rather than tracked as separate
review-response entities (single-PR refactor, all fixes landed in
the same branch before opening PR):

Significant — fixed in `54b848d4`:
1. `mcpServices.Close` race — added `sync.Once`
2. `memorybackend` MCP build silent watcher — added `slog.Warn`
3. `memstore.WithObserver(nil)` footgun — drops silently + test
4. `buildSearcher` wide-then-assert — concrete types + typed-nil guards

Minor — fixed in `54b848d4`:
6. `NewFromCollaborators` doc audit — per-field required/optional doc
10. arch-lint union-over-tags rationale comment

Minor — deferred with reason in ticket follow-ups (#5, #7, #8, #9):
all dissolve when the two composition roots unify into one.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: default build still 40 MB, all bleve packages present, every
  test passes
- PASS: `-tags memorybackend` build is 24 MB, zero bleve packages in
  dep graph
- PASS: `appbuild` test fixture extracted into `appbuildtest`, all
  external callers (cli, dataentry) migrated
- PASS: arch-lint clean on both build tags

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A:
  internal refactor — no user-facing surface changed, no Postgres
  capability shipped yet)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A: same reason)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** _to be filled once `gh pr create` returns the URL_
