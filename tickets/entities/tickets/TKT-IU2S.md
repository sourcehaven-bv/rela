---
id: TKT-IU2S
type: ticket
title: Delegate wsEntityManager to entitymanager.Manager (wire Manager into production)
kind: refactor
priority: high
effort: m
status: done
---

## Summary

`internal/entitymanager.Manager` (TKT-QTNX, PR #702) is the new production write
path — typed dependencies, automation+cascade orchestration, typed errors. But
it isn't wired anywhere yet: every consumer reaches EntityManager via
`ws.EntityManager()`, which returns `wsEntityManager`, which delegates to
workspace's own `createEntity` / `updateEntity` / `deleteEntity`. The new
Manager sits next to that path, unused.

This ticket flips the wiring so `wsEntityManager` delegates to
`*entitymanager.Manager`. After it lands:

- Workspace's own `createEntity`, `updateEntity`, `deleteEntity`,
`createEntityCore` and their automation/cascade plumbing become dead code and
are removed (~300-400 LOC).
- The new Manager is exercised by the entire MCP / CLI / dataentry
surface immediately, even before per-command migrations.
- Per-command migration tickets (TKT-KWAX, TKT-0SP1, TKT-9JEI, TKT-2IAC)
become mechanical: replace `ws.EntityManager()` with a directly- constructed
`*entitymanager.Manager`. No pipeline rewiring required.

## In scope

- Workspace constructs one `*entitymanager.Manager` at New() time with
the same `Deps` shape (Store, Meta, Templater, Automations, Cascade,
ScriptRunner).
- `wsEntityManager` becomes a thin forwarding adapter: CreateEntity /
UpdateEntity / DeleteEntity / RenameEntity / CreateRelation / UpdateRelation /
DeleteRelation all forward to the held Manager.
- Workspace's `createEntity` / `updateEntity` / `deleteEntity` /
`createEntityCore` and their automation-result merging code are deleted.
- Workspace's `autocascade_host.go` is deleted (Manager owns its own
cascadeHost; workspace no longer needs to satisfy autocascade.Host).
- workspace.go drops its `runner *autocascade.Runner` field and
associated wiring (`newWorkspace` lines 304-327).
- arch-lint workspace.mayDependOn pares back where it can.
- All workspace tests that pin createEntity / updateEntity / cascade
behavior continue to pass — they're now testing through the Manager.

## Out of scope

- Removing `wsEntityManager` adapter entirely (TKT-64R3 will, when
Workspace itself is deleted).
- Per-command migrations (TKT-KWAX, TKT-0SP1 etc.) — they get cheaper
but this ticket doesn't touch them.
- Audit / principal / policy plumbing.
- Renaming or changing `EntityManager` interface shape.

## Depends on

- TKT-QTNX (entitymanager.Manager exists). PR #702.

## Risks

- **Test churn.** workspace's create/update/cascade tests exercise the
legacy path. They should keep working after the flip (Manager implements the
same pipeline shape), but anything that asserts on a workspace-internal hook or
method name will break. Audit before deleting the legacy methods.
- **CreateOptions field mapping.** wsEntityManager today drops `Variant`
with a `_ = opts.Variant` comment because workspace's CreateOptions doesn't take
it. Manager's CreateOptions does. Once we flip, Variant starts working — which
is desirable but means tests that assumed variant was a no-op may now exercise
template variants. Search for callers that pass Variant.
- **rename signature mismatch.** wsEntityManager.RenameEntity today
calls `m.w.rename(current.Type, oldID, newID, ...)` and looks up current type to
pass entityType. Manager.RenameEntity takes no entityType. We need to confirm
cli/rename.go and any other callers don't depend on the type-mismatch error that
workspace.rename surfaced.
- **Cascade observability.** Workspace's tests that count
ws.lookupEntity calls etc. are reading the same store. Should still work but
tests that mock specific workspace-internal methods need updating.
- **Lua write deps.** newLuaScriptRunner today reads `w.scriptExec` +
`w.LuaWriteDeps()` per-call. Manager's ScriptRunner is supplied at construction.
Need to either pass a per-call ScriptRunner via a Deps field that re-resolves,
or hold a workspace-level adapter that closes over the workspace state. Both
work; pick the simpler shape during planning.

## Acceptance criteria

1. `internal/workspace.Workspace` holds `*entitymanager.Manager` and
constructs it in `New()`.
2. `wsEntityManager` methods forward to Manager (no business logic in
the adapter beyond minimal field translation).
3. `workspace.createEntity`, `updateEntity`, `deleteEntity`,
`createEntityCore` and their helpers are deleted.
4. `workspace.autocascade_host.go` is deleted.
5. `internal/workspace/workspace.go` drops the runner field and
`newWorkspace`'s automation/cascade wiring.
6. All existing tests pass.
7. `just ci` green.
