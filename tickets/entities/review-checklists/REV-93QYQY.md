---
id: REV-93QYQY
type: review-checklist
title: 'Review: Require BaseDir on git.Clone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just arch-lint` clean; `just comment-lint` no
unresolvable doc links across 11461 comments; `just plimsoll` clean; `just
lint-md` 0 issues. `just test` green under `-race -shuffle=on`. Coverage
thresholds satisfied.

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

**Review Responses:** RR-BPGG9C (critical), RR-6SL5UT, RR-HLWRMK, RR-L3CC5O,
RR-Q5N3CJ (significant). All addressed.

I had drafted this section as "no findings". That was wrong, and the critical
one is mine: making `BaseDir` required silently gutted `TestClone_PathExists`,
which constructed `CloneOptions` without a base. It began failing at my new
guard, never reached the `os.Stat` check it exists to cover, and kept passing
anyway because its assertion was a bare `err == nil`. The `os.Stat` guard had
zero coverage and the suite was green.

The lesson generalizes past this diff: tightening a precondition relocates the
failure point of every existing test that constructs the changed type. Those
tests need re-READING, not just re-running — a green suite is exactly what the
failure looks like.

Three of the other four are the same shape as each other, and worth noting
together: the containment check itself was correct and complete every time, and
the defect was next to it.

- `lastCloneDir` was cached BEFORE validation, so a rejected traversal still
reached `MkdirAll` via `InitRelaProject` (RR-HLWRMK).
- `GetDefaultCloneDir` dropped `os.UserHomeDir`'s error and returned a RELATIVE
base, so containment passed against the process cwd — the CWD default this
ticket's own reasoning rejects (RR-L3CC5O).
- A root base passed containment while checking nothing (RR-Q5N3CJ).

Validating at a boundary only helps if the rejected value stops there, and only
means something if the base is real.

RR-6SL5UT is a test-quality finding: the positive case drove a real `Clone`,
which — containment having passed — proceeded to a live unauthenticated fetch
against github.com on every run.

Self-review before the agent review checked three things:

1. **Does any existing caller break?** No. `grep` for `git.Clone(` /
`CloneOptions{` across `internal/` and `cmd/` finds exactly one call site
(`cmd/rela-desktop/main.go:364`), and it already passes `BaseDir`. No signature
changed, so this is source-compatible; a future caller that omits the field now
gets a clear error instead of silent unsafety.
2. **Is the guard in the right place?** It sits in `containedPath`, the function
that owns the invariant, not in `Clone` before the call — so a future test or
call site reaching `containedPath` directly gets the same answer. One place to
be right.
3. **Does the guard run before anything else can mask it?** Yes — it is the
first statement, ahead of `filepath.Abs`. An `Abs` failure on a valid-looking
path would otherwise have produced a different error for the same defect.

All three held. What self-review missed was everything ADJACENT to the check —
the existing test whose failure point moved, the caller that cached before
validating, the default that produced a relative base. I verified the change was
correct and did not ask what else the change disturbed.

One deliberate non-change worth recording: the empty-base check does NOT also
try to guess a base. A CWD default was considered and rejected in planning
because it would contain the clone somewhere the caller never named — relocating
the boundary rather than enforcing it. Both comments say so, at the two places
someone would edit when tempted to make the field "friendlier" again.

No unrelated changes in the diff.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | empty `BaseDir` fails with a required-BaseDir error | PASS | `TestClone_RejectsEmptyBaseDir` |
| 2 | the traversal shape fails on the MISSING BASE specifically | PASS | `TestClone_EmptyBaseDir_DoesNotSilentlyAllowTraversal` |
| 3 | existing containment behaviour unchanged | PASS | `TestClone_RejectsPathEscapingBaseDir` (4 cases) passes unmodified; `TestClone_AllowsPathInsideBaseDir` rewritten off the network per RR-6SL5UT, same assertion |
| 4 | a root base is refused | PASS | `TestContainedPath_RejectsRootBase` (added from RR-Q5N3CJ) |
| 5 | `os.Stat` path-exists guard still covered | PASS | `TestClone_PathExists`, repaired per RR-BPGG9C and now mutation-verified |

Both new tests were written FIRST and confirmed failing against the unfixed
code, so the defect is demonstrated rather than asserted. Mutation check:
restoring `if base == "" { return abs, nil }` reddens exactly those two and
leaves the four pre-existing containment cases green — showing they isolate the
empty-base hole rather than re-testing containment in general.

AC2's assertion on the error TEXT is the load-bearing detail. `Clone` fails for
several environmental reasons in a test run (no network, not a real repo), so a
bare `err != nil` would have passed against the UNFIXED code by accident and the
suite would have looked green while proving nothing.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NDCQA6 (done).

Note for the next reader: `docs/` in this repo is GENERATED from `docs-project/`
entities. Editing the generated file directly fails the Docs CI check — learned
the hard way on the sibling ticket TKT-LVSPSB earlier in this round. Nothing
under `docs/` changed here, so it does not bite this ticket.

No `docs/` change: `internal/git` is internal, the one caller already complies,
and no observable behaviour changes. The documentation that mattered was the
godoc — and it mattered a lot, because the defect was partly a documentation
defect. The old comment promised containment "so a future caller that forgets to
sanitize is still safe" while the code skipped the check exactly when that
caller forgot. Comment and code now agree.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

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
