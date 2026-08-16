---
id: TKT-YH52OM
type: ticket
title: 'Lua capability gating: http, ai, secrets, write_file are unconditional on every script surface'
kind: enhancement
priority: high
effort: l
status: backlog
---

## Problem

Every Lua script — on **every** surface — is handed the same capability set,
unconditionally:

| Binding | What it grants |
|---|---|
| `http.get/post/put/patch/delete/request` | arbitrary outbound HTTP |
| `rela.secrets` | the whole contents of `.rela/secrets.yaml` |
| `ai.chat` | billable LLM calls |
| `rela.write_file` | writes under `output/` |

`http` and `ai` are registered with an explicit "always registered" comment
(`internal/lua/runtime.go`, `registerHTTPModule` / `registerAIModule`); there is
no dep to gate them on.

The combination is the problem: **a script can read every secret and POST it
anywhere.** Not a hypothetical chain — two calls.

## Why this inverts the usual intuition

The instinct is "commands shell out, so they're the dangerous ones; actions are
just sandboxed Lua." That is backwards.

The Lua sandbox removes `load`/`dofile`/`loadstring`/`rawget` — it stops
arbitrary *code loading*, not *capability access*. Meanwhile a `command:` runs
under `internal/cmdexec` confinement: no network, temp-dir-only writes, rlimits,
process-group kill. A Lua action has **none** of that and has network access by
design.

So by capability, scripts are the stricter case, and `commands:` is the one
that's already confined.

`rela.bypass_acl` is the counter-example that proves the pattern works: it is
registered **only** when an `ElevatedManager` dep is wired (`runtime.go`,
TKT-D8T148), so a script cannot elevate unless an operator declared
`allow_acl_bypass` on that action. Capabilities should follow exactly this
shape.

## Scope: all script surfaces, not just actions

Six entry points share the runtime:

- `ExecuteAction` (HTTP `_action/` + webhook dispatch)
- `ExecuteDocument`, `ExecuteStandaloneDocument`, `ExecuteListDocument`
- `ExecuteCode`, `ExecuteFile`

Plus the scheduler and the automation engine. A document script can reach
`secrets` and `http` today just as an action can, so gating actions alone would
leave the same hole one endpoint over.

## Proposal

Per-script opt-in, following the `allow_acl_bypass` precedent: **config decides
whether the binding is registered at all**, so an ungated script cannot call
what it was never given.

```yaml
actions:
  notify_slack:
    script: notify.lua
    capabilities:
      http: true
      secrets: [slack_webhook_url]   # NOT a bool — see below
```

**`secrets` is a list, not a boolean.** A boolean grants the entire
`secrets.yaml`, so an action needing one Slack token would also receive the
database DSN and every API key. The list names exactly what is exposed; the
runtime builds a filtered map and the script sees nothing else.

Open design questions for the planning pass:

1. **Where does the declaration live per surface?** Actions and documents have
config blocks; `ExecuteCode`/`ExecuteFile` (CLI, scheduler) do not obviously.
2. **Default posture.** Fail-open preserves every existing script; fail-closed
is safer but breaks all of them at once. Given the capabilities are the actual
risk, gating them may make fail-open on *permissions* acceptable — that was the
reasoning for deferring the `permission:` decision.
3. **`write_file`** is already confined to `output/`; possibly lower priority
than the other three.
4. **Wiring shape.** `ElevatedManager` is a nil-able field on `ReadDeps`;
`http`/`ai` have no equivalent and would need one before they can be gated.

## Relationship to TKT-X06LA2

TKT-X06LA2 proposed gating *who may run an action*. That treats the symptom: it
leaves a secrets-capable, network-capable surface behind a permission an
operator must remember to set, and does nothing for the other five script
surfaces.

Gating the capabilities means an ordinary script cannot exfiltrate anything
regardless of who runs it. The `permission:` question then becomes a genuine UX
/ intent choice rather than the only thing standing between a caller and
`secrets.yaml`.

The DoS and read-gate halves of TKT-X06LA2 are already fixed (see that ticket).

## Origin

Discussion with the user on TKT-X06LA2. The capability framing, the all-surfaces
scope, and the secrets-as-a-list refinement are all theirs.
