---
id: TKT-3I173M
type: ticket
title: 'Enable gosec G703 (path traversal) and fix clone path containment'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

`G703` was in the `gosec.excludes` block in `.golangci.yml`, so path-traversal
taint analysis never ran. Enabling it surfaced three findings, one of them a
genuine vulnerability in `internal/git`.

`ExtractRepoName` returned the last path segment of a user-supplied URL with no
validation, and `IsValidRepoURL` accepted the result. Verified on pre-fix code:

```
url=https://github.com/user/..   name=".."   join=/home/me   valid=true
```

A clone therefore lands **outside** the directory the operator chose.
`storeCredentials` then writes a plaintext OAuth token to
`<escaped>/.git/credentials` — traversal plus credential disclosure.

The other two findings are false positives: `internal/docscapture/server.go`
writes to `filepath.Join(<os.MkdirTemp dir>, <literal name>)` (hardcoded
filenames, process-created temp dir), and `internal/docscli/docscli.go` writes to
`c.Output`, the operator-typed `--out` flag of a local CLI whose trust boundary is
the operator's own shell.

## Solution

Fixed in two layers so neither alone is load-bearing:

- **`Clone` is the boundary.** New `CloneOptions.BaseDir` with a `containedPath`
helper (`Abs` + `Clean` + `Rel`, rejecting `..`, `../…`, and `.`). Placed in
`Clone` rather than the caller, so a future caller that forgets to sanitize is
still safe. `git.Clone` has exactly one caller (`cmd/rela-desktop/main.go`), now
passing `BaseDir`.
- **`ExtractRepoName` hardened at source** — returns a safe single segment or
`""` (callers already treat `""` as "cannot determine name").

`internal/storage.RootedFS` was considered and deliberately not used: it is a
**keyed** API whose `resolve()` rejects absolute paths, and all three sites
legitimately write to absolute paths outside any project root (a temp dir, an
arbitrary `--out`, a user-chosen clone dir), so there is no root to bind them to.
Its godoc also scopes it to *project storage*, which these are not. Used the
sanctioned alternative — `filepath.Clean` plus explicit root containment — and
mirrored RootedFS's documented threat model in the godoc (string-level
validation, no symlink resolution).
