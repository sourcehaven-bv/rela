---
id: TKT-XTNED
type: ticket
title: Add Lua backend to data-entry commands
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

The data-entry app already has a `commands:` system in `data-entry.yaml` that
lets users attach buttons to screens (entity, list, view, dashboard) which
trigger a shell script via `sh -c`. Scripts receive context as JSON on stdin,
emit structured output via a `::rela::{...}` prefix protocol parsed as SSE
events, and can be cancelled mid-run.

This ticket adds a **Lua script backend** alongside the existing shell backend,
reusing the command system's per-screen scoping, SSE streaming, and
cancellation. Shell commands remain unchanged.

## Motivation

Shell commands are powerful but have real limits:

- **Not portable**: prototype examples rely on bash + `jq`; Windows users are stuck
- **Spawn overhead**: every invocation forks `sh`
- **Indirect graph access**: shell scripts must shell out to the `rela` CLI to read/write entities
- **Harder to write**: structured output means `echo '::rela::{...}'` with manual JSON escaping

Lua runs in the existing sandbox (see TKT-HVHF, TKT-17qi, the recent
cancellation work in 0f76c4a), is cross-platform, has direct bindings to the
graph (`rela.list_entities`, `rela.create_entity`, …), and can offer a
first-class API for emitting structured output without string-formatting JSON by
hand.

## Scope

**In scope:**

- New `lua:` key on command config in `data-entry.yaml`, alternative to `script:`. Exactly one of `script:` or `lua:` must be set per command
- Lua backend reuses the existing command scoping (`context`, `available_on`), so a Lua command is available on the same screen types as a shell command
- Context delivered to the Lua script as a `rela.context` table — not stdin JSON. Fields mirror the existing shell input shape per context type:
  - `entity`: `entity`, `relations`, `project`
  - `list`: `list_id`, `entities`, `project`
  - `view`: `view_id`, `entity`, `collections`, `relations`, `project`
  - `global`: `project`
- Output via dedicated emit helpers, not `print("::rela::...")`:
  - `rela.emit.message(text)` — status/progress message
  - `rela.emit.error(text)` — error message
  - `rela.emit.log(text)` — raw log line
  - `rela.emit.file({path, label, action})` — open-file action (matches existing shell protocol)
  - `rela.emit.open_url(url)` — open URL (matches existing shell protocol)
  - A general `rela.emit(msg_table)` escape hatch for any shape the helpers don't cover
- Each emit call streams an SSE event to the browser immediately (same events the existing protocol produces, so the frontend needs no changes)
- Cancellation: `POST /api/command-cancel/{execID}` already exists for shell commands; wire it to cancel the Lua runtime's context (cancellation support landed in 0f76c4a)
- Lua scripts live under a project-relative path (e.g. `commands/` or `actions/` — decide in planning), path-validated the same way shell `script:` paths would be if they referenced files
- Tests cover: config parsing (`script:` XOR `lua:`), execution per context type, emit-to-SSE streaming, cancellation, sandbox denial of forbidden APIs, error propagation, and one end-to-end test per context type

**Out of scope:**

- Field-level form patching / edit-form interaction (`set_fields` response) — deferred to a follow-up ticket
- New Lua capabilities such as HTTP client, filesystem access beyond the existing sandbox — separate tickets
- Deprecating or removing the shell `script:` backend
- Multi-step interactive flows (covered by FEAT-YIOF)
- Permissions / per-user command visibility
- Conditional button visibility (hide/disable based on state)

## Open questions for planning

1. **Script location**: do Lua command scripts live in the existing `actions/` directory (shared with TKT-HVHF actions), or a new `commands/` directory? `actions/` risks name collisions; `commands/` is clearer but adds a new convention.
2. **`rela.context` immutability**: context is a snapshot — should the table be read-only (metatable guard) so scripts can't accidentally mutate and expect it to persist?
3. **Emit ordering vs. script return value**: TKT-HVHF's existing action pipeline uses the Lua return value for `{redirect, message}`. Commands stream output instead. Does a Lua command's return value carry meaning (e.g. final success/error), or are emits the only channel and the return value is ignored?
4. **Error semantics**: if the Lua script raises, does the command pipeline emit a synthetic error SSE event and mark execID as failed, matching the shell exit-code path?

## Acceptance criteria

- A user can declare a command with `lua: my-command.lua` in `data-entry.yaml` and see the button on the configured screen(s)
- Clicking the button executes the Lua script inside the existing sandbox with `rela.context` populated according to the command's `context` type
- `rela.emit.message`, `rela.emit.error`, `rela.emit.log`, `rela.emit.file`, `rela.emit.open_url`, and the `rela.emit(...)` escape hatch each stream a corresponding SSE event to the browser without needing frontend changes
- `POST /api/command-cancel/{execID}` cancels a running Lua command; the script's next preemption point observes the cancellation and the SSE stream closes with a failure
- A config with both `script:` and `lua:` set on the same command is rejected at load time with a clear error
- A Lua command that raises surfaces as an error SSE event and marks the command as failed
- All existing shell command behavior is unchanged (regression tests still pass)
- Tests cover each acceptance criterion including one end-to-end test per `context` type (entity, list, view, global)
