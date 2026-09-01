---
id: REV-BXJ7MV
type: review-checklist
title: 'Review: Operator-configured recipient allowlist for mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/mail/ ./internal/lua/` ok. `golangci-lint` on both packages:
0 issues, after fixing two misspellings and one `testifylint` finding rather
than suppressing them.

One PRE-EXISTING `nilnil` finding in `internal/mail/script.go` is disclosed
rather than fixed here: it predates this branch and belongs to whoever owns that
function, not to a recipients ticket.

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

**Review Responses:** none as entities, but two real findings came out of
self-review and both are recorded in IMPL-BD4ZYD:

**The control was inert.** After the enforcement was complete and the suite
green, grepping for implementations of `RecipientPolicyCarrier` returned
nothing. No sender carried a policy, so production had no allowlist at all —
while every test passed, because each built a policy directly. Fixed by having
`LuaSender` carry it, and pinned by `TestLuaSender_CarriesRecipientPolicy`.

That is the finding worth carrying forward: for a control split across two
packages, "the logic is tested" and "the control is in force" are different
claims, and only the second matters. The seam needs its own test.

**A fail-closed path had no test.** Mutating `recipientPolicyFor` to permit when
a sender declares no policy reddened NOTHING, because every test used a carrying
sender. That is exactly the "test double that never thought about it" case the
design comment names. `TestRecipients_SenderWithoutPolicyDenies` now covers it.

No review agent was run — many agent invocations in this session died from
transient API errors, including this ticket's own implementation agent (twice).
A second pair of eyes is worth having, particularly on the rescope decision.

Self-review found no unrelated changes in the code. The branch does carry
salvaged work from the failed agent runs, committed as work-in-progress and then
completed here.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | absent block denies | PASS | `TestRecipients_UnconfiguredDenies` |
| 2 | literal match allowed | PASS | `TestRecipients_LiteralMatchAllowed` |
| 3 | domain pattern allowed | PASS | `TestRecipients_DomainPatternAllowed` |
| 4 | NOT a suffix match | PASS | `TestRecipients_DomainPatternIsNotASuffixMatch` (2 cases) |
| 5 | allow_any permits everything | PASS | `TestRecipients_AllowAnyPermitsEverything` |
| 6 | case-insensitive | PASS | `TestRecipients_MatchIsCaseInsensitive` |
| 7 | denial does not leak the allowlist | PASS | `TestRecipients_DenialDoesNotLeakTheAllowlist` |
| 8 | non-carrier sender denies | PASS | `TestRecipients_SenderWithoutPolicyDenies` |
| 9 | the operator's block reaches the binding | PASS | `TestLuaSender_CarriesRecipientPolicy` |

AC4 is the security-critical one: a suffix test lets `attacker@evil-example.com`
match `*@example.com`. AC9 is the one that would have been missed — see above.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-A3DEYY (done).

The guide states the deny-on-absence inversion explicitly, because an operator
who has internalised "absent means a sensible default" will otherwise read their
first denial as a rela bug. It also corrects an existing sentence — "a script
cannot reach a destination you did not set up" was true of the transport and
misleading about recipients; it is now true of both.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

This is a BREAKING change: a project that sends mail from Lua must add a
`recipients:` block or its sends will be denied. Waived by the project owner,
consistent with TKT-JVHSOZ. The denial names the missing key, so an operator who
hits it knows what to write.

Scope note for the reviewer: the graph-query form (`person where status =
'active'`) was RESCOPED OUT mid-implementation. It needs `internal/filter`,
which `.go-arch-lint.yml` withholds from `internal/mail` deliberately — a design
question rather than an import fix. The domain form covers the actual threat,
which is a script mailing an attacker-chosen address.

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
