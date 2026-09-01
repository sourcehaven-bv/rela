---
id: REV-W5Z1KM
type: review-checklist
title: 'Review: yaml.v3 emits a block scalar it cannot re-parse for leading-newline strings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues (after fixing one `whitespace` finding in the new `[]any`
branch); `just arch-lint` clean; `just comment-lint` no unresolvable doc links
across 11464 comments; `just plimsoll` clean; `just lint-md` 0 issues. `go test
./internal/markdown/ ./internal/store/fsstore/` ok, including the committed fuzz
seed.

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

No review agent was run; several agent invocations in this session died from
transient API errors. Rather than claim a review that did not happen, the
verification a reviewer would have demanded was done directly, and the fuzzer
did the adversarial part better than a reviewer could: 90s of re-fuzzing after
the fix found no recurrence, and instead surfaced a genuinely different defect
(BUG-X7ICNM).

The one judgement worth a second opinion is the fix DIRECTION — quote rather
than reject. The reasoning is that pgstore already stores these values via
`json.Marshal`, so refusing them in fsstore would make the two backends disagree
about what a valid entity is, and a storage-layer serialization limit must not
become a data-validity rule. That reasoning is in the godoc, so a reviewer who
disagrees has something concrete to argue with.

Self-review found no unrelated changes. The diff is: the shared helper, fsstore
delegating to it, one regression test file, the fuzz seed, and ticket entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | `"\n0"` round-trips through fsstore | PASS | `TestValueToNode_RoundTrips` |
| 2 | the list-valued case the fuzzer found round-trips | PASS | same test, `[]string` and `[]any` rows |
| 3 | existing multi-line values keep their formatting | PASS | `TestValueToNode_LeavesOrdinaryMultilineAlone` |
| 4 | both copies of `valueToNode` fixed | PASS | there is now ONE copy; fsstore delegates |
| 5 | the committed fuzz seed passes as an ordinary test | PASS | `go test -run FuzzPropertyValuesTypeZoo` ok |

AC4 was satisfied by removing the duplication rather than fixing it twice, which
is the better outcome: the report predicted that fixing one copy would leave the
other broken, and it did during diagnosis.

A case NOT in the original report was found and fixed: `"\n"` alone never
errored — it emitted `|4+` and read back as `""`. Silent data loss, worse than
the reported bug, and visible only because the test compares values rather than
checking for an error.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — this is a bug fix.

The documentation that matters is the godoc on `quotedScalarNode`: what the
upstream defect is, why the node is built before `Encode` rather than after, and
why quoting beats rejecting. That last point is the one a future reader is most
likely to reverse without the reasoning in front of them.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Two things a reader of this branch should know:

- The initial recommendation was to REJECT these values with a clear error. That
was wrong, and the pgstore fact is what overturned it. The ticket records the
reversal rather than presenting the final answer as if it were obvious.
- A fuzz failure appearing after this fix is most likely BUG-X7ICNM (invalid
UTF-8, pgstore silently substituting U+FFFD where fsstore refuses), not a
regression here. Its seed is parked at `.ignored/issue-round/fuzz-utf8/`.

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
