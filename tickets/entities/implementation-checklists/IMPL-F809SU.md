---
id: IMPL-F809SU
type: implementation-checklist
title: 'Implementation: Require BaseDir on git.Clone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Two guards in `containedPath`, plus two fixes in the desktop caller that review
turned up (see the review-responses).

The core change is one branch moved and inverted. `containedPath` had:

```go
if base == "" {
    return abs, nil   // skip containment
}
```

sitting *after* the path was cleaned. It now sits at the top and returns
`errors.New("clone base directory is required")`.

The tests were written FIRST and confirmed failing against the unfixed code —
that is what establishes the defect is real rather than theoretical.

Both tests go through the exported `Clone`, not `containedPath` directly. The
defect is that the BOUNDARY fails open, so a test that does not cross the
boundary would not have been testing the claim in the doc comment.

A second guard was added after review: a base that cleans to the filesystem root
is also refused. `containedPath("/", "/etc/passwd")` passed -- `Rel` gives
`etc/passwd`, no `..` -- so a root base was containment-shaped code that checked
nothing, the same silent no-op as the empty base reached by another route
(RR-Q5N3CJ).

Two caller-side fixes, both found in review and both the same shape: the
containment check was correct, and something around it undid the benefit.

- `cmd/rela-desktop` cached `lastCloneDir` BEFORE `Clone` validated it, so a
rejected traversal left the escaping path in app state where `InitRelaProject`
would later `MkdirAll` it. The guard held; the value it rejected walked around
it (RR-HLWRMK).
- `GetDefaultCloneDir` discarded `os.UserHomeDir`'s error and returned the
RELATIVE path `"rela-projects"`, which resolves against the process cwd.
Containment still passes -- it is a real base -- so the guard is satisfied while
the clone lands somewhere the user never chose. That is the CWD-default
behaviour this ticket's own doc comment argues against, reintroduced by a
dropped error (RR-L3CC5O).

Edge cases from planning, all covered: empty base with a valid path; empty base
with a traversal path; base set with a path inside; base set with a path
escaping / equal to base / unrelated absolute (the last three by the
pre-existing table test, which still passes unmodified).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`t.TempDir()` for paths, matching the file's existing tests. Each test specifies
only what it is about: `TestClone_RejectsEmptyBaseDir` uses a path that would be
perfectly valid under a real base, so the failure is attributable to the missing
base and nothing else.

Both tests assert on the error TEXT, not merely `err != nil`. That is
load-bearing here: `Clone` fails for several environmental reasons in a test run
(no network, not a real repo, path exists), so a bare non-nil check would pass
against the UNFIXED code by accident and the suite would look green while
proving nothing.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| step | expected | observed |
| --- | --- | --- |
| new tests against UNFIXED code | both fail | `TestClone_RejectsEmptyBaseDir` and `..._DoesNotSilentlyAllowTraversal` FAIL |
| after the fix | all green | `ok github.com/Sourcehaven-BV/rela/internal/git` |
| mutation: restore `if base == "" { return abs, nil }` | exactly the two new tests redden | both FAIL, the four pre-existing containment cases stay green |
| mutation: remove the root-base guard | `TestContainedPath_RejectsRootBase` reddens alone | FAIL, alone |
| mutation: delete the `os.Stat` branch from `Clone` | `TestClone_PathExists` reddens | FAIL (it did NOT, before RR-BPGG9C) |

The mutation reddening *only* the two new tests is the useful signal: it shows
they isolate the empty-base hole rather than re-testing containment in general,
which the existing table test already covers.

The third row is the one to read. Making `BaseDir` required silently gutted
`TestClone_PathExists`: it constructed `CloneOptions` without a base, so it
began failing at my new guard and never reached the `os.Stat` check it exists to
cover -- while still passing, because its assertion was a bare `err == nil`. The
`os.Stat` guard had zero coverage and the suite was green (RR-BPGG9C).

That is the same failure mode I had reasoned about explicitly for my own new
tests, one screen further down the same file. The generalizable lesson:
tightening a precondition relocates the failure point of every existing test
that constructs the changed type, so those tests need re-READING, not just
re-running.

No test in the package now touches the network. The positive containment case
used to drive a real `Clone`, which -- containment having passed -- proceeded to
an actual unauthenticated fetch against github.com on every run (RR-6SL5UT). It
now calls `containedPath` directly: 0.21s to 0.00s, and no dependence on whether
the runner has egress.

Caller audit: `grep` for `git.Clone(` / `CloneOptions{` across `internal/` and
`cmd/` finds exactly one call site (`cmd/rela-desktop/main.go:364`), and it
already passes `BaseDir`. So the field becoming required breaks nothing today —
which is the argument for doing it now rather than after a second caller exists.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The guard lives in `containedPath`, the function that owns the invariant, rather
than in `Clone` before the call. A future test or call site reaching
`containedPath` directly then gets the same answer — one place to be right.

DRY: nothing to extract. This deletes a branch rather than adding code.

Security: this IS the security fix. It closes a fail-open on a boundary whose
documented purpose is protecting the caller who forgets — `storeCredentials`
writes a plaintext OAuth token under the clone path, so a traversal is a
credential disclosure, not just a misplaced directory.

The reasoning for *why required* is recorded in two comments — on the `BaseDir`
field and on `containedPath` — because those are the two places someone would
edit when the instinct to make an optional field "friendlier" strikes. Including
why a CWD default was rejected: it would relocate the containment boundary to
somewhere the caller never named rather than enforce it.

No silent failures: the empty-base case now returns an error instead of
returning success with the check skipped, which was the defect.
