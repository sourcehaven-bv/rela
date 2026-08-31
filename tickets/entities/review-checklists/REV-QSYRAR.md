---
id: REV-QSYRAR
type: review-checklist
title: 'Review: Document why rela import bypasses transition guards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just comment-lint` no unresolvable doc links across 11461
comments; `just plimsoll` clean; `go test ./internal/importer/` ok. Coverage
unaffected — no code changed.

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

**Review Responses:** none — and this box is checked more weakly than usual, so
read the caveat.

No review agent was run. Several agent invocations in this session died from
transient API errors, and rather than claim a review that did not happen I
verified the thing a reviewer would have been asked to check: whether the
comment's factual claims about OTHER files are true. All four were checked by
grep and recorded in IMPL-YQO4HK with the commands.

That is the right target for this diff specifically. The comment asserts facts
about `cli/create.go`, `cli/restore.go` and `cli/normalize.go`, and a
confidently-wrong comment gets TRUSTED — it is worse than no comment. There is
no logic to review beyond that.

A second pair of eyes would still be worth having on the ARGUMENT, as distinct
from the facts: whether "the guard defends nothing against the only actor who
can reach this code" is the right call. That is a judgement, and it was the
project owner's.

Self-review found no unrelated changes: the diff is one godoc plus ticket
entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | a reader learns why the guards are absent without leaving the file | PASS | the godoc gives all three reasons and the sync divergence |
| 2 | states what would CHANGE the answer | PASS | "a non-CLI caller… should be revisited rather than inherited" |
| 3 | the claims in it are true | PASS | four claims verified by grep, table in IMPL-YQO4HK |

AC3 was the load-bearing one, and it earned its place: the intuitive summary
"the CLI writes directly anyway" turned out to be FALSE. Two of the three other
entity-writing CLI paths go through EntityManager and ARE guarded; the third
cannot change status. Had that gone unchecked, the comment would have justified
a correct decision with a wrong reason.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — this IS the documentation.

No `docs/` change: `rela import`'s user documentation describes what the command
does, not which internal write path it takes. The audience for this decision is
a maintainer or a security reviewer, which is why it belongs in the godoc.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: this ticket exists because the code was silent, not because it
was wrong. The behaviour was already the right one; nothing explained it, so an
external review reasonably read the absence as an oversight. That is the
recurring shape across several findings in this round — a settled decision that
does not READ as settled invites the same question repeatedly.

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
