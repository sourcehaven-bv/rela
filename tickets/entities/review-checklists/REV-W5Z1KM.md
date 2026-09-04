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

**Review Responses.** The first version of this checklist ticked the
`/code-review` box while the text beneath admitted no review agent had run
(transient API errors). The IB review of PR #1505 flagged that as a Low finding:
a checked box that the prose contradicts is not evidence. The review was run on
2026-09-04 (cranky-code-reviewer against the branch diff) and the box now means
what it says. It was worth running: the reviewer found a hole the fuzzer had
not reached in its 90s.

| # | severity | finding | response |
| --- | --- | --- | --- |
| 1 | critical | `map[string]any{"v": "\n0"}` still wrote `"0"` to disk with no error; the fix handled only top-level strings and flat lists, while the store fuzz target already generates this shape (`case 5`). | Fixed. `ValueToNode` now walks every container shape a property value can take (`[]string`, `[]any`, `map[string]string`, `map[string]any`, nested) and quotes breaking strings wherever they sit. `TestValueToNode_RoundTrips` covers the nested cases. |
| 2 | critical | The `[]any` branch swallowed `Encode` errors (`return nil, false`) and fell back to the buggy path, so a breaking string inside a nested container hard-failed. | Fixed. The walk returns `(*yaml.Node, error)` throughout; nothing is swallowed. |
| 3 | critical | `needsQuoting` was under-inclusive: `"\tx\ny"` errored on read (tab in the indentation column), and the comment claiming only a leading newline breaks was wrong. | Fixed, and the claim is now backed by fuzzing yaml.v3 directly in nine nesting contexts. That found a THIRD shape the reviewer had not: a multi-line string starting with a space breaks only under a sequence (`- ` above it anywhere), so `needsQuoting` takes the context in rather than quoting it in map context too, which would reflow existing files. |
| 4 | significant | `ValueToNode` was a pure alias for `valueToNode`. | Fixed. One exported function; the alias is gone. |
| 5 | significant | Tests mirrored the implementation's blind spot: nothing nested. | Fixed. Nested map, map-in-list, list-in-list, string-map and mixed-type list cases; plus `FuzzValueToNode`, which round-trips any string at every nesting position. |
| 6 | nitpick | `LeavesOrdinaryMultilineAlone` asserted a style flag, not the on-disk bytes. | Fixed. Asserts the emitted YAML is a block scalar. `TestValueToNode_MatchesEncodeWhenNothingBreaks` further pins byte-equality with plain `Encode` for every non-breaking value, including yaml.v3's numeric-aware map key order. |
| 7 | nitpick | `[]string` and `[]any` branches duplicated loop logic. | Fixed by the recursive walk. |

The reviewer agreed with the quote-rather-than-reject direction, which was the
judgement flagged as most wanting a second opinion.

Self-review found no unrelated changes. The diff is: the shared encoder, fsstore
delegating to it, one test file with a fuzz target, the fuzz seed, and ticket
entities.

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
