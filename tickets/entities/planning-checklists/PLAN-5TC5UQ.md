---
id: PLAN-5TC5UQ
type: planning-checklist
title: 'Planning: Root-base guard misses the Windows drive root'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `containedPath` refuses a Windows drive root (`C:\`) and UNC share root
(`\\server\share\`) as a clone base, using ONE expression that is also correct
on Unix rather than a platform special case bolted beside the existing check.

OUT, with reasons:

- *Symlink resolution.* Unchanged from TKT-S2SFTG: `containedPath` is
deliberately string-level (Clean + Rel), matching `storage.RootedFS`'s threat
model. Widening that is a different ticket.
- *Windows path normalisation beyond `Clean`* — short (8.3) names, case folding,
`\\?\` extended-length rewriting. `filepath.VolumeName` already treats `\\?\C:\`
as a volume, so its root is refused; going further would need a Windows runner to
verify and cannot be honestly tested on Linux CI. Claiming it without testing it
is exactly the failure mode this ticket is fixing.

**Acceptance Criteria:**

1. A Windows drive root is refused as a base.
*Test:* the root predicate returns true for `("C:", "C:\")`. Must be exercised
ON LINUX CI — see the Test Plan; a test that calls `containedPath("C:\\", ...)`
on Linux proves nothing, because Linux `filepath` sees `C:\` as an ordinary
relative filename.
2. A UNC share root is refused as a base, in BOTH spellings.
*Test:* the predicate returns true for `("\\\\server\\share", "\\\\server\\share\\")`
and for `("\\\\server\\share", "\\\\server\\share")`. **The second was missing
from this plan** and added after review (RR-0DTTMS) — it is the form
`filepath.Clean` actually returns, and the first implementation accepted it,
leaving the fail-open open for shares.
3. The Unix root refusal from #1496 is preserved, and preserved BY THE SAME
expression rather than by a leftover branch.
*Test:* the existing `TestContainedPath_RejectsRootBase` still passes unmodified,
plus a predicate-table row `("", "/")` → true.
4. Non-root bases are still accepted — the fix must not turn the guard into a
blanket refusal.
*Test:* predicate rows for `("C:", "C:\Users\dev")`, `("\\\\srv\\share", "\\\\srv\\share\\repos")`
and `("", "/home/dev")` all return false, and the existing
`TestClone_AllowsPathInsideBaseDir` still passes.
5. Every existing containment behaviour is unchanged.
*Test:* the whole pre-existing `internal/git` suite passes unmodified.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a one-expression change.

**Existing Solutions:**

- No library needed. `path/filepath` already exposes everything required:
`VolumeName` returns the leading volume (`C:` for a drive path,
`\\host\share` for UNC) on Windows and **`""` on every other platform**.
- That last fact was VERIFIED, not assumed: `internal/filepathlite/path_unix.go`
defines `volumeNameLen` as a constant `0`, and a scratch program on darwin
confirmed `VolumeName` is `""` for `/`, `/etc`, `C:\` and `\\server\share`
alike. So `VolumeName(b)+string(Separator) == b` degenerates on Unix to
`""+"/" == b`, i.e. exactly the `b == "/"` check #1496 wrote. The volume-aware
form SUBSUMES the existing one; it does not merely sit next to it.
- The `Clean` half was verified against the stdlib too
(`internal/filepathlite/path.go`): on Windows `Clean("C:\\")` keeps the trailing
separator (volume is copied verbatim, the rooted remainder cleans to `\`), and
`Clean("\\\\server\\share\\")` likewise. So the compared forms genuinely occur
after the `Clean` that `containedPath` already performs.
- In-repo prior art for path containment is `storage.RootedFS`, already cited by
`containedPath`'s comment. This ticket does not change that model.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Extract the predicate into a tiny pure function that takes the values it
compares:

```go
func isFilesystemRoot(volumeName, cleanedPath string, separator rune) bool {
    return cleanedPath == volumeName+string(separator)
}
```

called as
`isFilesystemRoot(filepath.VolumeName(absBase), absBase, filepath.Separator)`.

Extraction is not decoration — it is what makes the Windows branch TESTABLE on a
Linux runner (see Test Plan). The parameters are the seam: a test supplies the
triple a Windows `filepath` would have produced, without needing a Windows
filesystem.

**Correction made during implementation.** The plan first took only
`(volumeName, cleanedPath)` and read `filepath.Separator` internally. That was
wrong, and the test caught it immediately: `filepath.Separator` is a
build-constant, `/` on Linux, so `isFilesystemRoot("C:", "C:\\")` compared
`C:\` against `C:/` and returned false. Two of the three values the predicate
compares are platform-dependent, not one — so a seam that parameterises only one
of them still cannot express the Windows case, and the "Windows" rows would have
had to be written with a Unix separator to pass, i.e. testing a string that
Windows never produces. Passing the separator too is what makes the extraction
honest rather than merely present. Worth recording because the first shape LOOKED
sufficient and would have been kept had the table not been written first.

**Alternatives considered:**

- *Add a second `||` clause beside the existing check* (`absBase == sep ||
filepath.VolumeName(absBase)+sep == absBase`). Rejected: the first clause becomes
dead once the second exists on Unix, so the code would carry a redundant branch
that reads as though it handles a case the other does not. The issue explicitly
suggested "in addition to"; the stronger answer, once `VolumeName == ""` on Unix
is verified rather than assumed, is one expression that is correct everywhere.
- *`runtime.GOOS == "windows"` special case.* Rejected: it hard-codes at runtime
what the build already decided, cannot be exercised on the other platform's CI,
and would need its own dead branch on Linux.
- *Compare against `filepath.Separator` and `os.PathSeparator` both.* Rejected —
they are the same constant; it addresses nothing.
- *Leave it and document Windows as unsupported for clone.* Rejected: the MSI is
shipped (`.github/workflows/release.yml`, `os_name: windows`), so the platform is
supported in fact.

**Files to modify:**

- `internal/git/clone.go`
- `internal/git/clone_test.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `BaseDir` is caller/operator-supplied and mandatory since TKT-S2SFTG. This
ticket adds the second half of "and it must actually constrain something": a base
that contains the entire volume constrains nothing.
- `Path` is caller-supplied and its final segment can derive from a
remote-controlled URL (`ExtractRepoName`). Unchanged.
- The base check is a refusal of one degenerate value, evaluated on the CLEANED
absolute form so that `C:\`, `C:\.`, `C:\foo\..` all reduce to the same question.
The containment check it protects remains an allowlist (one subtree via
`filepath.Rel`), not a blocklist of bad substrings.

**Security-Sensitive Operations:**

`storeCredentials` writes a plaintext OAuth token to `<Path>/.git/credentials`.
That is what makes a defeated containment check a credential disclosure rather
than a misplaced directory, and it is why the guard has to hold on every platform
the binary ships to, not just the one the tests happen to run on.

The error names the base directory, which the caller supplied — no disclosure.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

The load-bearing design decision here is *how to get real coverage of the Windows
branch on a Linux runner*. The naive test — `containedPath("C:\\", "C:\\x")` on
Linux — proves NOTHING: Linux `filepath` treats `C:\` as an ordinary relative
filename with no volume, so `Abs` prefixes the CWD and the assertion passes or
fails for reasons that have nothing to do with Windows. A test like that is worse
than no test, because it reports coverage of a branch it never entered.

So the predicate is extracted to take `(volumeName, cleanedPath)` as plain
strings, and a table test feeds it the pairs a **Windows** `filepath` would
produce. Those pairs are not invented: each is justified above from the stdlib
source (`volumeNameLen` for the volume, `Clean` for the trailing separator). The
predicate is total and platform-independent — string equality — so a Linux run
exercises precisely the code a Windows run would.

Table rows, each mapping to an acceptance criterion:

| volume | cleaned base | root? | AC |
|---|---|---|---|
| `""` | `/` | yes | 3 |
| `""` | `/home/dev` | no | 4 |
| `C:` | `C:\` | yes | 1 |
| `C:` | `C:\Users\dev` | no | 4 |
| `c:` | `c:\` | yes | 1 (lowercase drive letter) |
| `\\server\share` | `\\server\share\` | yes | 2 |
| `\\server\share` | `\\server\share` | yes | 2 — **added in review**, RR-0DTTMS |
| `\\server\share` | `\\server\share\repos` | no | 4 |
| `\\?\UNC\server\share` | same | yes | 1/2 — **added in review** |
| `\\?\C:` | same | yes | 1 — **added in review** |
| `C:` | `C:foo` | no | 4 — **added in review** (drive-relative is not a root) |
| `""` | `.` | no | 4 (relative base, refused later by Rel, not here) |
| `""` | `""` | no | 4 — **added in review** (guards the new clause) |

Plus the platform-native end-to-end case kept as-is
(`TestContainedPath_RejectsRootBase`), so the wiring from `containedPath` into
the predicate is covered on the runner's own platform (AC3, AC5).

**What this plan got wrong, and the limit of the technique.** The table above
originally listed `\\server\share\` only. `filepath.Clean` does NOT append a
trailing separator when the whole path is the volume, so the row asserted a
string Windows never produces while the spelling that does occur went unguarded
— and `filepath.Rel` treats that bare base as absolute, so containment passed for
every path on the share. Found in code review (RR-0DTTMS), fixed by a second
clause in the predicate.

The general lesson, recorded because the plan reasoned right up to it and
stopped: parameterising the predicate makes the Windows branch EXECUTE on Linux,
but every input is still a hand transcription of believed `Clean` behaviour, and
nothing on Linux can check that belief. Executing a branch with inputs the
platform cannot produce is coverage without verification. The answer is a
build-tagged `clone_windows_test.go` that derives its triples from the host
`filepath` instead of from memory (RR-S2X70O) — it does not run on Linux CI, so
it complements the table rather than replacing it, but it is what stops the table
drifting from reality.

**Edge Cases:**

- Lowercase drive letter (`c:`) — Windows accepts it; the predicate is a plain
comparison so it holds without a case rule.
- UNC share root vs. a directory under the share (AC2 vs AC4) — the boundary
where the volume prefix ends is exactly what could be got wrong.
- Relative base (`.`): NOT a root, and deliberately still rejected further down by
`filepath.Rel`/`Abs` semantics rather than here. The predicate must not
over-claim.
- Empty cleaned path cannot occur — `filepath.Clean("")` is `"."` and
`containedPath` has already run `Abs`.

**Negative Tests:**

The AC4 rows are the ones that stop the fix over-reaching. A guard that refused
every base would pass AC1–AC3 and break every real caller; without the negative
rows the table would not notice.

Mutation check (mandatory before review): revert the predicate body to the
pre-fix `cleanedPath == string(filepath.Separator)` and confirm that exactly the
Windows/UNC root rows redden with a message naming the pair, and that the Unix
rows stay green. Verify the mutation landed in executable code and not in a
comment — a suite that stays green after a "mutation" has only proved the
mutation missed.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A test that appears to cover Windows but does not.* This was correctly named
as the primary risk — and the mitigation named here was INSUFFICIENT, which is
the most useful thing in this document. Taking the values as parameters and
mutation-testing them does prove the branch executes. It does not prove the
inputs are ones Windows can produce, and they were not: the `\\server\share` row
asserted a `Clean` output that does not exist (RR-0DTTMS), passing green over a
case that cannot occur while the real case went unguarded. "Verifying nothing"
has two failure modes — a branch that never runs, and a branch that runs on
fiction — and this plan only defended against the first. Now also mitigated by
`clone_windows_test.go`, which derives its inputs from the host `filepath`
(RR-S2X70O).

- *Writing a test that cannot distinguish the fix from the bug.* MISSED in
planning, found in review (RR-T1XVAX). `TestContainedPath_UsesIsFilesystemRoot`
was added to pin the wiring, but on Linux it can only reach the predicate
through `/`, where old and new checks agree — so it passed identically against
unfixed code while reading as a guarantee. The check to apply to any new test on
a security fix is not "does it pass" but "does it fail against the code I am
replacing"; mutation testing asks exactly that, and this test was written after
the mutation runs rather than subjected to one.
- *Asserting `VolumeName` returns `""` on Unix without checking.* Would make the
"one expression subsumes both" claim load-bearing and unverified. Mitigated by
reading `internal/filepathlite/path_unix.go` and running a scratch program on the
host before writing the fix — recorded under Research.
- *Over-refusal breaking the one real caller.* `cmd/rela-desktop/main.go` derives
its base from `os.UserHomeDir`, which is never a volume root on any platform.
Covered by the AC4 negative rows.
- *A future reader "simplifying" the predicate back to a bare separator
comparison,* since on a Linux dev machine the volume term always looks like dead
weight. Mitigated by a comment at the predicate stating that the volume term IS
the Windows case and is `""` on Unix — the two facts a person needs before
deleting it — and by the table rows that fail if they do.

**Effort:** xs

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — `internal/git` is not a public API, no command, flag or configuration
changes, and no observable behaviour change for any base an operator would
realistically use. The documentation that matters is the godoc on the predicate,
which is where the "why the volume term is not dead code on Linux" reasoning
goes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none critical or significant. Two decisions worth
recording. First, rejecting the "add a second clause" shape the issue text
suggested, in favour of one subsuming expression — the added clause would leave a
permanently-dead branch on both platforms. Second, extracting the predicate for
testability rather than testing through `containedPath` alone: without the seam
there is no honest way to exercise the Windows case on the runner CI actually
uses, and the alternative (a Windows CI job) is far more machinery than an
`xs` fix warrants.
