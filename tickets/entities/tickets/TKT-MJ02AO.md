---
id: TKT-MJ02AO
type: ticket
title: 'Per-command ACL guard: gate command execution and button visibility on a named permission (entity/list/global; view deferred)'
kind: enhancement
priority: high
effort: l
status: done
---

## Description

Data-entry commands (`data-entry.yaml` `commands:`) execute arbitrary shell via
`sh -c` and currently have **no authorization whatsoever**. This ticket adds a
configurable per-command ACL guard that gates both execution (server-side,
authoritative) and button visibility (client-side, cosmetic).

Prep work for TKT-72SCPR, which renders command buttons on the dashboard and
list surfaces. That ticket is blocked on decision A of PLAN-CNDJ78; this ticket
is that decision.

**Fine-grained `permission:` control covers `entity`, `list` and `global`.
`view` gets no fine-grained control in this ticket — it fails closed under a
configured `acl.yaml`** (see "Deferred: view context").

## Motivation — verified current state

`handleCommandExec` (`internal/dataentry/commands.go:281`) does exactly four
things before spawning a shell: HTTP method check, command-ID lookup in
`s.Cfg.Commands`, stdin build, then `exec.CommandContext(r.Context(), "sh",
"-c", cmd.Script)` at `commands.go:360`.

Grepping both command files for authorization identifiers:

```
grep -n "acl|ACL|translateVerb|WriteRequest|ReadOnly|readOnly|Principal" \
  internal/dataentry/commands.go internal/dataentry/command_handler.go
(no matches)
```

The only gates on `/api/command/` are `requireSameOrigin` and `requireLocalHost`
(`router.go:105-108`) — CSRF and network-location controls. They answer "did
this request come from our own page on this host", never "may *this principal*
do this". `attachACLRequest` (`router.go:157`) attaches an `acl.Request` to
context for downstream consumers; `handleCommandExec` never reads it.

**Consequences today:**

1. **No principal-aware authorization.** Any client that can reach the server
and knows a command ID can execute it. (RR-65KG68)
2. **`--read-only` does not block command execution.** `acl.ReadOnlyACL`
(`internal/acl/readonly.go:18`) implements exactly one method,
`AuthorizeWrite(WriteRequest)`. Command exec constructs no `WriteRequest`, so
read-only is never consulted. (RR-CWWJGW)
3. **`available_on` is display scoping, not a boundary.** `matchesPage` runs
only on the GET `/_commands` render path. Exec reads
`entity_id`/`list_id`/`view_id` straight from the query string and never
validates them against `cmd.AvailableOn`. (RR-L6UXCF)

## Design: reuse the named-permission pattern

The `acl.ACL` interface is `Subject`-shaped:

```go
type ACL interface {
	AuthorizeWrite(ctx context.Context, req WriteRequest) Decision
}
```

`Subject` is a **sealed sum** (`internal/acl/subject.go:20`, unexported
`isSubject()`) with only `EntitySubject` and `RelationSubject`. `Op` has only
four verbs: `OpCreate`/`OpUpdate`/`OpDelete`/`OpRename` (`acl.go:85-88`).

**A command does not always have an entity subject** — `context: global` and
`context: list` act on the project or a type-set, not a row. So this does not
fit `AuthorizeWrite` and must not be forced into it (that would mean unsealing
`Subject` or inventing a fake one).

**The right precedent already exists in-tree:** `acl.PermHistoryRead`
(`internal/acl/policy.go:43`, `"history:read"`) is a *global named permission*
granted via a role's `permissions:` list, used for exactly this shape — a
capability with no entity to evaluate against. It is checked via the read gate:

```go
// internal/dataentry/history_handler.go:122
if !gate.HoldsPermission(ctx, acl.PermHistoryRead) {
```

Commands should follow the same pattern.

## Proposed approach

**Config** — new optional `permission:` key on `CommandConfig`
(`internal/dataentryconfig/config.go:563`):

```yaml
commands:
  nightly-export:
    label: "Nightly export"
    context: global
    script: "./scripts/export.sh"
    permission: "command:nightly-export"   # NEW
```

Grant in `acl.yaml` via the existing role `permissions:` list — no new policy
machinery.

### Fail policy — AGREED, and uniform across all four contexts

**One rule, no per-context exceptions:**

- Under `NopACL` (no `acl.yaml` at all) → **fail-open**. Every existing
deployment behaves exactly as today.
- Once an `acl.yaml` exists → **fail-closed**. A command the principal cannot
be shown to be authorized for is denied.

Applied per context:

| context | `permission:` set | no `permission:`, `acl.yaml` present |
|---|---|---|
| `entity` | grant required | **denied** |
| `list` | grant required | **denied** |
| `global` | grant required | **denied** |
| `view` | **n/a — key not honored** | **denied** |

`view` is not an exception to the fail policy; it is an exception to
*fine-grained control*. Under a configured `acl.yaml`, view commands are denied
outright — there is no `permission:` escape hatch for them until the traversal
scoping question is resolved. That is the strict reading of "nothing on views":
no gated execution, not merely no per-command grant.

Under `NopACL`, view commands continue to execute exactly as today, consistent
with the fail-open half of the rule.

**Migration note:** a deployment adding its first `acl.yaml` must add
`permission:` keys and grants for the commands it wants to keep working, and
will lose view commands entirely until view support lands. Deliberate breaking
change; the failure mode is a denied command rather than an unguarded one.
Record as a `decision` entity during planning.

