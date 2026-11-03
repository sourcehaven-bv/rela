---
id: REV-CGR9IJ
type: review-checklist
title: 'Review: List export reads relation columns in one batched query'
started: "2026-10-27"
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

`just ci` (check, coverage-check, build, docs-check) ran on this branch before
commit.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CLGENR (significant, addressed), RR-XHVOQT,
RR-7MUZG0, RR-VHRV6Q, RR-YV5BOF, RR-VX6UD9 (minor, addressed), RR-A2PJ69 (nit,
addressed). The rela-security-reviewer agent reported no security findings: the
batched read feeds the same face-ownership filter and neighbor gate as before,
and a failed read returns the generic `export_failed` error.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: one relation query per export, independent of row count.
  `TestQueryBudget_ListExportIsSizeIndependent` holds 6 reads at 10 and 50
  rows; the postgres demo export dropped from 1,107 to 11 queries.
- PASS: output identical. The demo export body is byte-identical to develop,
  `TestExport_List_RelationColumnsGolden` passed before and after, and the
  golden parity bodies did not change.
- PASS: face ownership and gating unchanged.
  `TestExport_List_FacedRowCarriesOnlyItsOwnEdges` and the existing faced and
  ACL export tests pass.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor, no behavior change)
- [x] ~~User-facing documentation updated~~ (N/A: export output unchanged)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs checklist for a refactor)

**Docs Checklist:** N/A

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
