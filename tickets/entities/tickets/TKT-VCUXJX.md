---
id: TKT-VCUXJX
type: ticket
title: Make git clone containment unrepresentable via a validated CloneTarget
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

`git.CloneOptions.BaseDir` is required as of TKT-S2SFTG, but "required" is
enforced at runtime, on a struct literal, with an error the caller only sees
when they run the code. The forgetful caller — the stated threat model — can
still write the mistake and compile it.

Raised during the TKT-S2SFTG review as the stronger design.

## Proposal

Make the invariant unrepresentable:

```go
func NewCloneTarget(baseDir, path string) (CloneTarget, error)
func Clone(target CloneTarget, opts CloneOptions) error
```

`CloneTarget` is opaque and holds the validated absolute path. Then:

- "forgot to sanitize" becomes a compile error rather than a runtime one;
- `storeCredentials`'s doc prose ("MUST already have passed containedPath")
becomes a type signature;
- the `#nosec G703` annotation in `clone.go` gets a justification a tool can
check, rather than an assertion it must take on faith.

This also matches CLAUDE.md's "constructors reject nil required fields" rule and
the `param-contract` commentlint rule about preconditions asserted in prose
about a bare `string`.

## Why not in TKT-S2SFTG

That ticket was a security fix closing a fail-open. This changes the package's
exported surface and every call site, so it belongs on its own where it can be
reviewed as a design change rather than smuggled in beside a CVE-shaped fix.

## Context worth keeping

This is the SECOND round on this code. The first pass added `BaseDir` as
optional with a doc comment claiming a guarantee it did not provide; the second
(TKT-S2SFTG) made the code match the comment. The systemic lesson is that a
comment is not an enforcement mechanism — which is exactly what this ticket
proposes to fix properly.
