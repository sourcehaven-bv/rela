---
id: REV-Z8ENUOX
type: review-checklist
title: 'Review: Budget tests count rows read, not only store calls'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (ran go test on every touched package: storetest, elementnaive, acl, schema, search, visibility, lua, mcp, cli, appbuild flow tasks, dataentry; all ok, also under -race (dataentry 543s on a loaded machine after search-backed tests moved to 20 vs 200 padding). Postgres-tagged tests not run: RELA_TEST_DATABASE_URL unset)
- [x] Lint clean (`just lint`) (golangci-lint serially per touched package: 0 issues; just arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A: test-only change; no production code changed, storetest gains tests)

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-GTE28AT, RR-LD5AGY4, RR-8WTN2SO)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GTE28AT, RR-LD5AGY4, RR-8WTN2SO, RR-T65A72P, RR-C21IXY7 (deferred), RR-A7OXS2L, RR-D8W6ZEB, RR-CZFUF00, RR-V9HMQPN, RR-E1E3OSQ, RR-D8RLG7L, RR-AZOV6P0, RR-MG6N3MJ, RR-2N2NGMB, RR-C5W1TJ2, RR-LJ2CVS5, RR-ERHFQ1W

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Counting counts rows as well as calls: PASS (TestCounting_Rows)
- Helper asserts rows equal at 100 and 1,000 entities: PASS (storetest.AssertScaleFree)
- One full-scan call fails the helper: PASS (TestAssertScaleFree_FullScanFails)
- Existing budget tests assert rows: PASS; tests that reveal a real scan stay calls-only with TODO(TKT-FED6FMM)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: test harness only; contributor rule collection-reads.md updated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-13U1JH4

## Final Checks

- [x] ~~Commit message explains the why~~ (N/A: user asked for a minimal commit message; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed (the TODO(TKT-FED6FMM) markers are tracked by that ticket)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command~~ (N/A: user asked not to push or open a PR in this session)

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
