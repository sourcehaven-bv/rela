---
id: TKT-AQIT9M
type: ticket
title: 'Command exec ungated under default NopACL: refuse on non-loopback bind, with an explicit override flag'
kind: enhancement
priority: high
effort: m
status: done
---

<!-- @managed: claude-workflow v1 -->

## Description

`authorizeCommand` (DEC-EIHQSU, `internal/dataentry/commands.go:84`) now gates
`commands:` execution and is fail-closed for nil / unknown-ACL / `ReadOnlyACL` /
`Declarative`-without-permission. **But its `acl.NopACL` arm returns `true`
unconditionally**, and `NopACL` is the default: `buildACL(policy=nil)` returns
`acl.NopACL{}`, and `policy` is nil whenever no `acl.yaml` is present.

Result: a `rela-server` with no `acl.yaml` runs every configured command ungated
— arbitrary `sh -c` for any authenticated session. A **non-loopback** bind
without `acl.yaml` is only *warned about* (`cmd/rela-server/main.go:333,355`),
not refused. So the deployment shape in TKT-PYPNWO (headless remote behind
oauth2-proxy, plausibly no `acl.yaml`) has fully open shell execution behind
only the proxy.

Not a regression from the gating work — it's the pre-existing NopACL behavior.
This ticket makes the network default fail closed, refactors the decision to the
seam, and provides a deliberate override for legitimate single-user network
deployments.

## Chosen approach — two-impl authorizer at the seam + explicit override flag

### The two-impl seam (the pattern you asked for)

Mirror `scriptEntityReader` / `scriptTracer` in `internal/appbuild/appbuild.go`:
inspect the ACL **once at construction**, hand the consumer one of {ungated,
gated, deny}. The consumer holds an interface and never type-switches.

1. **Narrow consumer-side interface** in `internal/dataentry`:
   ```go
   type commandAuthorizer interface {
       Authorize(ctx context.Context, cmd CommandConfig) bool
   }
   ```
`commandHandler` holds a `commandAuthorizer` instead of `aclImpl func()
acl.ACL`; `handleCommandExec` / `resolveCommands` call `h.authz.Authorize(...)`.

2. **Implementations:**
   - `ungatedAuthorizer` → always `true`. NopACL path, byte-identical to today's
local behavior. Named so every bypass is greppable (cf. TKT-1WV50C).
   - `gatedAuthorizer` → wraps the Declarative read gate; enforces `cmd.Permission`
(+ the deferred `context: view` deny). The current Declarative arm's body.
   - `denyAuthorizer` → always `false`. The `DenyReader` analogue for
"exec must not happen here" (ReadOnly; network-no-policy without override; gate
required-but-unbuildable).

3. **Selection at the wiring site** (`if d == nil { ... }` shape):
   - Declarative policy present → `gatedAuthorizer`.
   - No policy + **loopback** bind → `ungatedAuthorizer` (desktop/dev unchanged).
   - No policy + **non-loopback** bind → `denyAuthorizer` **unless overridden** (below).
   - `ReadOnlyACL` → `denyAuthorizer`.
   - Gate required but unbuildable → `denyAuthorizer`.

### The override (decision A: hard refusal by default, opt-out flag)

Refusal is the *default* on a network bind; a host that isolates at another
layer opts back in with a deliberate, greppable flag — **exact precedent:
`--unconfined-commands` / `RELA_UNCONFINED_COMMANDS=1`** (`main.go:91`), which
does the same thing for the sandbox dimension of the same shell-exec surface.

- New flag `--allow-unauthenticated-commands` (env `RELA_ALLOW_UNAUTHENTICATED_COMMANDS=1`),
help text modeled on `--unconfined-commands`: names the risk, states when it's
legitimate (single-user deployment isolated by Docker port-publishing / host
firewall / another auth layer), points to `docs/server-security.md`.
- Effect: on non-loopback + no-policy, the flag selects `ungatedAuthorizer`
instead of `denyAuthorizer`. It is the ONLY way to get ungated commands on a
network bind. Loopback never needs it; a configured policy never needs it.
- Log LOUDLY at startup when the override is active (like the non-loopback
warnings already do), so it shows up in every boot log.

**Why a flag, not bind-sniffing alone:** the Docker case binds `0.0.0.0` so the
published port reaches the host, but the real trust boundary is Docker's port
mapping / host firewall, not the bind address. Bind-based refusal would break
that legitimate deployment; the override lets the operator assert "I isolate
elsewhere" explicitly rather than the platform guessing. Same reasoning
`--unconfined-commands` already encodes.

### Why this over the earlier options

The three earlier candidates all bolted a special case onto the existing
type-switch. Two impls behind an interface (a) removes the type-switch from the
hot path, (b) makes "which authorizer" one greppable wiring line, (c) matches
the read-gate seam, (d) puts the loopback/remote + override policy in
`cmd/rela-server` where the bind and flags already live, not in `dataentry`.

## Files (anticipated — confirm in planning)

- `internal/dataentry/commands.go` — replace `authorizeCommand` type-switch with
the `commandAuthorizer` interface + `ungated`/`gated`/`deny` impls
- `internal/dataentry/command_handler.go` — hold `commandAuthorizer`, drop
`aclImpl func() acl.ACL`
- `cmd/rela-server/main.go` — `--allow-unauthenticated-commands` flag + env
fallback (beside `--unconfined-commands`); select the authorizer from (policy,
bind, override) in/near `discoverOptions`; loud startup log
- `internal/appbuild/appbuild.go` — likely a new `Option` carrying the authorizer
choice (parallel to `WithACL`), or expose the selection helper
- `docs/server-security.md` — document the network default + the override, next to
the `--unconfined-commands` and non-loopback sections
- Tests: preserve `TestCommandExecReadOnlyDenied` + the NopACL-grants canary; add
{non-loopback + no-policy → denied}, {…+ override → allowed}, {loopback +
no-policy → allowed}

## Open sub-question for planning

Desktop (`rela-desktop`, Wails, no network listener) must stay ungated — it has
no bind at all. Confirm the selection treats "no HTTP listener" as
loopback-equivalent (it should: the Wails asset server is in-process, strictly
local).

## The precise gap (reference)

```go
// commands.go — authorizeCommand (current)
case acl.NopACL, *acl.NopACL: return true   // every command runs
```

```go
// appbuild.go — buildACL
if policy == nil { return acl.NopACL{}, nil, nil }   // nil when no acl.yaml
```

## Repro

1. `rela-server --bind 0.0.0.0:8080`, project has a `commands:` entry, no `acl.yaml`.
2. POST `/api/command/<id>` from any browser reaching the server.
3. The command runs — NopACL arm returns true.

## Version

Verified on `develop` @ `dd0fe649` (after rebasing in the `authorizeCommand`
gating work). Gating is correct for configured-ACL deployments; this ticket is
the no-policy network default + override.
