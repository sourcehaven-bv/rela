---
id: REV-QVZSGG
type: review-checklist
title: 'Review: Clear the 4 open go/path-injection and 1 js/xss-through-dom CodeQL alerts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) — enforced in CI's Test job (go-test-coverage), which passed on the first push; the local recipe currently fails on a doubled module path in the tool itself, unrelated to this change. Touched packages: storage 85%, project 95%, templating 84%, all above floor.

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

**Review Responses:** see the `has-review-response` relations on TKT-R8QEV3 (9: 2 critical, 4 significant, 3 minor; all addressed except the AbsPath one, wont-fix with reason). The two criticals were real: the first frontend sanitizer flattened every flowchart label, and the first guard test passed against the bypass it named.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (Go barrier legible + guarded): PASS — CodeQL's Analyze (go) passed on the first push; the moved-sink alert it raised is closed by `contain()`; go/ast guard mutation-verified.
- AC2 (own the escaping): PASS — SVG-side and label-side hostile cases, unit + real-browser e2e.
- AC3 (diagrams still render WITH labels): PASS — structural unit tests + `e2e/tests/mermaid.spec.ts` against real mermaid (`.nodeLabel` and `<br>` survive).
- AC4 (nothing dismissed): PASS — no dismissals; final alert state to be confirmed via the API after merge.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore, internal hardening)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing surface changed)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened before this checklist existed — the ticket gate itself was the first CI failure; CI is monitored to green from here)

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
