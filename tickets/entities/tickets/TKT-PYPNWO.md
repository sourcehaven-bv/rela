---
id: TKT-PYPNWO
type: ticket
title: Command file actions (open/reveal) launch xdg-open on the server — no-op and unsafe for remote deployments
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A `commands:` script emitting `::rela::{"type":"file"}` makes the SPA `POST
/api/open-file`, and the server spawns an OS opener (`xdg-open`/`open`/
`explorer`) on the host `rela-server` runs on. Correct for
`rela-desktop`/localhost; for a remote headless deployment it silently no-ops
and lets an authenticated remote client drive server-side process launch.
Deliver the file to the browser instead when not local.

## Summary

When a `commands:` script emits `::rela::{"type":"file", …}`,
`CommandModal.vue:112-121` calls `POST /api/open-file`, and the server launches
an OS opener on its own host, chosen by `runtime.GOOS`:

```go
// internal/dataentry/commands.go:513 (openFileCommand)
case "linux":
    if action == "reveal" { return exec.Command("xdg-open", filepath.Dir(filePath)) }
    return exec.Command("xdg-open", filePath)
```

(`openURLCommand`, `commands.go:571`, has the same shape.)

This is correct for the desktop/local model — `rela-desktop` (Wails) or
`rela-server` on loopback, where the server host *is* the user's machine. It
breaks for a remote deployment (`rela-server-postgres` on a headless VM behind
oauth2-proxy):

- No display on the server, so `xdg-open` silently no-ops. The UI shows the command
Completed with Open/Reveal buttons that do nothing.
- Even if it worked it would open **on the server**, never what a remote user wants.
- An authenticated remote user makes the server process spawn `xdg-open`/`open` on
a server-side path. Constrained to the project root by `containedProjectPath`
(`commands.go:604`), but still remote-client-driven local process launch.
- `--read-only` does **not** block it: the launcher endpoints bypass the ACL write
path (`ReadOnlyACL` only denies `acl.WriteRequest`).

Note `auto_open !== false` in `CommandModal.vue` — undefined auto-opens, so this
fires without an explicit user click by default.

## Current state of remote-vs-local awareness

There is **no** headless/remote mode concept — no `isRemote`, no capability
negotiation exposed to the SPA. Loopback detection exists but only drives
security *warnings*, not feature gating:

- `isLoopbackHost` — `cmd/rela-server/main.go:505`
- non-loopback warnings at `main.go:333, 340, 355`
- the one hard refusal precedent: `--debug-pprof` errors on non-loopback (`main.go:478`)

`commandHandler` (`internal/dataentry/command_handler.go:25-30`) is four
closures over `App` — the natural seam for a fifth `isLocal func() bool`.

## Expected

For a browser session against a remote server, a file the command produced
should be delivered to the browser (HTTP download / new tab), not opened via a
launcher on the server host. `reveal` has no meaningful remote equivalent.

## Suggested directions

- Detect remote-vs-local (non-loopback bind, an explicit `--headless`/`--remote`
flag, or HTTP-daemon-vs-Wails context) and in remote mode serve the file over
HTTP — ACL-applied, project-contained — instead of spawning a launcher.
- Add a first-class "download this file" affordance to the `::rela::` file message,
decoupled from the desktop launcher. `TKT-Q85275` already built an ACL-gated
attachment download endpoint; reuse that shape rather than inventing a second
one.
- At minimum, refuse `/api/open-file` when running as the HTTP daemon on a
non-loopback bind and surface a clear "not available on a remote server" rather
than a silent no-op.

## Related dead code found while investigating

`/api/open-url` is **not mounted** (`command_handler.go:34-36` registers only
`/api/command/`, `/api/command-cancel/`, `/api/open-file`), and the SPA's
`processSSEEvent` has no case for `type:"open"` — it's parsed and streamed
server-side, then ignored. `type:"entity"`, `"group"`, `"endgroup"` likewise
fall through silently despite backend test coverage. Decide whether to wire or
delete.

## Repro

1. Run `rela-server-postgres` on a headless Linux host behind a reverse proxy.
2. Define a `commands:` entry (`context: entity`) whose script writes a file and
emits `::rela::{"type":"file","path":"…","action":"open"}`.
3. Trigger from the browser → command completes, file is created on the server,
Open/Reveal do nothing.

## Version

Reproduced on `f25fa238` and current `develop`. On `develop` the handler moved
into `command_handler.go`/`commands.go` via a refactor; behaviour unchanged.
