---
id: IMPL-33H5RI
type: implementation-checklist
title: 'Implementation: Root-base guard misses the Windows drive root'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The change is one predicate plus its seam:

```go
if isFilesystemRoot(filepath.VolumeName(absBase), absBase, filepath.Separator) {
    return "", errors.New("clone base directory must not be the filesystem root")
}

func isFilesystemRoot(volumeName, cleanedPath string, separator rune) bool {
    if volumeName != "" && cleanedPath == volumeName {
        return true
    }
    return cleanedPath == volumeName+string(separator)
}
```

The second clause was NOT in the plan and was added in review (RR-0DTTMS). The
plan assumed `Clean` always leaves a trailing separator on a root; it does not
when the whole path is the volume (`\\server\share`, `\\?\C:`), and `Rel` then
treats that base as absolute — so the share root was still accepted. See the
mutation notes below.

The error text is unchanged, so the pre-existing `TestContainedPath_RejectsRootBase`
assertion on "filesystem root" still holds — deliberate: a Windows drive root IS
the filesystem root of its volume, and inventing a second message would fork the
caller's error handling for no distinction that matters.

**Deviation from plan.** `separator` was added as a third parameter during
implementation. The plan had the predicate read `filepath.Separator` itself, and
the first test run showed why that fails: `filepath.Separator` is a build
constant, so on Linux `isFilesystemRoot("C:", "C:\\")` compared `C:\` against
`C:/`. Recorded in PLAN-5TC5UQ under Technical Approach, because the wrong shape
was the plausible one and a later reader will consider "simplifying" back to it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`TestIsFilesystemRoot` is a table over `(volume, cleaned, separator, want)`; each
row carries only those four, and the failure message interpolates the row's own
values rather than a literal. `windowsSep`/`unixSep` are named constants so a row
declares which platform it models instead of repeating an opaque `'\\'`.

`clone_windows_test.go` (`//go:build windows`) is the deliberate opposite of the
table: it never writes a cleaned form, it Cleans the input and passes the result
through. That is the point — it cannot inherit a mistaken belief about `Clean`,
which is exactly what the table did for `\\server\share` (RR-0DTTMS). It does not
run on Linux CI, so it does not replace the table; the two are complementary and
both files say so in their comments.

A test was also REMOVED. `TestContainedPath_UsesIsFilesystemRoot` walked
`filepath.Dir` up to the root and asserted `containedPath` rejects it, but on
Linux that root is always `/` — the one case where the old and new checks agree
— so it passed identically against unfixed code (RR-T1XVAX). A comment now sits
in its place explaining why no such Linux test exists, since writing it is the
obvious next move for the next person.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Stdlib claims verified before relying on them.* A scratch program on the host
confirmed `filepath.VolumeName` returns `""` for `/`, `/etc`, `C:\` and
`\\server\share` on darwin, and `internal/filepathlite/path_unix.go` defines
`volumeNameLen` as constant 0. The Windows `Clean` behaviour (volume copied
verbatim, trailing separator retained for `C:\` and `\\host\share\`) was read
from `internal/filepathlite/path.go` and `path_windows.go`. So the table's
Windows rows are derived, not imagined, and "the volume-aware form subsumes the
Unix one" is checked rather than assumed.

*Acceptance criteria.* AC1 (drive root) → rows `windows drive root`,
`windows drive root lowercase`, plus `extended drive volume` / `extended drive
root`. AC2 (UNC share root) → rows `unc share root trailing separator` AND
`unc share root bare volume`; the second is the spelling `Clean` actually
produces and the first implementation missed it (RR-0DTTMS). AC3 (Unix
preserved, by the same predicate) → row `unix root` plus the unmodified
`TestContainedPath_RejectsRootBase`. AC4 (no over-refusal) → the six `false`
rows — including `windows drive relative` (`C:foo`) and `empty path no volume`,
which are what stop the new bare-volume clause over-reaching — plus the
unmodified `TestClone_AllowsPathInsideBaseDir`. AC5 → whole `internal/git` suite
green with `-count=1`.

*Mutation testing.* Four mutations, each verified to have landed in EXECUTABLE
code by re-reading the mutated line and its line number before running — a
mutation that lands in a comment leaves the suite green and proves nothing.

1. *Volume term dropped* — `return cleanedPath == string(separator)`
(clone.go:183). Reddened exactly three subtests:
`isFilesystemRoot("C:", "C:\\", "\\") = false, want true`, the lowercase drive
variant, and the UNC form. Every Unix row and every pre-existing test stayed
green — the correct signal, since this mutation IS the pre-fix code and the
pre-fix code was right on Unix.
2. *Call site disabled* — `if false && isFilesystemRoot(...)` (clone.go:144).
Reddened `TestContainedPath_RejectsRootBase`. This is the mutation a predicate
table alone cannot catch: a correct predicate that nothing calls.
3. *Bare-volume clause disabled* — `if false && volumeName != "" && ...`
(clone.go:194). Reddened exactly `unc share root bare volume` and
`extended drive volume`, and nothing else. This is the mutation that pins
RR-0DTTMS: before that fix, both rows were the failing state of real code.
Re-run after the RR-IX0Q97 row removal, to confirm the narrowed table had not
lost its grip on the fix — it had not.
4. *Non-empty-volume guard dropped* — `if cleanedPath == volumeName`
(clone.go:194). Reddened `empty path no volume`
(`isFilesystemRoot("", "", "/") = true, want false`). This is the over-refusal
direction, and it is the one that proves the fix narrows only to genuine roots
rather than becoming a blanket refusal. Without a negative row it would have
passed silently.

All restored from pre-mutation copies; `go test -count=1 ./internal/git/...`
green afterwards.

*Cross-compilation.* `GOOS=windows go vet ./internal/git/` passes, so
`clone_windows_test.go` stays compiling on CI even though Linux never runs it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The extraction is the one place this could have been over-engineered. It earns
its keep for a reason other than tidiness: it is the only way to execute the
Windows comparison on the runner CI actually uses. A predicate inlined at the
call site would be untestable there, and the alternative — adding a Windows CI
job — is disproportionate to a one-expression fix.

`string(filepath.Separator)` still appears in `containedPath`'s `..` prefix check
below the guard. Deliberately not folded together: that one is about the
separator *inside* a relative path, a different question from "is this the volume
root", and merging them would couple two checks that only look alike.

Security: the change strictly narrows what is accepted, so it cannot admit a base
that was previously refused. The AC4 negative rows are what pin that it narrows
only to roots.
