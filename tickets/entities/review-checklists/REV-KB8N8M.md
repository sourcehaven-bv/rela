---
id: REV-KB8N8M
type: review-checklist
title: 'Review: Implement in-app configuration editing (Configure space)'
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review and code review (cranky backend, security,
frontend): RR-19K90H, RR-1IXAQ5, RR-1VCG28, RR-3P5TAM, RR-6RR0PW, RR-6ZMAPQ,
RR-76VJLA, RR-7KJOAK, RR-7MZ7QF, RR-93H2JP, RR-A6IGMM, RR-B4J1E4, RR-CVD63U,
RR-D9S15W, RR-DCG957, RR-E8XSDJ, RR-EB83J0, RR-EMVODM, RR-EWWND6, RR-F8NGL2,
RR-G2AZ9Q, RR-GOCII8, RR-HYU4H6, RR-IJU1J8, RR-IY1DXT, RR-M343BQ, RR-M3VHQH,
RR-M8Q4CZ, RR-N4G7P3, RR-NOKSKF, RR-OSRK3J, RR-OY92Q5, RR-P5HRGM, RR-PEXQRL,
RR-POUOW7, RR-QCFVG6, RR-SVHXE0, RR-SX0GQ8, RR-TCRYDH, RR-U7R63P, RR-UCUPAK,
RR-UEUXOL, RR-UKM5ES, RR-VGSVN2, RR-WERY5D, RR-YOTNOW, RR-Z2U13A, RR-ZINV3D,
RR-ZXMHNA. Two minor findings deferred with a reason; none open.

**Automated checks evidence (2026-10-05):** `go test -race ./...` all packages
pass; golangci-lint 0 issues on touched packages (also with `postgres` and
`sqlite` tags; one pre-existing `unused` in appbuild_sqlite.go); arch-lint,
plimsoll, comment-lint gates clean. `just coverage-check`: every floor passes
(`internal/git` reran at 38.1% after the local gpg pinentry was restored).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `TestServeConfigure_Gate`, `TestConfigure_Gate`, `TestValidateConfigEditing`.
2. PASS: Configure screens for every area; e2e opens Configure.
3. PASS: e2e draft survives reload, Discard all; `configDraft.test.ts`.
4. PASS: `changes.test.ts`, review drawer.
5. PASS: `TestApply_UnchangedTreeIsByteIdentical`, `TestApply_EditsTouchOnlyTheirLines`,
`TestConfigure_RenameSwitchesServer` (added property served at once),
`TestConfigure_NewTypeSurvivesRestart`.
6. PASS: `TestConfigure_RemoveUsedOptionMigrates` (file, applied.json, value, audit).
7. PASS: `TestConfigure_RenameSwitchesServer`, e2e property rename.
8. PASS: `TestSave_Refusals`; problems shown per screen (`problemsFor`).
9. PASS: `TestSave_Conflict`.
10. PASS: `TestCheckEdit*`, `TestMergeKeysAreRefused`, `TestSave_Refusals` (acl_reference).
11. PASS: `TestSave_PrepareFailureRestoresFiles`, `TestSave_MigrationNotStartedRestoresFiles`,
`TestSave_MigrationFailureRollsForward`.
12. MOVED: the migration history view is backlogged as TKT-W833MJ.
13. PASS: every preview uses `ConfigPreview`, labelled Preview.
14. PASS: audit on every save and refusal with principal, hashes and changed
paths (`TestChangedPaths`, service tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-L3R1EP

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: minimal commit messages are the project owner's standing rule)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: the PR is opened with `/pr` after this ticket is done.)

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
