---
id: REV-S1EVV7
type: review-checklist
title: 'Review: split the SQLite connection from the store'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race` on both the default and `sqlite` builds; `just build-check-tags` clean)
- [x] Lint clean (`just lint`: 0 issues; `just arch-lint`: OK; `just plimsoll`: OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (no package fell below its floor)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-NG220X, RR-7J4A0T)
- [x] All significant review-responses addressed (RR-KFW7G6, RR-AL8RQG)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-NG220X, RR-7J4A0T (critical); RR-KFW7G6, RR-AL8RQG
(significant); RR-IB66ZI (minor, addressed); RR-33CJKK (minor, deferred —
pre-existing).

Both critical findings were reproduced before fixing and re-verified after:
`isFresh` was instrumented to show it returned false on a brand-new database,
then a simulated migration 3 reproduced the predicted `duplicate column name`
fresh-install failure; the DSN truncation was reproduced in a standalone
program that read version 0 and left a stray file behind.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: no implementation checklist; evidence is below)

**Acceptance Status:**

- PASS — conformance suite still passes: `go test -race -tags sqlite
  ./internal/store/...` clean, including `storetest.RunAll` and the fuzz
  functions.
- PASS — a v1 database migrates forward: `TestMigratesV1Forward` builds a
  v1-shaped file, opens it, and asserts the version, the new table and the
  preserved rows.
- PASS — a database from a newer binary is still refused:
  `TestRefusesNewerSchema` (pre-existing, still green).
- PASS — `just build-check-tags` clean across all four tag combinations.
- PASS — plimsoll: `Store` fell from 53 to 50 methods and the directive was
  ratcheted down to match; exported count unchanged at 32.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-S1EVV7)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-S1EVV7

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
