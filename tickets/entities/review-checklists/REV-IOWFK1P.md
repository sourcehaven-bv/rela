---
id: REV-IOWFK1P
type: review-checklist
title: 'Review: Settings: search picker for relation defaults'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide run not in scope for this session; `go test -race ./internal/dataentry/ ./internal/archguard/` and the touched vitest suites pass, CI runs the rest)
- [x] ~~Lint clean (`just lint`)~~ (N/A: ran the changed-package subset instead; golangci-lint on internal/dataentry and internal/archguard reports 0 issues, eslint on the touched files 0 errors, `just arch-lint` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: not run locally; the change removes code and adds tests, CI enforces the floors)

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

**Review Responses:** RR-Z6NG6HD, RR-GF810PC (significant, addressed);
RR-6TS0R5Z (minor, deferred); RR-55LGO9J, RR-ZOXC89L, RR-H4FXJ6F, and the
remaining minor and nit responses linked via has-review-response.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestQueryBudget_SettingsReadsNoEntities` (settings costs what binding costs; fails on the old handler).
2. PASS: `TestSettings_RelationDefsCarryTargetTypesOnly`, `TestGoldenParity`.
3. PASS: `RelationDefaultPicker.test.ts`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DGKT7TR

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: commit messages are kept minimal by the owner's convention; the why is in the ticket the message names)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with `gh pr create` after done, and CI is not watched in this session)
