---
id: TKT-T7G7LT
type: ticket
title: Root-base guard misses the Windows drive root
kind: refactor
priority: medium
effort: xs
status: done
---

## Description

The root-base guard in `containedPath()` (`internal/git/clone.go`, added by PR
#1496 / TKT-S2SFTG) compares the cleaned base directory against the Unix form of
the separator only:

```go
if absBase == string(filepath.Separator) {
    return "", errors.New("clone base directory must not be the filesystem root")
}
```

A Windows drive root (`C:\` after `filepath.Clean`) is therefore not recognised
as a root. `rela-desktop` ships as a Windows MSI
(`.github/workflows/release.yml`, `os_name: windows`), so on that supported
platform exactly the scenario PR #1496 set out to close remains open: a root
base that makes the containment check pass unconditionally while appearing to
have verified something.

GitHub issue #1498. Source: IB-review of rela#1496.

**Violated requirement**: POLICY-015 §3 (Secure by Design) — security is
considered from the first phase of development (shift-left), and OWASP Top 10
risks are structurally addressed.

## Why it matters

The root-base guard is not decoration. `containedPath` is the boundary that
stops a caller-supplied `Path` — whose final segment can derive from a
remote-controlled URL via `ExtractRepoName` — from escaping the directory the
operator chose. `storeCredentials` then writes a plaintext OAuth token into
`<Path>/.git/credentials`, so a defeated containment check is a credential
disclosure, not merely a misplaced directory.

With `BaseDir = "C:\"` on Windows, `filepath.Rel` succeeds for every absolute
path on that drive and never yields a `..` prefix, so the containment check
returns "contained" for `C:\Windows\System32`, `C:\Users\victim`, anything. The
guard reads as present and does nothing — the identical silent-no-op shape that
the empty-base and Unix-root cases were both refused for.

Severity is low in the same sense #1496's was: the one in-tree caller
(`cmd/rela-desktop/main.go`) derives its base from `os.UserHomeDir`, which is
never a drive root. The defect is that the boundary's guarantee is
platform-dependent, which is precisely what a shift-left requirement is about.

## Scope

IN: recognise every Windows volume root as an invalid base — the drive root
(`C:\`), the UNC share root in BOTH spellings (`\\server\share\` and the bare
`\\server\share`, which is what `filepath.Clean` actually returns), and the
extended-length forms `\\?\C:` and `\\?\UNC\server\share`. One predicate,
correct on both platforms, rather than a bolted-on platform special case.

The bare-volume spelling was missed by the first implementation and found in
code review (RR-0DTTMS): `filepath.Clean` appends no trailing separator when the
whole path IS the volume, and `filepath.Rel` then treats that base as absolute,
so the share root was accepted and every path on the share reported contained.

OUT:

- Symlink resolution. `containedPath` stays deliberately string-level (Clean +
Rel), matching `storage.RootedFS`'s threat model. Unchanged from TKT-S2SFTG.
- Case-insensitive matching and 8.3 short-name expansion on Windows. Those
change what counts as the SAME path, not what counts as a root, and neither can
be settled without a Windows runner. A different ticket with a different
argument.
- Drive-relative bases (`C:foo`). Not a root — it names a path relative to the
drive's working directory — and it is refused further down by `Abs`/`Rel` rather
than here. Pinned negatively so the guard does not over-claim.
