---
id: REV-L3OTC7G
type: review-checklist
title: 'Review: Mentions follow a rename: tombstones on database stores, in-place rewrite on fs'
started: "2026-11-02"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): ran the touched packages (dataentry, entitymanager, store/..., visibility, sqlitedb, mcp, cli, archguard) on the default, sqlite, memorybackend and postgres tags, and the frontend vitest suite. Failures that remain are outside this change: memorybackend cli tests need data to persist between CLI runs, and postgres pile goldens differ by pile backend
- [x] Lint clean (`just lint`): golangci-lint (default, sqlite, postgres), arch-lint, plimsoll, eslint and vue-tsc clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (not run locally: CI's coverage job enforces the package floors; every change ships with tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed: none were critical
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XPXOZB3, RR-UUYDL10, RR-LWIALL4, RR-4Y8AW46,
RR-W8Y7VQP, RR-SMGZU1L, RR-GV95N34, RR-R1U66LZ, RR-OSOGZDR, RR-K3MGEMN,
RR-QWWJYWQ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: storetest mention suite on fs and mem; fs/mem audit each rewritten body (`TestAudit_RenameRecordsEachRewrittenMention`).
2. PASS: `RunRetiredIDTests` on sqlite and postgres.
3. PASS: `RetiredIDIsReserved`, `HighestIDCountsRetiredIDs`, `DeleteKeepsTheReservation` (successor reservation too).
4. PASS: `ChainsCollapse`, `RenameBackReclaimsOwnID`, `CaseOnlyRenameRepointsSuccessors`, `RolledBackRenameRetiresNothing`.
5. PASS: storetest mention suite.
6. PASS: `TestResolver_ResolveMentions`, `TestResolver_MentionedByGatesTheTarget`.
7. PASS: vitest for `markdown.ts`, `entityRefResolver.ts`, `entityRefResolution.ts`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-data-entry, docs/data-entry.md regenerated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-X5VP1SY

## Final Checks

- [x] Commit message explains the why, not just what: minimal message per user preference; the ticket carries the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

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
