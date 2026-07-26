---
id: TKT-ACSBSA
type: ticket
title: 'lua: extend rela.bypass_acl''s admin handle with read methods (elevated reads, closure-scoped)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Follow-up to TKT-ZF2DTV (DEC-O59WM4). Once Lua reads are ACL-bound to the acting
principal, a script that legitimately needs to read beyond its caller's view has
no escape hatch. Add read methods to the **existing** `admin` handle that
`rela.bypass_acl(fn)` already provides for writes, rather than introducing a
second escalation mechanism.

```lua
rela.bypass_acl(function(admin)
  local e = admin.get_entity("PERSON-1")   -- raw, unredacted
  ...
end)
-- admin is dead here; rela.get_entity remains scoped to the caller
```

## Why this shape

`bypass_acl` is an **object capability scoped to a closure** (TKT-D8T148):
elevation is never ambient, the surrounding `rela.*` bindings stay gated even
inside the closure, the handle self-invalidates on exit (`live=false` on every
path), and it is two-key — the operator sets `allow_acl_bypass` on the action
AND the Mutator must offer `ElevatedProvider`. Extending it keeps one mechanism,
one mental model, and makes the privilege legible **at the call site** instead
of ambient for a whole script (the reason per-action `roles:` config was
rejected in DEC-O59WM4).

## Scope

- Add `get_entity`, `list_entities`, `get_relations` to `newElevatedHandle`
(`runtime.go:1561`), routed to the RAW store handle, subject to the same `live`
guard as the existing write methods.
- Decide and document the contract: elevated reads return **raw** entities
(full properties, no row gate) — a half-elevated read is a confusing contract,
and the closure is already the boundary.
- The handle needs a raw read dependency; `WriteDeps` currently carries
`ElevatedManager` (writes) only. Thread the raw store handle that TKT-ZF2DTV's
ReadDeps split already keeps for write-prep.
- Tests: elevated read returns hidden properties; the same call OUTSIDE the
closure (via `rela.get_entity`) does not; handle captured into a global and used
later raises; no elevation registered when `allow_acl_bypass` is unset.
- Audit/observability: consider whether an elevated read should leave a trace
(writes already do via the audit log; reads have no equivalent today) — at
minimum a decision recorded, not silently omitted.

## Non-goals

Runtime-wide read elevation, config-declared read roles on actions, or any
ambient mode — all rejected in DEC-O59WM4. `create_entity`/`update_entity` on
the elevated handle remain deferred (their own surface, per the existing godoc
at `runtime.go:1568-1573`).

## References

- `internal/lua/runtime.go:1519` (`luaBypassACL`), `:1561` (`newElevatedHandle`)
- DEC-O59WM4, TKT-D8T148, TKT-ZF2DTV
