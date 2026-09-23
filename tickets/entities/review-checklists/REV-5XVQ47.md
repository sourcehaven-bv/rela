---
id: REV-5XVQ47
type: review-checklist
title: 'Review: related(): traverse incoming edges (filter B on properties of A where A -> B)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; timing-sensitive tests in dataentry, jobs,
  validation and docscapture flaked under full-suite load and passed on rerun)
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

**Review Responses:** RR-0YJOPK (critical), RR-2ZYS3G, RR-30PBD2, RR-3QEVFT,
RR-3RIDCH, RR-4TG7CN, RR-6S4YWO, RR-6TZPNA, RR-75477L, RR-S7D39S
(significant), all addressed. Minor and nit: RR-0KM63X, RR-3TOFSO,
RR-48FBVK, RR-721D7X, RR-DQDJAW, RR-DUVVZ1, RR-EZJJAN, RR-HIW9Q3, RR-M25IN1,
RR-PGT1M2, RR-S4FCL3, RR-UAL3GB, RR-V0X3PZ, RR-V22182, RR-WEL2HC, RR-ZY04OK
addressed; RR-31AL2A deferred; RR-PQFKE6 and RR-VEEAZO won't-fix.
Follow-up: TKT-44PVX2 (chained slot collision).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 incoming hop via inverse ID: PASS. `queryscope_traversal_e2e_test.go`
  EndToEnd and NoACL; storetest inbound endpoint-match cases on mem, fs,
  sqlite and pg; manual rela-server run.
- AC2 mixed-direction chains: PASS. storetest
  `chain_outbound_then_inbound`; predicatefns direction tests.
- AC3 load-time refusals (symmetric, empty From, non-string, undeclared
  enum value, hidden field, non-scope surfaces): PASS. predicatefns,
  scopes, conditionlint and `TestQueryScopeTraversalFieldErrors` tests; boot
  refusal verified manually.
- AC4 ACL: hidden far rows never count, unsupported shapes are a 422: PASS.
  e2e interleaved principals, `UnsupportedForPrincipalIs422`, acl gate tests.
- AC5 bounded queries: PASS. `TestQueryBudget_TraversalScopeIsSizeIndependent`
  (6 queries at 10 and 50 rows).
- AC6 index use on postgres: PASS. pgstore EXPLAIN test on the MatchingIDs
  shape shows the derived index, no Seq Scan.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-62Y0ZU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is opened
  after done, on request)

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
