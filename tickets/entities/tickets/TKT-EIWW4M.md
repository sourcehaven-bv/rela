---
id: TKT-EIWW4M
type: ticket
title: 'acl: fail-closed script read wiring in data-entry (rela#1198)'
kind: enhancement
priority: medium
effort: s
status: done
---

## Summary

Closes rela#1198 (IB review, CONTROL-5-15). `dataentry.App.scriptReader` /
`scriptTracer` degrade to the **raw ungated store** on an ACL-gate construction
fault, with only a `slog.Warn`. The equivalent helpers in `appbuild` make the
opposite choice on identical error branches — `slog.Error` +
`visibility.DenyReader{}` / `DenyTracer{}` (RR-GKCZO5).

## Premise corrections (verified against develop @ dd0fe649)

The issue's two headline claims are **wrong**. Both were checked in code and
independently re-verified before planning.

### Correction 1: MCP is NOT served by this wiring

The issue's strongest argument is that MCP is "een machine-naar-LLM-kanaal, geen
mens die de warn-log direct ziet". That argument points at code that does not
exist.

- MCP builds its own services: `appbuild.Discover(..., WithACL(acl.NopACL{}))`
(`internal/cli/mcp_wiring.go:43`) and passes `svc.LuaWriteDeps()` — the
**unrestricted** accessor (`appbuild.go:310`, doc: "UNRESTRICTED reads") — into
`mcp.Deps` (`mcp_wiring.go:66`).
- `internal/mcp` never imports `internal/dataentry`; no entry point wires
both into one process (`cmd/rela-server`, `cmd/rela-desktop` construct
`dataentry.NewApp` and never import `internal/mcp`).

So MCP reads unrestricted **by deliberate, documented design** — the local stdio
transport means the filesystem is already the trust boundary
(`mcp_wiring.go:34-41`) — not via a fail-open fallback. Changing `dataentry`
would not alter MCP behavior by one byte. Whether NopACL-for-MCP is right is a
separate question, out of scope here.

### Correction 2: the fail-open branches are currently UNREACHABLE

All three `scriptReader` error branches are dead code at this call site:

| Constructor | Only errors when | Reachable? |
|---|---|---|
| `NewDeclarativeGate` | `d == nil` | No — guarded by the `!ok \|\| d == nil` early return above it |
| `NewPolicyReader` | gate/redact/get nil | No — gate is a non-pointer struct value; redactor is a struct literal; `a.store` is nil-rejected in `NewApp` |
| `NewScriptReader` | reader/raw nil | No — both non-nil per above |

This is therefore **latent exposure, not a live leak**. It activates on a future
refactor (a nilable store/redactor, or a new error condition in any of the three
constructors) — exactly the kind of silent activation that is cheap to prevent
now and expensive to detect later.

## What IS real: the webhook path

The issue's conclusion is right for a reason it did not identify.
`App.luaWriteDeps` has exactly three consumers:

- `actions.go:94` — interactive HTTP action (human at the UI)
- `document.go:268` — `export_render`
- **`webhook.go:181` — the IdP webhook: unattended machine-to-machine**

The webhook runs as a real stamped principal (`User: "webhook:"+claims.Event`,
`Tool: ToolWebhookReceiver`) with **no human in the loop** — its own godoc says
"not a human at the UI". The "an operator sees the outage immediately" rationale
that justifies fail-open for interactive requests does **not** hold here. That
is the genuine asymmetry, and it is sufficient on its own.

## Approach

Make all three consumers fail closed, matching `appbuild`. The
`DenyReader`/`DenyTracer` machinery already exists and is already the precedent
— this is a wiring change, not new infrastructure.

Rationale for not splitting per-consumer: an interactive caller getting
`ErrReaderUnavailable` is a loud, immediate, diagnosable outage — which is the
*stated goal* of the fail-open argument, achieved without the ungated read.
There is no case where returning the raw store is preferable to a clear error,
so the added complexity of a per-consumer policy buys nothing.

## Acceptance criteria

- `scriptReader` returns `visibility.DenyReader{}`, `scriptTracer` returns
`visibility.DenyTracer{}`, on any construction fault; logged at `slog.Error`,
wording matched to `appbuild`.
- NopACL path (`d == nil`) still returns the raw store — unchanged.
- A genuine DENY remains a deny, distinct from `ErrReaderUnavailable`.
- Tests pin fail-closed for the reader and tracer, and pin that the
policy-less path stays unrestricted.
- Issue #1198 answered with both premise corrections stated explicitly.

## Non-goals

MCP's `NopACL` wiring (deliberate; separate decision). `export_render`
output-side redaction. Field-level redaction on scheduled jobs (RR-7408F5).
