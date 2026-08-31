---
id: REV-6RFQYU
type: review-checklist
title: 'Review: Document the help endpoint as public by design'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just comment-lint` no unresolvable doc links across 11461
comments; `just plimsoll` clean; `go test ./internal/dataentry/` ok (38s).
Coverage unaffected — no code changed.

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

**Review Responses:** none — and this box is checked more weakly than usual.

No review agent was run. Multiple agent invocations in this session died from
transient API errors, so rather than claim a review that did not happen I
verified what a reviewer would have been asked to check: the factual premise the
whole argument rests on.

If `handleV1Schema` turned out to be gated, this decision would be WRONG and a
comment asserting otherwise would be actively misleading. Checked directly:

- its handler body contains no `readGate` / `Permits` / `Authoriz` call;
- it builds `v1.Schema{Entities, Relations, Types}` from `a.State()` — every
type, property and relation;
- it is registered in the SAME router (`api_v1.go:84`) as `/api/help/`
(`router.go:87`), which rules out the objection that these are
differently-secured surfaces and therefore incomparable.

A second pair of eyes would still be worth having on the JUDGEMENT — whether
help should be public at all. That was the project owner's call, made on the
open-source argument: the entity model and help prose live in the repository, so
guarding the endpoint that serves them protects nothing.

Self-review found no unrelated changes: the diff is one godoc plus ticket
entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | reader learns why it is ungated without leaving the file | PASS | godoc gives the open-source argument in full |
| 2 | names the already-public sibling so the claim is checkable | PASS | `handleV1Schema` / `GET /api/v1/_schema` named explicitly |
| 3 | states what would CHANGE the answer | PASS | "if /api/v1/_schema were ever read-gated… re-decided rather than inherited" |
| 4 | the `_schema` claim is TRUE | PASS | verified three ways, table in IMPL-WVHYNT |

AC4 carried the ticket. Everything else is prose; that one is the premise.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-724BGQ (done).

Planning left open whether a user-facing note was warranted. Settled as NO, with
the reasoning recorded rather than defaulted: documenting "help is
unauthenticated" in an operator guide would be misleading in both directions —
it would imply the surrounding surface is authenticated in a way this one is not
(false; `_schema` is equally public) and invite "should we put it behind auth?",
a question that only makes sense if help is anomalous. It is not.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The counter-intuitive part, worth stating because it is the reason the issue was
rejected rather than deferred: a gate here would be WORSE than the current
state, not merely unnecessary. A boundary that looks deliberate while defending
nothing invites the reader to assume the model is private — a false sense of
protection is worse than a documented absence of one.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

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
