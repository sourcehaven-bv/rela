---
id: TKT-X06LA2
type: ticket
title: 'Actions: gate entity_id on the read path, fix the writeMu DoS (permission gate deferred)'
kind: enhancement
priority: high
effort: m
status: in-progress
---

## Status

Two of the three problems are **fixed** in this PR. The third — a `permission:`
field gating *who may run an action* — is **deliberately deferred** to
TKT-YH52OM; see "Why the permission gate moved" below.

## Fixed: entity_id was a read-side ACL bypass

Found while implementing this ticket, and worse than the DoS it was filed for.

`handleV1Action` resolved the caller-supplied `entity_id` through the **raw
store** and handed the result to the script as the global `entity`
(`script.Engine.ExecuteAction` sets it) — no read gate, no field redaction. Any
caller who could POST an action could name any id and have that entity's full
properties, `visible:`-hidden fields included, placed in script scope and echoed
back through the action's own response.

Reproduced before fixing: a principal holding no role POSTs
`{"entity_id":"TKT-SECRET"}` and the response contains the ticket's title.

The gate now consults the **stored** type, never `req.EntityType`. Authorizing
against a caller-supplied type is a cross-type escalation — claim a type you may
read, name an id of a type you may not, and an AllowAll verdict on the claim
grants it. That is the defect BUG-ZWTDH9 fixed on the sync channel.

An absent id falls through with `ent` nil rather than 404ing, so the endpoint
does not become an existence oracle for ids the caller cannot read.

Tests, all mutation-verified (gating on the claimed type fails the escalation
test; removing the gate fails both leak tests):

- `TestAction_EntityIDRespectsReadGate`
- `TestAction_EntityIDCrossTypeEscalation`
- `TestAction_EntityIDPermittedStillWorks`

## Fixed: the writeMu DoS

The gate runs **before** `writeMu.Lock()`. Previously a denied caller still
acquired the process-wide write lock and burned the full `actionTimeout`, so an
unauthorized POST loop could serialize every writer in the process. Decide, then
work — the same ordering the document renderer uses.

## Why the permission gate moved to TKT-YH52OM

The original proposal (add `Action.Permission`, mirror `authorizeCommand`)
treats the symptom.

The real problem is not *who* may run an action — it is that **every** script,
on every surface, unconditionally gets `http.*`, `rela.secrets`, `ai.chat` and
`rela.write_file`. A script can read every secret and POST it anywhere, in two
calls. Gating who may run it leaves that surface behind a permission an operator
must remember to set, and does nothing for the other five script entry points
(documents, standalone documents, list documents, code, file) or the scheduler
and automation engine.

This inverts the intuition the original ticket was written on. "Commands shell
out, actions are just sandboxed Lua" is backwards: the Lua sandbox blocks
arbitrary *code loading*, not *capability access*, while a `command:` runs under
`cmdexec` confinement with no network. By capability, scripts are the stricter
case.

`rela.bypass_acl` already demonstrates the fix shape — registered only when an
elevated handle is wired, so a script cannot elevate unless an operator declared
it.

TKT-YH52OM carries that work, including the refinement that `secrets` must be a
**list of named secrets**, not a boolean: a boolean grants the whole
`secrets.yaml`, so an action needing one Slack token would also receive the
database DSN.

Once capabilities are gated, fail-open on `permission:` becomes defensible and
the decision is a UX/intent one rather than the only thing between a caller and
`secrets.yaml`.

## Webhook path: exempt, deliberately

`dispatchWebhookAction` (`webhook.go`) is reachable only with a signed webhook
token and runs under a synthetic `webhook:<event>` principal that holds no
roles. A permission check there would deny every webhook and break IdP
provisioning outright — the token *is* the authorization. It takes no
caller-supplied `entity_id`, so the read-gate fix does not apply to it either.

## Origin

Found by the code-review survey while planning TKT-TXDK8U.
