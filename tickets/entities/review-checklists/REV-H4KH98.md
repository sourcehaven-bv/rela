---
id: REV-H4KH98
type: review-checklist
title: 'Review: Scope the timing claim in the ACL guide to entity-level filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint-md` 0 issues. `just docs` regenerated cleanly, so the generated
`docs/acl-security.md` matches its `docs-project/` source — that consistency is
what the Docs CI check compares, and getting it wrong has already failed CI
twice in this session.

No code changed, so the Go gates and coverage are unaffected by this diff.

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
verified the premise a reviewer would have challenged.

The correction asserts that rela has a central enforcement point for entity
access and that field values reach a caller only through the redacting path.
That claim carries the whole "strength, not shortcoming" framing — if it were
false, the passage would be self-congratulatory rather than accurate. Checked:

- `internal/dataentry/visiblereader.go` states in its own doc that it holds the
store privately and exposes only gated reads, precisely because
gate-by-convention was the read-ACL bug class (TKT-N26KLB, #1010).
- Exactly one non-test `a.store.GetEntity` remains on the HTTP path
(`api_v1.go:2326`), and it reads an entity only to compare its TYPE for a
document-kind mismatch, returns no properties, and is followed immediately by an
ACL gate.

Had that grep returned several ungated reads, this would have become a different
ticket.

A second pair of eyes is still worth having on the TONE — whether the passage
lands as an honest scoping rather than either an admission or a boast. That is a
judgement, and it is the thing this kind of correction most easily gets wrong.

Self-review found no unrelated changes: the diff is one guide passage plus its
regenerated output and the ticket entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | distinguishes row filtering from field redaction | PASS | "That last point is about rows, not fields" — the original claim is retained and scoped |
| 2 | does not overclaim in either direction | PASS | "not constant-time" (precise) rather than "vulnerable to timing attacks" (alarmist) |
| 3 | explains why the trade is right | PASS | per-principal conditional grants would need reimplementing in SQL per backend, against a microsecond signal |
| 4 | the claims about the code are TRUE | PASS | verified three ways, table in IMPL-UMIBIP |

AC2 was the hardest to hit and is the reason this ticket needed prose rather
than a one-line edit. The original text overclaimed; the obvious correction
underclaims. Both failures make the doc less useful than saying nothing.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MZH232 (done).

Notable: the original sentence is KEPT rather than replaced. It is true about
rows, and deleting it would have lost a real property (pgstore pushes
entity-level filtering into the query) merely to avoid stating a limit.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Recurring shape worth noting across this round: several findings came from a
settled decision that did not READ as settled. This one is the variant where the
docs said something slightly stronger than the code delivers — same failure
mode, opposite direction, and the fix is the same kind of prose.

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
