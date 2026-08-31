---
id: TKT-S2SFTG
type: ticket
title: Require BaseDir on git.Clone
kind: bug
priority: medium
effort: xs
status: done
---

## Description

`CloneOptions.BaseDir` (`internal/git/clone.go`) is optional. When empty,
`containedPath` skips the containment check entirely:

```go
if base == "" {
    return abs, nil
}
```

The code's own documentation claims containment lives at the `Clone` boundary
"so a future caller that forgets to sanitize is still safe". That guarantee is
false in exactly the case it describes: a caller who forgets `BaseDir` gets no
containment, silently. The forgetful caller is the entire threat model, and it
is the one case that fails open.

GitHub issue #1270. Source: IB-review of rela#1247.

**Violated requirement**: POLICY-015 §3 (Secure by Design) — security is
considered from the first phase of development (shift-left).

## Why it matters

`ExtractRepoName` returns the URL's last path segment and can yield `..`. A
caller that derives `Path` from it without setting `BaseDir` would clone outside
the intended directory — and `storeCredentials` writes a plaintext OAuth token
into `<Path>/.git/credentials`, so a traversal both escapes the chosen directory
and drops a credential there.

Not acutely exploitable today: there is exactly one caller
(`cmd/rela-desktop/main.go`) and it sets `BaseDir` correctly. The risk is
architectural, which is precisely what the shift-left requirement is about.

## Scope

IN: make `BaseDir` required — `Clone` rejects an empty value rather than
skipping containment.

OUT: symlink resolution. `containedPath` is deliberately string-level (Clean +
Rel), matching `storage.RootedFS`'s threat model: the target is "the
caller-supplied final segment contains traversal syntax", not "an attacker
already has write access to the base directory". Changing that is a different
ticket with a different argument.
