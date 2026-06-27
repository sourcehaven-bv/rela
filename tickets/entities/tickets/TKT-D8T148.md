---
id: TKT-D8T148
type: ticket
title: 'ACL-bypass automation scripts: rela.bypass_acl(closure) with a scoped, invalidated-after elevated write handle'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Automation Lua scripts run through `entitymanager.Manager` (the cascade Mutator
IS the Manager — `manager.go:320/451`), so their `rela.create_relation` /
`create_entity` writes are **ACL-checked against the triggering principal** via
`Manager.authorizeAndAudit` (the single authz chokepoint). This blocks a
legitimate pattern: a script enforcing a **system invariant** on behalf of a
user who isn't authorized to make that write directly.

Motivating case (submitter→triager→team ACL flow): on ticket create, an
automation stamps `submitter --created-by--> ticket` so the submitter can read
their own ticket via ACL. The script reads the submitter from
`rela.principal.user` (TKT-5U6NRR), but `created-by` is gated on source type
`person`, and the submitter has no `write: [person]` (nor should they). Denied.

## API — `rela.bypass_acl(closure)` with a separate elevated handle

Elevation is **lexically scoped** to a closure and carried by a **distinct
capability object** (object-capability), not an ambient flag:

```lua
rela.bypass_acl(function(admin)
  -- elevated ONLY inside this closure, ONLY via `admin`
  admin.create_relation(rela.principal.user, "created-by", entity.id)
end)
-- back to gated authority here; rela.* was never elevated
```

Decisions:
- **Separate handle** (`admin`): the handle IS the capability. `rela.*` write
bindings stay gated *always*; you cannot make an elevated write without naming
`admin`. This is what makes it leak-proof (see below).
- **Operator gate `allow_acl_bypass: true`** on the automation action (metamodel,
operator-only). Without it, `rela.bypass_acl` is absent (nil) → calling it
errors. So elevation needs BOTH operator blessing (unlock) AND explicit script
use (where). v1 is blanket (all write types); a relation/entity **type
allowlist** (`allow_acl_bypass: [created-by]`) is a deferred follow-up.
- **Handle invalidated after the closure returns**: once `fn` returns, `admin`'s
methods raise. A script that stashes `admin` in a global and calls it later gets
a dead handle — the lexical scope is enforced, not conventional (mirrors the
frozen `rela.principal`).

## Leak-proofing (the load-bearing property — from go-architect review)

A ctx-marker approach was REJECTED: `WithElevated(ctx)` would leak into the
nested cascade the elevated write dispatches (`Cascade.Process(ctx, ...)`,
manager.go:315/446 — same ctx, never cleared), elevating all downstream writes.

The handle approach is leak-proof by construction: the elevated authority lives
only in `admin` inside `fn`. Any nested cascade an elevated write triggers
re-enters `Manager.Process` with `Mutator: m` (the GATED Manager) — elevation
does not propagate to descendants. Plus handle-invalidation kills the
escaped-handle vector.

## Scope

- `allow_acl_bypass bool` threaded: metamodel `AutomationAction` yaml →
`automation.Action` → `LuaToExecute` → `autocascade.ScriptAction`.
- An **elevated Mutator**: an `entitymanager` write handle whose
`CreateRelation` / `CreateEntity` / etc. skip the `authorizeAndAudit` deny.
Short-circuit ABOVE `recordDeniedWrite` (manager.go:166) so no denied-write row
is recorded; instead record ONE audit row with the **real principal**
(`principal.From(ctx)`) + a greppable `acl_bypass=true` marker (alongside the
existing `triggered_by=automation:<name>`).
- Lua: register `rela.bypass_acl(fn)` ONLY when the runtime is built for an
allow_acl_bypass action. It calls `fn(admin)` where `admin` is a Lua table
backed by the elevated Mutator; after `fn` returns (or errors), invalidate
`admin`. `admin` exposes only the Mutator surface (create/update/delete
entity+relation), never reads, never the ungated `Host`.
- Errors inside the closure propagate (a failed elevated write must surface, not
silently no-op).

## Tests

- (a) elevated `admin.create_relation` succeeds where the triggering principal's
gated `rela.create_relation` is denied (created-by from a person the user can't
write).
- (b) audit row names the real principal + `acl_bypass=true`; no denied-write row.
- (c) NON-elevated script (no allow_acl_bypass) has no `rela.bypass_acl` and its direct write
is still denied.
- (d) **leak test**: a nested cascade triggered by an elevated write is NOT
elevated (a gated write downstream is still denied).
- (e) **escaped-handle test**: `admin` captured into a global and called after
the closure returns raises (invalidated).

## Out of scope / future

- Type allowlist (`allow_acl_bypass: [type, ...]`).
- Unifying the existing implicit declarative `cascadeHost` ACL-bypass under this
same explicit concept (document the inconsistency now, unify later).

## Security notes

- Elevation is on the **script** (operator-authored, metamodel-only), gated by
`allow_acl_bypass`, scoped to a closure, carried by a capability object, and the
handle dies after the closure. Four independent constraints.
- More auditable than the status quo (declarative bypass is silent + unmarked).
- Real identity never lost; the frozen `rela.principal` blocks self-reattribution.
