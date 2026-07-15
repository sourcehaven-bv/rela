---
id: TKT-VG9P1
type: ticket
title: Extract dataentry sync route cluster into syncHandler
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc (M5.2).
Delivered in PR #1134.

## What

Moved the 16-method `/api/sync/` route cluster (`sync.go` + `sync_handlers.go`)
off `App` into a new `syncHandler` value-service.

- **`App`: 170 → 154 methods** — `//plimsoll:max-methods` directive ratcheted.
- Narrow consumer-side interfaces at the call site: `syncStore` (entity/relation
gets), `syncDeleter` (delete path). The two optional capabilities — the
pgstore-only manifest source and the `*entitymanager.Manager` id-preserving
applier — are resolved once in `newSyncHandler` (nil on fs/memory builds, where
the endpoints degrade to 501).
- `writeMu` shared by **pointer** so sync pushes/deletes serialize against every
other data-entry mutation handler, exactly as before (race detector guards).

## Verification

- `go test -race ./internal/dataentry/...`
- Builds + vet across default / `postgres` / `memorybackend`
- `just plimsoll`, `just arch-lint`, `golangci-lint` (0 issues), `gofmt`