**Enforcement (authoritative)** — in `handleCommandExec`, before the context
switch at `commands.go:301`:

- Resolve the effective permission per the table above; deny with 403 if the
principal does not hold it.
- `context: view` under a configured `acl.yaml` → 403 unconditionally.
- Under `ReadOnlyACL`, deny **all** command execution regardless of
`permission:` or context. Closes RR-CWWJGW and makes
`e2e/tests/read-only-mode.spec.ts:8`'s "403 at the server on click" claim true
for command buttons, which it currently is not.

**Resolution (cosmetic)** — `resolveCommands` (`commands.go:47`) filters out
commands the principal cannot execute — including all view commands under a
configured `acl.yaml` — so `GET /_commands` returns only executable ones and
buttons never render. UX affordance, not a security boundary; the 403 is the
boundary.

**Consider bundling RR-L6UXCF** (~10 lines): call `matchesPage` in
`handleCommandExec` with the supplied params and 403 on mismatch, making
`available_on` an actual boundary rather than display-only. Same file, same
review, closes a documented lie.

## Deferred: view context

`view` gets **no fine-grained control** here. Under `NopACL` view commands run
unchanged; under a configured `acl.yaml` they are denied outright.

**Why.** A view command's payload is not one entity. `executeView`
(`views.go:19-49`) runs a multi-pass graph traversal (up to 10 passes,
`views.go:33-43`) and `buildViewInput` (`commands.go:181-215`) hands the script
`Collections` — every entity the traversal reached — plus the relations between
them. One `view_id` + one `entity_id` yields an arbitrarily wide slice of the
graph, and `executeView` reads the store directly with **no read-gate scoping**.

A `permission:` grant on a view command would therefore confer read access to
the whole traversal closure, not just the entry entity — a materially broader
grant than the same permission elsewhere, and one operators cannot reason about
from the config. Rather than ship a grant whose blast radius is unclear, view
commands are denied under any configured policy.

Note the entry *is* a real, type-checked entity (`views.go:24` rejects a
mismatched type; `buildViewInput` sets `Entity: vr.Entry`), so this is not a
"view has no subject" problem — it is a blast-radius problem.

**Follow-up obligations when view support is picked up:**

- Decide whether `executeView` should be read-gate scoped for command exec
- Document what a view-command permission confers
- Then honor `permission:` for `view` and lift the unconditional deny

## Scope

**In scope:**

- `permission:` key on `CommandConfig` + `validateCommands` support
- Uniform fail policy (fail-open under `NopACL`, fail-closed with `acl.yaml`) — record as a `decision` entity
- 403 enforcement in `handleCommandExec` for `entity`, `list`, `global`
- Unconditional 403 for `context: view` under a configured `acl.yaml`
- Blanket deny of command exec under `ReadOnlyACL` (all contexts)
- Permission-aware filtering in `resolveCommands`, including view suppression under a configured policy
- `validateCommands` warns when `permission:` is set on a `context: view` command (the key is not honored — silently ignoring it would mislead)
- Go tests: allowed/denied per in-scope context, view deny under policy, view unchanged under `NopACL`, read-only deny, filtered resolution, both halves of the fail policy
- Docs: `docs/data-entry.md` (the `permission:` key, and that view is not gateable yet), `docs/acl-security.md` (the named-permission family), correct the read-only-mode deferred-sites note

**Out of scope:**

- Fine-grained `permission:` support for `view` — deferred, see above
- Read-gate scoping of `executeView`
- Rendering commands on dashboard/list surfaces (TKT-72SCPR)
- Lua command backend (TKT-XTNED)
- Authentication — rela-server still has no auth layer (`policy.go:30-32`); this gates on whatever principal is stamped today
- Per-command *argument* policy beyond the `matchesPage` check

## Relationship to TKT-72SCPR

TKT-72SCPR is blocked on PLAN-CNDJ78 decision A. **This ticket resolves it as
option 1** (gate exec on ACL) rather than option 3 (document as operator-trust).
It should land first so TKT-72SCPR renders buttons against a gated surface, and
so `CommandButtons.vue` inherits permission-filtered results for free.

The view deferral is aligned across both tickets: TKT-72SCPR also drops its view
acceptance criteria, retiring the unresolved `pageType: 'view'` behavioral-delta
decision (RR-N643V4).

RR-65KG68, RR-CWWJGW and RR-L6UXCF are filed against TKT-72SCPR and *addressed*
here. When this lands, update those three with a resolution pointing at this
ticket.

## Acceptance criteria

- A command with `permission: X` is executable by a principal holding X and 403s for one that does not, for each of `entity`, `list`, `global`
- A command with `permission: X` is absent from `GET /_commands` for a principal lacking X, and its button does not render
- With no `acl.yaml` present, a command with no `permission:` executes exactly as today, in **all four** contexts including `view` (fail-open regression test)
- With an `acl.yaml` present, a command with no `permission:` is denied (fail-closed test)
- With an `acl.yaml` present, a `context: view` command is denied **even when `permission:` is set and granted**, and is absent from `GET /_commands`
- `validateCommands` warns when `permission:` is set on a `context: view` command
- Under `rela-server --read-only`, **every** command 403s on exec regardless of `permission:` or context, and no command buttons render
- `docs/acl-security.md` documents the command-permission family alongside `history:read` and states that `view` cannot yet be gated per-command
- `e2e/tests/read-only-mode.spec.ts`'s deferred-sites comment is corrected for command buttons
