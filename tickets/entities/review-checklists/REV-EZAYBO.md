---
id: REV-EZAYBO
type: review-checklist
title: 'Review: Traverse condition: predicate expressions on view traverse steps'
started: "2026-10-15"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test -count=1 ./internal/...` passes; `./...` adds only `cmd/rela-desktop` TestChromeStyle_TargetsShippedClasses, which needs the built SPA and fails the same on develop in this worktree)
- [x] Lint clean (`just lint`) (golangci-lint on the four touched packages: 0 issues; go vet, go-arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (not run locally; CI runs it)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a
merge gate, but a finding your diff *introduces* should be fixed or
suppressed — don't grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels
with the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer on the full diff)
- [x] All critical review-responses addressed (RR-H3L0W5: foreign properties are nil-typed)
- [x] All significant review-responses addressed (RR-J1NAZL, RR-H1PU3U, RR-C635JJ addressed; RR-EM1W9I deferred with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-H3L0W5 (critical, addressed), RR-J1NAZL, RR-H1PU3U,
RR-C635JJ (significant, addressed), RR-EM1W9I (significant, deferred:
per-request compile is shared with lists), RR-HJY2WF, RR-P3DZW8, RR-7CTA5B,
RR-3IMQ7L, RR-WGFTBY (minor, addressed), RR-PRO7ON (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (PLAN-F9JT70)
- [x] Test evidence documented in implementation checklist (IMPL-QYOMDJ)

**Acceptance Status:**
1. PASS: only finished tasks (TestCompileViewConditions_Traverse; "compound" case in TestExecuteViewWithCondition).
2. PASS: unfinished tasks and projects (TestCompileViewConditions_Traverse; "disjunction over types").
3. PASS: foreign property reads nil for equality, ordered use refused at load (TestEvaluator_CompileForCollection).
4. PASS: `where:` unchanged and ANDed (TestExecuteViewWithWhere, "ANDed with where", "where alone still works").
5. PASS: load errors (TestCompileViewConditions_Traverse, TestValidateConfig_TraverseWhereMustParse, manual `rela validate` on the atlas config).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0N16TD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
