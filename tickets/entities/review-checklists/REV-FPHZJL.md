---
id: REV-FPHZJL
type: review-checklist
title: 'Review: pgstore silently substitutes U+FFFD for invalid UTF-8 where fsstore refuses'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues after one `gocognit` finding on the first walker was
resolved by splitting it per shape; `just arch-lint`, `just comment-lint`,
`just plimsoll`, `just lint-md` clean. `just test` (race, shuffle) ok with
`RELA_TEST_DATABASE_URL` set so pgstore ran for real. `just coverage-check`
passes.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a
merge gate, but a finding your diff *introduces* should be fixed or
suppressed — don't grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels
with the code and is reviewed in this diff:

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

**Review Responses** (cranky-code-reviewer, 2026-09-04, over this branch's
own diff against the #1505 branch it stacks on):

| # | severity | finding | response |
| --- | --- | --- | --- |
| 1 | critical | pgstore's version writers (`insertVersion`, `insertRelationVersion`) call `marshalProps` with no gate. Reachable from the public `VersionWriter` surface. Version rows are the audit trail — the worst place for a silent substitution. | Fixed by moving the gate INTO `marshalProps` in pgstore and sqlitestore: the one place properties become JSON in each backend, so no marshal site can bypass it. The explicit calls at the four CRUD entry points in those two backends were removed as redundant; fsstore and memstore keep theirs. `TestWriteVersionRejectsInvalidText` covers both version writers (pgstore-only, DB-gated, since no other backend writes versions). |
| 2 | critical | The walker missed `map[any]any`, which yaml.v3 produces for a nested mapping with a non-string key and the importer passes to `CreateEntity` as-is. Not an invalid-UTF-8 vector today (yaml.v3 refuses at parse), but a walker whose contract is "every nesting" silently skipping a container type is how this class recurs. | Fixed; unit cases for a bad value and a bad string key under an any-keyed map. The godoc now says a missing container type is a hole, not a pass. |
| 3 | significant | `normalizeProps` folds nil and empty slices together; `map[string]string` was not normalized. | Folding nil/empty is necessary — no serializing backend can tell them apart, both are written as `[]` — and is now documented as deliberate, alongside what is deliberately NOT folded (missing key, nil value, empty container stay distinct). `map[string]string` added for symmetry. |
| 4 | significant | The fuzz oracle asserted only `assert.Error` on a rejected write; a conflict or I/O failure would satisfy it. | Fixed: `assert.ErrorContains(err, ruleErr.Error())` — the store must have refused for the rule's reason. |
| 5 | significant | NUL rejection is right but is a behaviour change: fsstore/sqlitestore/memstore previously stored NUL, so an existing row with one now fails on UPDATE, far from the cause. | Agreed. Recorded on the bug entity under "Behaviour change" and in the PR description. |
| 6–9 | nitpick | Validate-before-lock is correct; error wrapping is consistent per backend; `frontmatter.Split` change verified safe for all three callers and CRLF; the modulo fix is correct. | No change. The `sqlitestore: create: store: property …` double prefix is that backend's pre-existing wrapping style. |
| — | leverage | Three walkers over the property value domain (`storeutil.validatePropertyValue`, `canonical.normalize`, `entity.CloneValue`) disagree about which container types exist — this ticket's bug class one level up. | Out of scope here; filed as TKT-OFQG3J. |

Self-review: the diff is the shared rule and its tests, the gate at each
backend's serializer or write path, the conformance case, the fuzz oracle,
the pgstore version test, the `frontmatter.Split` fix (BUG-NWQA0E) with its
test, one fsstore fuzz seed, and ticket entities. The branch is stacked on
#1505 (`yaml-block-scalar-quoting`); its commits are in this branch's history
but not in this PR's own change set, and the PR description says so.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion (from the bug's Scope) | status | evidence |
| --- | --- | --- | --- |
| 1 | invalid UTF-8 in a property value is rejected at a point every backend shares | PASS | `storeutil.ValidateProperties`; `RejectsInvalidUTF8Properties` green on all four, red on all four with the wiring reverted |
| 2 | the caller gets an error, never a substituted value | PASS | same case checks the stored value is unchanged after each refused write |
| 3 | the parked seed becomes a regression test | PASS | `f.Add("prop", 0, "\n\xc80")` in the shared target, so all four backends carry it |
| 4 | assert a round trip and that the backends agree, not "no error on write" | PASS | fuzz target compares the read-back property map; oracle is directional |
| 5 | fsstore is NOT made to accept it (out of scope) | PASS | fsstore now refuses at the gate with the same error as the others rather than at yaml.v3 |

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Two things a reader of this branch should know:

- The round-trip assertion did most of the work. Every time it was
  tightened it found a pre-existing divergence within seconds; the list is
  in BUGA-YCYAWA. Anyone adding a serialization seam should copy the shape:
  write, read back, COMPARE.
- pgstore and sqlitestore gate inside `marshalProps`; fsstore and memstore
  gate at their write entry points, because they have no single serializer
  (fsstore's YAML path already refused, memstore has none). If a fifth
  backend arrives, the conformance case will say whether it forgot.

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
