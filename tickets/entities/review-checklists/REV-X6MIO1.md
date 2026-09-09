---
id: REV-X6MIO1
type: review-checklist
title: 'Review: FuzzCloneNestedValues reports correct property-key refusals as crashes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` 0 failures. `just lint` 0 issues. `just comment-lint` clean across
13903 comments. `just coverage-check` PASS on both thresholds, total 79.3%. Also
`just arch-lint` OK and `go build -tags sqlite ./...` clean.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

The review noted the sign-bug explanation now appears in both this target and
`FuzzPropertyValuesTypeZoo` — a `duplication` shape. Left as-is: the two
comments explain the same Go idiom at two independent call sites, and the
advisory rule's prescribed remedy (hoist to a shared site) would mean inventing
a helper for a two-line modulo expression, which CLAUDE.md's "three similar
lines is better than a premature abstraction" argues against. Advisory only,
does not gate.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0DLHGU (significant, addressed)

One significant finding, verified independently before acting on it and then
fixed: the first version copied TypeZoo's `ValidateProperty` skip without
re-checking its premise, which silently dropped live clone-path inputs. Two
minor comment-accuracy findings were also fixed (the `Clone()` rationale and the
"same defect TypeZoo fixed" phrasing). A minor finding about pgstore lacking a
seed was addressed by adding one.

Diff is four files plus four seeds, all on this change — no unrelated edits.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Sweep-reported crash no longer reported on any backend — PASS. All four
backends (fsstore, memstore, sqlitestore, pgstore) run the committed seeds
green.
- Fix is load-bearing, not a silenced assertion — PASS. Reverting the source
while keeping the seeds fails; restoring it passes.
- Target not made vacuous — PASS. Probe confirms `""` and `"a/b"` are accepted
by the store and reach the clone assertions, coverage the first version
discarded.
- No new crashers under active fuzzing — PASS. 25s per backend after the fix,
nothing written to any corpus.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: test-harness
fix, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: no behaviour change to any
shipped code path)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
