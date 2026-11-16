---
id: REV-P7MNSEW
type: review-checklist
title: 'Review: Push ingest find and rename dry-run lookups into the store'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: machine-load limit; touched packages run instead: store conformance on all four backends, visibility, entitymanager, ingest, subscription, scheduler, archguard, dataentry webhook/golden/rename, all with -race where untagged)
- [x] Lint clean (`golangci-lint --allow-serial-runners` on touched packages, default, postgres and sqlite tags)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs the repo-wide test run; new code is covered by the added tests and CI enforces the floors)

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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer agents)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (RR-FNM685Y deferred to TKT-S7KB1WU with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SSG9R4V, RR-LSBD0CF, RR-8UDAF03, RR-FNM685Y,
RR-ZB5N1ZF, RR-0I7BD2S, RR-BU24VR1, RR-Z9TF4KU, RR-Z02O0XX

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestHolderLookup_FirstEqualCostIsScaleFree.
2. PASS: TestHolderLookup_FirstEqual on memstore, sqlite, postgres.
3. PASS: TestRenameDryRun_CostIsScaleFree, TestRenameDryRun_FoldedCollision.
4. PASS: storetest Query/IDTakenFolded and SoftDelete on all four backends.
5. PASS: TestNoNewUnpagedReads with both entries removed.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal performance change, no user-facing behaviour)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened after done with gh pr create; CI is not watched in this task)

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
