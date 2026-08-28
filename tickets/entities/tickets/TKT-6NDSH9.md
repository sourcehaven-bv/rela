---
id: TKT-6NDSH9
type: ticket
title: Move Lua action + webhook dispatch onto writeHandler (complete the write surface)
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc (M5.4 tail).
Follows [[TKT-HKY8RJ]] (write nucleus).

## What

Moved the last two direct `writeMu` takers off `App` onto `writeHandler`:

- `handleV1Action` (actions.go) — receiver change in place; the interactive
Lua action endpoint (`POST /api/v1/_action/{id}`).
- `dispatchWebhookAction` (webhook.go) — the free dispatch function retargeted
from `*App` to `*writeHandler`; `SetWebhookReceiver` / `registerWebhookRoutes`
stay on `App` (public wiring + routing), the dispatch closure passes `a.write`.

Three new `writeHandler` collaborators, all LIVE closures over App (wired
identically in `NewApp` and `rebindApp`): `engine` (`app.scriptEngine`),
`luaDeps` (`app.luaWriteDeps` — the bundle is derived per call from swappable
collaborators), `fullScriptDetail` (`app.allowFullScriptDetail` — the security
layer is wired after construction via `SetSecurityConfig`).

- **`App`: 115 → 114 methods** (base had absorbed develop's DEC-O59WM4
script-read helpers); `//plimsoll:max-methods` ratcheted 115 → 114.
- **This completes the write surface**: every data-entry write path now lives
on `writeHandler` or holds a pointer to the shared mutex (sync/attachment
handlers) — no `App` method takes `writeMu` directly anymore. The [[DEC-8UIL0]]
Tx arc now has a single obvious seam.
- PURE STRUCTURAL: bodies verbatim, lock scope unchanged, webhook principal
stamping (`webhook:<event>`) and correlation-id behavior unchanged.

## Verification

- `go test -race ./...` (full suite)
- Builds across default / `postgres` / `memorybackend`
- `just plimsoll`, `just arch-lint`, `golangci-lint` (0 issues), `gofmt`
