---
id: TKT-JRY8V5
type: ticket
title: Gate the launcher routes (/api/open-file, handleOpenURL) — currently ACL-unaware and unaffected by --read-only
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`registerCommandRoutes` mounts three routes. TKT-MJ02AO gated one of them.
`/api/open-file` performs **no ACL check** and still spawns the OS file handler
under `rela-server --read-only`.

Raised as RR-PG8HR2 during TKT-MJ02AO's code review, deferred there with reason.

## Current state

`internal/dataentry/command_handler.go`:

```go
mux.HandleFunc("/api/command/", h.handleCommandExec)        // gated (TKT-MJ02AO)
mux.HandleFunc("/api/command-cancel/", h.handleCommandCancel) // owner-bound (TKT-MJ02AO)
mux.HandleFunc("/api/open-file", h.handleOpenFile)          // UNGATED
```

`handleOpenURL` also exists and shells out via `openURLCommand`, but is
deliberately **not mounted** — worth deciding whether it should stay that way or
be mounted-with-a-gate.

## What is already defended (so this is not over-rated)

- `containedProjectPath` confines the path to the project root — traversal,
absolute paths, NUL bytes and symlink escape all handled
- `openFileCommand` passes the path as a **discrete argv element**, never
through a shell — no `sh -c`, no argument injection
- POST-only, behind `requireSameOrigin` + `requireLocalHost`

It cannot execute attacker-supplied content: the file must already be in the
project.

## Residual risk

It is still a **process spawn dispatched by MIME association**. `xdg-open` on a
`.desktop` file, or `cmd /c start` on Windows, launches by handler association
rather than merely displaying. Benign on the intended local-desktop deployment
(the user could open the file themselves); meaningful on a shared or remote
`rela-server`, where it is an ACL-unaware spawn on the server host.

## The design question this ticket must answer

Not "add a permission check" — the interaction matters:

**`auto_open` triggers this route programmatically.** When a command emits a
`file` SSE message and `auto_open` is not disabled, the SPA POSTs
`/api/open-file` on the user's behalf. Gating it naively would break auto-open
for commands the principal *is* authorized to run.

Options:

1. **Deny under `ReadOnlyACL` only** — cheapest, matches operator expectation,
leaves `Declarative` ungated.
2. **Its own named permission** (`command:open-file` or similar) — consistent
with the `command:*` family, but an operator must now grant it separately or
auto-open silently stops working.
3. **Ride on the originating command's permission** — most precise (the file
came from a command the principal was allowed to run), but requires correlating
the open-file request with the exec that produced the path, which the current
protocol does not carry.

Option 1 is the likely v1; 3 is the "right" answer if the SSE protocol ever
carries an exec correlation id.

## Scope

**In scope:**

- Deny `/api/open-file` under `ReadOnlyACL`
- Decide and implement the `Declarative` posture per the options above
- Verify the `auto_open` flow still works for authorized commands (this is the
regression risk)
- Decide whether `handleOpenURL` should remain unmounted or be mounted with the
same gate
- Go tests: read-only deny, auto-open still works when authorized
- Docs: `docs/acl-security.md` — the launcher routes' posture

**Out of scope:**

- Read-gate scoping of command payloads (TKT-2FDTJE)
- Changing `containedProjectPath` — the containment logic is sound

## Acceptance criteria

- Under `rela-server --read-only`, `POST /api/open-file` is denied
- The `auto_open` flow still opens files for a principal authorized to run the
producing command (regression test — this is what a naive gate breaks)
- The `handleOpenURL` mount decision is recorded in the code comment
- `docs/acl-security.md` states the launcher routes' authorization posture
