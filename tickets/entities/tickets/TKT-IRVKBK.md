---
id: TKT-IRVKBK
type: ticket
title: 'Enable gosec G702 (command injection) with reviewed exec-site annotations'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

`G702` was in the `gosec.excludes` block in `.golangci.yml`, so command-injection
taint analysis never ran. Enabling it surfaced 11 findings across the exec sites.

One is a genuine defense-in-depth gap. `internal/dataentry/document.go` runs an
operator-configured command through `sh -c`, which is the intended feature. The
gap was upstream: `renderCommand` string-spliced the request-derived `entryID`
into that command via `{id}` / `{id_lower}` **before** it reached the shell. It
was safe only because all three HTTP callers happened to call
`isSafePathSegment` first — an invariant held by convention across call sites,
with nothing enforcing it at the point of use.

The remaining 10 are reviewed false positives: the `commands.go` `sh -c
cmd.Script` site (where the shell is the feature and request data reaches the
script only as stdin JSON and `RELA_*` env vars), and 9 launcher sites
(`open`, `xdg-open`, `explorer`, `cmd /c start`) that use argv arrays with
compile-time-constant program names, so no shell parses the argument.

## Solution

- Move the `isSafePathSegment` check into `renderCommand`, so the guarantee lives
where the value enters the shell string and a future caller that forgets the
upstream check gets an error instead of an injection. This is defense-in-depth,
not a live exploit fix — no current path was vulnerable.
- Add `--` to the two `open(1)` calls so argument-injection safety is structural
rather than a side effect of the upstream validator. `xdg-open` has no `--`
support; its inputs are always absolute paths or schemed URLs.
- Annotate each reviewed site with a narrow per-line `#nosec G702` naming its
specific trust boundary — no blanket file-level suppressions.
- Remove `G702` from the `gosec.excludes` block.
