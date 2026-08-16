---
id: TKT-X06LA2
type: ticket
title: 'Actions: gate entity_id on the read path, fix the writeMu DoS (permission gate deferred)'
kind: enhancement
priority: high
effort: m
status: in-progress
---

## Problem

`handleV1Action` (`internal/dataentry/actions.go:45`) runs a configured action —
a Lua script, or a declarative `set:` mutation — with **no per-principal
authorization check at all**. The handler validates the method, the id shape,
and looks up the config; then it takes `writeMu` and calls
`engine().ExecuteAction`. There is no `readGateFromContext`, no
`acl.WriteRequest`, no `authorizeCommand` equivalent.

Contrast `commands:`, which has `Permission` on the config
(`dataentryconfig/config.go:606`) and a real gate in `authorizeCommand`
(`internal/dataentry/commands.go:84-120`), re-consulted at exec time. `Action`
has no authorization field whatsoever (`dataentryconfig/config.go:100-108`).

## Impact

The enforcement that does exist is indirect and downstream: the script's writes
go through `EntityManager` and its reads through the ACL-bound `scriptReader`,
so under a restrictive policy the *effects* are largely denied. But:

- The action always **starts**, always burns the 5s `actionTimeout`, and always
holds `writeMu` — a denied principal can serialize every writer in the process
by POSTing in a loop.
- A `set:` action's mutation and any script side effect that isn't an entity
write are not covered by that downstream gating.
- Under `--read-only` the action still executes; only its writes fail.

There is a **second entry point**: `dispatchWebhookAction`
(`internal/dataentry/webhook.go:153-183`) runs actions under a synthetic
`principal.Principal{User: "webhook:" + claims.Event}`, gated only by webhook
token validation. Any fix has to cover both.

## Proposal

Add `Permission string` to `Action` and gate `handleV1Action` on it, mirroring
`authorizeCommand`:

- **Closed switch over the `acl.ACL` implementation**, so a new implementation
denies until someone adds an arm (`commands.go:72-83`).
- **Match both value and pointer forms** — `AuthorizeWrite` has a value
receiver, and matching only the value form once dropped `&acl.ReadOnlyACL{}`
into the granting default arm: a silent `--read-only` bypass reachable by one
`&`.
- **`ReadOnlyACL` denies every action, in every context.**
- **Do not key off the read gate alone.** `readGateFromContext` returns
`nopReadGate` under *both* NopACL and ReadOnlyACL, and
`nopReadGate.HoldsPermission` returns `true` — that combination was live bug
RR-CWWJGW. The ReadOnlyACL arm must be independent and come first.
- Decide the fail-open/fail-closed posture for an action with no `permission:`
under a configured policy. Commands fail **closed** there
(`commands.go:112-114`) on the grounds that a command shells out; an action runs
Lua with write access, so the same argument applies — but it is a breaking
change for existing configs and needs a deliberate call.
- Cover `dispatchWebhookAction` too, or document explicitly why the webhook
principal is exempt.

## Relationship to TKT-TXDK8U

TKT-TXDK8U (nav filtering) hides an `action:` entry the user cannot use, keyed
on a `permission:` declared **on the nav entry**. That is presentation only and
does not close this hole: the menu hides the button, but `POST
/api/v1/_action/{id}` still executes for anyone who posts to it.

If this ticket adds `Action.Permission`, TKT-TXDK8U's nav-entry `permission:`
could later derive from it rather than being restated — see the "future: derive
from target" note there.

## Origin

Found by the code-review survey while planning TKT-TXDK8U. Split out at the
user's direction: a missing authorization gate is a security fix, not a tail on
a UX ticket.
