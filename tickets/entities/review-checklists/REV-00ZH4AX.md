---
id: REV-00ZH4AX
type: review-checklist
title: 'Review: Flows M1: run flows without channels'
started: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./...` passed; flowcfg, cli and metamodel re-run after the review fixes)
- [x] Lint clean (golangci-lint on every changed package: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (flowcfg 84.6%, floor 78 added in .testcoverage.yml; CI runs the full check)

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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-9T4G21Q, RR-JIX6EZR (design); RR-4RX3VMA, RR-ERYWEJN, RR-HCV3XAG, RR-U5MQYC9, RR-GALL4FA, RR-EIRWL5J (significant, addressed); RR-2W3F8AU, RR-JXKKQCO, RR-UC53M6X, RR-T2YJ92R, RR-FKCAQNB, RR-F5WMM33, RR-0EBEM3X, RR-9J8O3RB, RR-BROVIXK, RR-AI4KYQC, RR-H3E4LJC, RR-UJZMNBT (addressed); RR-XC2WBH9, RR-EMIUGI8 (deferred to TKT-9IT3LN8); RR-6OEYLW8 (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1 PASS (TestParse_Sample); 2 PASS (TestParse_Problems); 3 PASS (TestParse_Version); 4 PASS (TestParse_ReservedPrefix, TestIsReservedName); 5 PASS (TestFlowShow_Write, TestValidate_Flows).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-SPC0YDH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
