---
id: REV-TQEMN1
type: review-checklist
title: 'Review: Remove sidebar entity counts (badges, ACL-scoped counting path, docs)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [ ] All tests pass (`just test`)
- [ ] Lint clean (`just lint`)
- [ ] Comment lint gate clean (`just comment-lint`)
- [ ] Coverage maintained (`just coverage-check`)

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

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [ ] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [ ] Each acceptance criterion tested (reference planning checklist)
- [ ] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [ ] Docs-checklist created and linked via `has-docs`
- [ ] User-facing documentation updated
- [ ] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [ ] Commit message explains the why, not just what
- [ ] No TODOs or FIXMEs left unaddressed
- [ ] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
