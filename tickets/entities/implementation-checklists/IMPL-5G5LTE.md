---
id: IMPL-5G5LTE
type: implementation-checklist
title: 'Implementation: yaml.v3 emits a block scalar it cannot re-parse for leading-newline strings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`quotedScalarNode` in `internal/markdown/parser.go` builds a double-quoted
scalar node for values yaml.v3 cannot round-trip, and reports whether it did.
`valueToNode` consults it first, falling through to `Node.Encode` otherwise.

Built BEFORE `Encode` rather than fixing up its result. The first attempt
post-processed the returned node and failed identically — `Node.Encode`
round-trips internally and returns the error itself, so there is no node to fix.

Handles the shapes the fuzzer reaches: bare `string`, `[]string`, and `[]any`
containing strings (a breaking value can sit inside a list, which is where it
was found). Values that do not break fall through untouched.

The duplication is REMOVED, not doubled. `valueToNode` existed in both
`internal/markdown/parser.go` and `internal/store/fsstore/markdown.go`; fsstore
now delegates to an exported `markdown.ValueToNode`. arch-lint already permits
that dependency and fsstore already imported the package, so this cost nothing
and closes the failure mode the report warned about.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`TestValueToNode_RoundTrips` is table-driven over all six characterized shapes
plus two list variants, and asserts marshal → unmarshal → **compare**.

Comparing values rather than checking for an error is the load-bearing choice.
Before the fix `"\n0"` FAILED to write, which was the safe outcome; a change
that stopped the error while still emitting unreadable YAML would have turned a
loud failure into a corrupt file on disk. A write-only assertion would have
called that a pass. It is also the only reason the `"\n"` case was found: that
one never errored, it silently returned `""`.

`TestValueToNode_LeavesOrdinaryMultilineAlone` guards the other direction —
quoting everything multi-line would churn the on-disk formatting of every
existing entity whose property spans lines.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| step | expected | observed |
| --- | --- | --- |
| probe all six shapes BEFORE the fix | three fail | `"\n0"` and `"\nx"` error; `"\n"` silently returns `""` |
| same probe AFTER the fix | all round-trip | PASS |
| the original crashing fuzz seed, committed to `testdata/fuzz/` | passes | ok |
| 90s re-fuzz after the fix | no recurrence of this defect | none — surfaced a DIFFERENT one (BUG-X7ICNM) |
| ordinary multi-line string | keeps block style | not quoted |

Gates: `just lint` 0 issues (after fixing one `whitespace` finding in the new
`[]any` branch), `just arch-lint` clean, `just comment-lint` no unresolvable doc
links across 11464 comments, `just plimsoll` clean, `just lint-md` 0 issues. `go
test ./internal/markdown/ ./internal/store/fsstore/` ok.

The re-fuzz row is worth reading. It found `"\n\xc80"` — leading newline plus an
invalid UTF-8 byte — which is NOT this bug: fsstore correctly refuses invalid
UTF-8 as unrepresentable in YAML, while pgstore silently substitutes U+FFFD and
reports success. Filed as BUG-X7ICNM with its seed parked rather than committed,
since alone it is a deliberately-failing test. Recording it here because a
reader seeing a fuzz failure after this fix should know it is a separate,
already-tracked defect and not a regression.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: this diff REDUCES duplication rather than adding any — one shared
implementation replaces two copies that had already drifted in exactly the way
the report predicted.

No silent failures, and that phrase carries unusual weight here: the entire fix
is about NOT converting a loud failure into a silent one. Every case now either
round-trips exactly or errors; nothing is written that cannot be read back.

The godoc records why quoting beats rejecting — that refusing would make fsstore
and pgstore disagree about what a valid entity is, and a storage-layer
serialization limit must not become a data-validity rule. That reasoning is the
part a future reader is most likely to reverse without it.
