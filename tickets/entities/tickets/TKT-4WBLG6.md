---
id: TKT-4WBLG6
type: ticket
title: Extract markdown AST helpers off lua.Runtime (plimsoll ratchet 105 → ~60)
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of [[TKT-N0IKN9]] (plimsoll god-object decomposition), opening the
`lua.Runtime` arc. Runtime sits at `//plimsoll:max-methods=105`; `markdown.go`
alone contributes 46 receiver methods, and 41 of them touch **no Runtime state
except `r.L`** (verified: only `L.NewTable` / `L.SetField` / `L.NewFunction`).

## What

**Pure structural extraction, no behavior change, no exported-API change** (all
46 methods are unexported; Runtime's 10 exported methods are untouched).

- New `mdHelpers` type in `markdown.go` holding the Lua state — receiver change
in place for the 41 state-free AST methods (parse/render/shift-headers/
extract/concat/AST↔Lua conversion/deep-copy/resolve-refs machinery).
- The 3 graph-coupled entity-ref methods (`luaMdEntityRefs`,
`parseEntityRefsOpts`, `buildEntityRefValue`) move to a narrow binding struct
holding just the read deps + caller-ctx closure (the `ReadDeps` consumer-side
pattern from `deps.go`).
- `registerMarkdownModule` stays on Runtime as the one-line wiring seam,
mirroring `registerURLModule`.
- Lower `//plimsoll:max-methods` on Runtime to the new count (~60).

## Why this shape

The package's own `urlHelpers` godoc (urls.go) argues the exact case: hanging a
pure function group off Runtime made a god-object; grouping by file doesn't help
because the linter (and any reader of Runtime's API) counts per type.
`registerCryptoModule` / `registerDateHelpers` are the free-function variants of
the same precedent.

## Done when

`just plimsoll` passes with the Runtime directive lowered to the new count, `go
test ./internal/lua/...` and the full suite pass, `just arch-lint` clean.
