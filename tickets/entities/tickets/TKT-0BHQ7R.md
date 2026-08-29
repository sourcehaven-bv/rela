---
id: TKT-0BHQ7R
type: ticket
title: Extract Tier-B capability bindings off docRuntime (36 → 31)
kind: refactor
priority: medium
effort: s
status: review
---

Sub-ticket of [[TKT-N0IKN9]], opening the `internal/docs` `docRuntime` arc.

## What

**Pure structural extraction, no behavior change, no exported-API change, no
Lua-visible change.**

Six fields and five methods move off `docRuntime` onto a focused
`tierBBindings`, keeping registration on the runtime as the wiring seam:

- **Fields**: `capturer`/`capturerErr`, `apiClient`/`apiClientErr`,
`projectDir`, `outDir`.
- **Methods** (−5): `luaScreenshot`, `readAnnotations`, `resolveOutPath`,
`relOutPath` (`screenshot.go`), `luaAPI` (`api.go`).

## Why this cluster is a boundary

These are the **injected, may-be-nil, fail-loud Tier-B capabilities**.
`capturer` is nil unless the CLI injects it — that is what keeps core docs
browser-free — and `apiClient` is nil-able for the same documented reason: a
skipped assertion looks exactly like a passing one. Both carry their own
error-explaining field for the fail-loud message.

Nothing else in the type touches them, while `luaScreenshot` previously had
reach into `policy`, `store` and `tracer` for no reason. That is plimsoll's
stated rationale: a receiver with dozens of private helpers is one struct whose
fields they can all reach.

## The seedOps seam

`screenshot.go` and `api.go` read `seedOps` — the create/link log a
`screenshot{}` island replays against a fresh fsstore temp project (DR-S2). The
extracted type receives it as `seed func() []SeedOp`, **not a captured slice**:
registration happens once before any island runs while the ops accumulate as the
manual executes, so a value snapshotted at construction would always be empty.

`rejectUnknownKeys` now takes a small consumer-side `luaFailer` interface rather
than the whole runtime.

## Preventive, not a gate fix

On `develop` `docRuntime` is at 36 — under the 40-method line, carrying no
`//plimsoll:max-*` directive. This ratchets to 31 for headroom; the type is a
façade over the `doc.*` Lua binding table (33 of 45 methods on the worlds branch
are one-verb `luaX` callbacks), so it grows with the doc language.

Interacts with the parallel seed-cluster ticket, which moves `seedOps` itself.
Both branch off develop independently; whichever lands second reconciles the
seam (mechanical) and settles the final count.

## Done when

- [x] `docRuntime` at 31 methods
- [x] `go build ./...` and `-tags postgres` clean
- [x] `go test ./internal/...` passes
- [x] `just arch-lint` OK
- [x] `just plimsoll` exit 0
- [x] PR open (#1476)
