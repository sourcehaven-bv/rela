---
id: TKT-DOPCTI
type: ticket
title: Extract HTTP/cache/AI binding clusters off lua.Runtime (ratchet 60 → ~45)
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of [[TKT-N0IKN9]], continuing the `lua.Runtime` arc after
[[TKT-4WBLG6]] (markdown cluster, 105 → 60).

## What

**Pure structural extraction, no behavior change, no exported-API change.**
Three shallow binding clusters move off Runtime into focused types, keeping each
`register*` method on Runtime as the one-line wiring seam:

- **HTTP** (`http.go`, −8): `luaHTTPRequest/Get/Post/Put/Patch/Delete`,
`luaHTTPSimple`, `doHTTPRequest` → `httpBindings`. `httpContext` narrowed to
take `*lua.LState` instead of `*Runtime`. The `caps.HTTP` capability gate stays
in `registerBindings` — no security semantics move.
- **Cache** (`cache.go`, −4): `requireCacheContext`, `luaCacheGet/Set/Memoize`
→ `cacheBindings` holding the narrow `cacheStore` interface + a `scriptPath
func() string` closure (scriptPath is mutable via `SetScriptPath` after
construction, so it must NOT be captured by value).
- **AI** (`ai.go`, −3): `luaAIChat/Complete/Embed` → `aiBindings` holding
`ai.Provider`.

## Rebase note (2026-08-29)

Originally stacked on TKT-4WBLG6's branch; after that merged (#1461) this was
rebased onto `develop`, which had meanwhile landed TKT-XWZIOB (mail.send, 60 →
61). Both histories are preserved in the directive comment and the arithmetic
re-derived: **61 → 46**, not the originally-planned 60 → 45. Two develop-side
changes were merged in rather than overwritten: `doHTTPRequest` now takes an
`httpRequestOpts` struct instead of 7 positional args (develop's refactor, kept,
with the receiver retargeted to `httpBindings`), and the `mail.send`
method-value rationale.

Ratchet `//plimsoll:max-methods` on Runtime **61 → 46** (verified by count).

## Done when

plimsoll passes with the lowered directive, full test suite + race on
internal/lua green, arch-lint/comment-lint/golangci-lint clean.
