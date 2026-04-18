---
id: TKT-AHUNF
type: ticket
title: Enable additional golangci-lint v2 linters for high-signal checks
kind: chore
priority: low
effort: s
status: done
---

## Description

Enable a curated set of v2-era linters that weren't in the repo's baseline set.
Chosen for high signal-to-noise in this codebase (CLI + HTTP server + Lua
sandbox + MCP + desktop).

**Linters to enable:**

| Linter | Why |
|--------|-----|
| `contextcheck` | Catches ctx-threading bugs; directly addresses the class of finding cranky raised on TKT-AWX7V (critical #1, significant #2). |
| `containedctx` | Prevents context.Context stored in struct fields. |
| `copyloopvar` | Flags `x := x` patterns now unnecessary on Go 1.22+. `--fix` supported. |
| `intrange` | Suggests `for i := range N`. `--fix` supported. |
| `usetesting` | Enforces `t.Context`/`t.TempDir`/`t.Setenv` in tests. |
| `sloglint` | Pairs with existing depguard-enforced `log/slog` usage. Catches inconsistent attr styles. |
| `perfsprint` | Mechanical perf wins on `fmt.Sprintf`. `--fix` supported. |
| `usestdlibvars` | `http.StatusOK` over `200`, `http.MethodPost` over `"POST"`, etc. |
| `forcetypeassert` | Requires the `ok` form on type assertions. |
| `predeclared` | Flags shadowing of stdlib builtins (`len`, `error`, etc.). |
| `gocheckcompilerdirectives` | Typo-checks `//go:` directives. |

**Approach:**

1. Enable all eleven in `.golangci.yml`.
2. Run `golangci-lint run --fix ./...` for the mechanical ones.
3. Hand-fix what `--fix` doesn't handle; add targeted exclusions where noise exceeds signal (document each with a one-line reason).
4. Run `just ci` end-to-end.
5. Open PR with auto-merge; monitor.

**Out of scope:**

- The 15+ other disabled linters deemed too opinionated for this codebase (wrapcheck, wsl, varnamelen, exhaustruct globally, etc.). Documented in the TKT-AWX7V review conversation.
