---
id: TKT-0CHRO7
type: ticket
title: Extract the seed cluster off docRuntime (36 → 33)
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of [[TKT-N0IKN9]], part of the `internal/docs` `docRuntime` arc.

## What

**Pure structural extraction, no behavior change, no exported-API change, no
Lua-visible change.**

Two fields and three methods move off `docRuntime` onto a focused seed recorder
in `seed.go`, keeping registration on the runtime as the wiring seam:

- **Fields**: `seedCounts`, `seedOps`.
- **Methods** (−3): `luaCreate`, `luaLink`, `mintID` (from `module.go`).

## Why this cluster is a boundary

Seeding is WRITE-side fixture construction; the rest of `docRuntime` is
READ-side graph interrogation and assertion. Both fields exist for documented,
self-contained reasons: `seedCounts` is a per-type auto-id counter so `mintID`
avoids a full-store scan per `create()`, and `seedOps` records every create/link
so a `screenshot{}` island can replay them against a fresh fsstore temp project
(DR-S2).

## Embedding was rejected deliberately

The `seedOps` reads in `screenshot.go`/`api.go` could have been left untouched
by embedding the seed type, so `dr.seedOps` keeps resolving by promotion. Tested
against plimsoll v0.2.0 in a scratch module: **it counts only directly-declared
methods, not promoted ones** — a type with 3 promoted and 1 own method passes
`max-methods=1`.

So embedding would have reported 33 while leaving every seed method callable on
`dr`, satisfying the linter without removing the reach the linter exists to
detect. Rejected as gaming the metric. A named field costs three one-token edits
and actually severs the coupling.

Worth knowing for the rest of the ratchet arc: plimsoll can be satisfied by
embedding without decoupling anything.

## Preventive, not a gate fix

On `develop` `docRuntime` is at 36 — under the 40-method line, carrying no
`//plimsoll:max-*` directive. No directive was added: annotating a compliant
type pins it rather than ratchets it.

Interacts with the parallel Tier-B ticket, which reads `seedOps` through a `seed
func() []SeedOp` seam. Both branch off develop independently; whichever lands
second reconciles (mechanical) and settles the final count.

## Not verified

The end-to-end screenshot replay was not exercised (needs Chrome and a built
SPA). `internal/docscapture` passed from cache, which is legitimate only because
`ApplySeed` and `SeedOp` are unchanged in shape and signature. The replay
argument rests on the method bodies being byte-identical apart from the
receiver.

## Done when

- [x] `docRuntime` at 33 methods
- [x] `go build ./...` and `-tags postgres` clean
- [x] `go test ./internal/...` passes
- [x] `just arch-lint` OK, `just comment-lint` clean
- [x] `just plimsoll` exit 0
- [x] PR open (#1475)
