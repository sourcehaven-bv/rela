---
id: REV-7SY12K
type: review-checklist
title: 'Review: ACL: world-shaped read grants, state-shaped write grants (Step 3)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)  <!-- full Go suite 97 packages exit 0; frontend 2302 tests; E2E 286+ pass in CI -->
- [x] Lint clean (`just lint`)  <!-- golangci-lint 0 issues -->
- [x] Comment lint gate clean (`just comment-lint`)  <!-- no unresolvable doc links across 12,918 comments -->
- [x] Coverage maintained (`just coverage-check`)  <!-- CI Test job green -->

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

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: superseded by the combined review on PR #1452)
- [x] ~~All critical review-responses addressed~~ (N/A: superseded by the combined review on PR #1452)
- [x] ~~All significant review-responses addressed~~ (N/A: superseded by the combined review on PR #1452)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] ~~Each acceptance criterion tested (reference planning checklist)~~ (N/A: superseded by the combined review on PR #1452)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: superseded by the combined review on PR #1452)

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: superseded by the combined review on PR #1452)
- [x] User-facing documentation updated  <!-- docs/acl-security.md, metamodel.md, cli-reference.md, CLAUDE.md regenerated -->
- [x] ~~Docs-checklist marked as done~~ (N/A: superseded by the combined review on PR #1452)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed  <!-- remaining TODOs are ticket-referenced (BUG-ABXMAV, TKT-R68TV8), not loose ends -->
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI  <!-- PR #1452 -->
- [x] All CI checks pass  <!-- all green except this gate -->
- [x] PR URL documented below  <!-- https://github.com/sourcehaven-bv/rela/pull/1452 -->

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
