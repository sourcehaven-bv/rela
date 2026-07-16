---
id: TKT-KUFLD
type: ticket
title: Extract dataentry command route cluster into commandHandler
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc (M5.2).
Delivered in PR #1138 (stacked on [[TKT-VG9P1]] / #1134).

## What

Moved the 11-method user-command surface (`commands.go`) off `App` into a new
`commandHandler`.

- **`App`: 154 → 143 methods** — `//plimsoll:max-methods` directive ratcheted.
- Covers the SSE shell-exec endpoints (`/api/command/`, `/api/command-cancel/`),
the file/URL launchers (`/api/open-file`; `handleOpenURL` stays a tested method,
unmounted as before), and `resolveCommands` (consumed by `handleV1Commands`,
which now delegates via `a.commands.resolveCommands`).
- Collaborators are narrow **closures over App** (consumer-side interfaces),
mirroring `affordanceService`: `schema`, `services`, `projectRoot`,
`executeView`. The handler owns no mutable state — the in-flight exec registry
stays a package-level `sync.Map`.

## Verification

- `go test -race ./internal/dataentry/...`
- Builds + vet across default / `postgres` / `memorybackend`
- `just plimsoll`, `just arch-lint`, `golangci-lint` (0 issues), `gofmt`
