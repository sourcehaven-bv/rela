---
id: REV-DUURED
type: review-checklist
title: 'Review: seqtrace: sequence diagrams of traced request flows'
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

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent; rela-security-reviewer ran alongside)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 17, all `addressed`: RR-0YRJE4, RR-DZTPA9, RR-OOFJMO,
RR-ELUVJB, RR-JGD9QQ, RR-QDO1NM, RR-JKYSQ4, RR-K846NK, RR-1OXIDN, RR-J41O57,
RR-JHE7BW, RR-AXPWHL, RR-XONFGQ, RR-QQ1NGR, RR-GRSTEK, RR-Z4X674, RR-B3JC2L.
Self-review found one unrelated-looking change that is required:
`internal/acl/facelessguard_test.go` now skips `.ignored/`, because the guard
walked seqtrace's source copies there and failed locally.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `TestFile_KeepsLineNumbersAndDirectives`.
2. PASS: `TestDemo`.
3. PASS: `TestRender`, `TestRenderValues`, `TestLabelOneLine`.
4. PASS: `TestSummarize`, `TestSummarizeArgs`, `TestSummarizeResults`.
5. PASS: `just seqtrace-demo` rerun after review fixes: ten scenarios with the
   expected codes; trace mode 0600; the redacted salary value absent from the
   trace; server, postgres and container gone afterwards.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QFITZH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (in progress; see the note below)

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
