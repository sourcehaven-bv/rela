---
id: REV-2FK0S7H
type: review-checklist
title: 'Review: rela test v1 (3/4): rela test command, rela.test Lua module, seeded fakers and principals'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just coverage-check` run: every package passes except `cmd/rela-desktop` TestChromeStyle_TargetsShippedClasses, which needs the built SPA that this worktree lacks; unrelated to the diff)
- [x] Lint clean (`just lint`: 0 issues on every build tag; `just arch-lint` and `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (floors pass; total 85.5%, apptest 80.5%)

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
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0BMK7CQ RR-0J6ZQ0D RR-1EROMWV RR-20PC78X RR-4GYMWTQ RR-527I4I4 RR-551E324 RR-6ZF7YLK RR-7NGQ1ZM RR-7O9RLLH RR-837KUM6 RR-AB91P36 RR-CQYAQ14 RR-E7UYYO4 RR-GGMO6I2 RR-GRODXA3 RR-K36ZSXH RR-MGXDYCH RR-PIOFDQ7 RR-PMTIQ03 RR-Q12IUYI RR-R9JUHTP RR-RPAPKPJ RR-RSSNPUP RR-U30FQ6D RR-W3C62AR RR-Y9Q57SI. Two minors (AC3 and AC4 coverage) are deferred to TKT-E2IMTMG with a reason; all others are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-KIFFRVI)

**Acceptance Status:** AC1-9 and AC11-16 PASS, each pinned by a test in `internal/apptest/apptest_test.go` or `internal/cli/test_test.go` as listed in IMPL-KIFFRVI. AC3 and AC4 have narrower coverage than planned; see the deferred review responses.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: the guide and CLI reference are part 4, TKT-E2IMTMG)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VAP0I07

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: nothing committed yet; the commit is made when the user asks)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: `/pr` runs after the ticket is done, per TKT-UFV01M)

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
