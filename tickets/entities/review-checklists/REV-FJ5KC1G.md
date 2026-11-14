---
id: REV-FJ5KC1G
type: review-checklist
title: 'Review: Gantt read grows faster than the data on postgres'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide tests are not run locally by instruction; the touched packages pass with `-race`: pgstore with `-tags postgres`, dataentry gantt and golden tests on default, sqlite and postgres, tools/scalerun. CI runs the full suite)
- [x] ~~Lint clean (`just lint`)~~ (N/A: repo-wide lint is not run locally by instruction; golangci-lint is clean on pgstore and dataentry with `-tags postgres`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs the repo-wide test run; the change adds tests and removes none, CI checks the floors)

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
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run directly on the diff)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4M70CBG, RR-5PJYOQY, RR-4VP4TED, RR-WRUET9F,
RR-8GN4PJ8, RR-430N79W, RR-JTMB50H, RR-40U6WLA, RR-FNDDY95, RR-PAPZI8X,
RR-2VJXHQL, RR-7M2ZOHQ. All minor or nit: ten addressed, one deferred to
TKT-JDJ9VHR, one wont-fix.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS. `TestListRelations_LargeIDBatchPages`: 600 + 1,200 ids, 1,200 rows,
one statement. A batch of 500 ids or fewer keeps the fixed page and the cached
statement, so it is one statement while its result fits one page.
2. PASS. `TestListRelations_LargeIDBatchIsNotPrepared`: no prepared statement
left after a 600-id batch; the 10-id control leaves one.
3. PASS. `TestGanttEdges_OneStatementPerRelationType_Postgres` at 100 and
1,200 edges.
4. PASS. Gantt tests, `TestGantt_ACLRollupExcludesHiddenChild`,
`TestGantt_TruncatedIsPostFilter` and `TestGoldenParity` on default and sqlite,
unchanged goldens.
5. PASS. Local p50 ratio scale 5 / scale 1 is 5.3x for 5x the data (was
about 11x in the CI baseline). `scalerun.yaml` holds it at `ratio_max: 7`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-W7V5PE3

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with `gh pr create` as a stacked PR, and CI is not monitored by instruction)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M.
-->
