---
id: DOCS-ZFWTD6
type: docs-checklist
title: 'Docs: Root-base guard misses the Windows drive root'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The godoc is a real deliverable here, because the code it describes looks wrong
on the only platform most contributors will ever run it on. On Linux,
`filepath.VolumeName` returns `""` unconditionally, so `isFilesystemRoot`'s
`volumeName` term reads as concatenating nothing — an obvious simplification
target, and deleting it silently restores the exact fail-open this ticket closes.
The doc comment therefore states the two facts a person needs before touching it:
that `VolumeName` is `""` only on non-Windows platforms, and that on Windows the
same expression catches a drive root and a UNC share root.

It also records why the function takes three parameters instead of reading
`filepath.VolumeName` and `filepath.Separator` itself, which is the other thing
that reads as needless indirection. All three values are platform-dependent, and
a Linux test cannot obtain the Windows ones from its own `filepath` — so the
parameters are what let CI execute the Windows comparison at all. Without that
note the natural "tidy-up" is to inline them, which would leave the Windows
branch permanently untested while the suite stayed green.

The test carries the same argument from the other side. `TestIsFilesystemRoot`'s
comment explains why the table supplies `(volume, cleaned, separator)` triples
rather than calling `containedPath("C:\\", ...)`: on Linux that call never enters
the branch under test — `C:\` is just a relative filename there — so it would
report coverage it does not have. The comment also records that the Windows
values are derived from the stdlib's `volumeNameLen` and `Clean`, not invented,
because a reader has no other way to check that the table models Windows
faithfully.

Two further comments carry real load, and both were written because review found
the thing they warn about.

`clone_windows_test.go` opens by saying why it exists at all, given that it never
runs on CI. A build-tagged test file that the pipeline skips looks like dead
weight, and the reason it is not — the Linux table hand-transcribes what `Clean`
is believed to return, and got `\\server\share` wrong exactly that way — is not
recoverable from the code. The comment names the incident so the file's value is
legible to someone deciding whether to delete it.

And where `TestContainedPath_UsesIsFilesystemRoot` used to be, there is now a
comment saying why no Linux wiring test exists: any such test can only reach the
predicate through `/`, where the old and new checks agree, so it would pass
against the unfixed code while reading as a guarantee. Documenting a deliberate
absence matters more than usual here, because writing that test is the obvious
next move and it looks like an improvement.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change — no new
command, flag, dependency or supported platform; the Windows MSI already shipped)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern. It makes an
existing containment boundary hold on a platform where it did not, and the
`storage.RootedFS` threat model it follows is already documented there)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: `internal/git` is reached
from the desktop clone flow, not from any CLI command; no help text mentions it)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no user-observable change. Every base an
operator can realistically configure behaves identically before and after — the
desktop caller derives its base from `os.UserHomeDir`, which is never a volume
root. A changelog line would describe a behaviour change users cannot observe)
- [x] ~~API docs updated (if applicable)~~ (N/A: `internal/git` is internal by
construction and has no published API surface)

## Rationale for N/A

Nothing an operator or user configures, invokes or sees changes. The one in-tree
caller (`cmd/rela-desktop/main.go`) passes a home-directory-derived base on every
platform, so no existing path behaves differently.

What changes is the failure mode for a base that a future caller, a future config
default, or a hand-edited setting could supply: on Windows, `C:\` used to be
accepted as a base and silently made the containment check unconditional, and now
it is refused with the same error `/` already produced. The audience for that is a
maintainer reading the godoc, not a user reading `docs/` — which is why the whole
documentation effort went into the comments.

One thing is deliberately NOT written down anywhere user-facing: the concrete
exploitation path (that `storeCredentials` drops a plaintext OAuth token under the
clone path, so a defeated containment check is a credential disclosure). It stays
in the godoc, where a maintainer needs it to understand why the guard cannot be
relaxed. Publishing it in a guide would hand over the recipe for no operator
benefit. Same call as DOCS-NDCQA6 made for #1496, and for the same reason.
