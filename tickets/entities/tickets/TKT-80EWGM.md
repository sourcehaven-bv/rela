---
id: TKT-80EWGM
type: ticket
title: 'Unified targeted-write primitive: entitymanager.PatchEntity replaces four hand-rolled property merges'
kind: refactor
priority: medium
effort: l
status: done
---

Add a single targeted/partial entity-write primitive on `entitymanager.Manager`
and migrate the hand-rolled read-modify-write merges (Lua, MCP, CLI) onto it.
One merge implementation, one clear/unset semantic, one body tri-state —
reducing implementation and maintenance burden for both existing writers and new
ones (CalDAV/VTODO inbound sync being the next). Deletes
`lua.ReadDeps.WritePrepStore`.

## Problem

There is **no shared partial-write helper anywhere in the codebase**.
`internal/entity` exposes only `GetString` / `SetString` / `Clone` — no merge,
no unset. Every write path hand-rolls its own read-modify-write, and the
resulting dialects are mutually incompatible:

| Path | file:line | Clear a property? | Clear content? |
|---|---|---|---|
| dataentry PATCH | `write_handler.go:406-428` | yes — out-of-band `properties_unset []string` | yes — `*string` tri-state |
| MCP `update_entity` | `tools_entity.go:218-226` | yes — in-band `nil` sentinel | **no** |
| Lua `rela.update_entity` | `runtime.go:1743-1758` | **no** | yes (`""` clears) |
| CLI `update` | `update.go:38-62` | **no** (`-P key=` sets empty string) | **no** |

Four writers, four semantics, for one conceptual operation. The closest thing to
a shared concept is the `(setKeys map[string]any, unsetKeys []string)` pair in
`affordances.validateFieldWrite` — but that **validates**, it does not apply.

Each new writer must reinvent the merge and independently rediscover the
read-out / write-prep boundary (DEC-ZBI39P). The boundary is currently enforced
by convention and prose in four places (`lua/deps.go` field godoc, root
`CLAUDE.md`, `lua/runtime.go:1736-1739` inline, and
`TestScriptReads_UpdatePreservesHiddenProperties`). That volume of defensive
documentation is the smell: the codebase guards a sharp edge with warnings
rather than removing it.

**The key inversion this buys: forgetting a property yields a no-op, not an
erasure.**

## Findings from investigation (2026-08-09)

Recorded because they narrow the scope from the originating brief:

- **No live data loss exists today.** Every read-before-write already uses a raw,
ungated handle: MCP `deps.Store`, CLI `svc.Store`, Lua `WritePrepStore`,
dataentry `entityReader.getEntity` (godoc: *"deliberately UNGATED — ACL scoping
is applied elsewhere"*). This ticket is justified by **semantic divergence and
maintenance burden**, not by an imminent bug.
- **`visibleReader.getVisible` returns UNREDACTED entities** — it row-gates only
(`visiblereader.go:57-65`). Field redaction happens in `toV1` on the wire
struct, on a freshly-built map with no aliasing back to the stored entity
(`entityserializer.go:154`, `affordances.go:933`). So the dataentry PATCH path
is safe even against a hypothetical "tidy onto getVisible".
- **`automation` `set` actions are NOT a consumer.** `engine.go:250` only records intent
into `Result.PropertiesSet`; the entitymanager applies it to the **in-flight**
entity (`manager.go:590`). There is no read-modify-write there to migrate.
- **dataentry PATCH is already effectively this primitive**, just private and
HTTP-shaped. It is the reference semantics to generalize, not a site to fix.

## Scope

Build the primitive; migrate the three hand-rolled merges. Unification is the
goal — **not** retrofitting authorization.

- **CLI** has full access by design; the field-write gate must be a **no-op** there.
- **MCP** is client-side/local today (operator trust boundary). It will be reworked when
it moves to a remote MCP; migrating now means the gate is already a **wiring
choice** at that point rather than a rewrite.

### 1. The primitive

Shape is a proposal; naming and exact home are the implementer's call.

```go
// in or near internal/entitymanager
type PropertyPatch struct {
    Set   map[string]any  // properties to write
    Clear []string        // properties to explicitly remove
    Body  *string         // nil = leave body untouched
}

func (m *Manager) PatchEntity(ctx context.Context, id string, p PropertyPatch) (*entity.UpdateResult, error)
```

Requirements:

1. **Goes through `entitymanager.Manager`** — automations, validation, transitions,
unique checks, and audit must all fire. Not a store-level primitive.
2. **Merge happens against the raw entity, internally.** Callers never hold a raw store
handle to do it themselves — that is the entire point.
3. **A redacted-read caller is safe by construction.** A caller who can see 3 of 10
properties can patch those 3 without touching the other 7.
4. **Explicit clear is distinct from absent.** `Clear: ["due"]` removes it; omitting
`due` from both leaves it untouched.
5. **`Set` and `Clear` ordering follows dataentry's documented precedent**
(`write_handler.go:410-413`): unset applied **after** upserts, so a patch that
both sets and clears the same key ends cleared.

### 2. Field-write gate as an injected capability

Whether a caller may `Set`/`Clear` a property they cannot read is **already
answered in-tree**: `affordances.validateFieldWrite` (`affordances.go:319`)
enforces *"hidden/read-only fields cannot be set OR unset."* That answer stands
— do not re-litigate it.

The gap is that this validator lives in `internal/dataentry`; MCP and CLI never
call it. Supply it to `PatchEntity` as a **wiring-site capability**, mirroring
how read-side allow-all works (`AllowAllReader`, DEC-ZBI39P): never inferred
from identity, always injected. CLI and other operator-trust-boundary paths wire
a permissive/no-op gate; request-scoped paths wire the real resolver.

### 3. Migrations

- `internal/lua` `luaUpdateEntity` (`runtime.go:1724`) — canonical case. **Deletes
`lua.ReadDeps.WritePrepStore`**, its only consumer. Removes the footgun field
and the prose guarding it.
- `internal/mcp` `update_entity` (`tools_entity.go:190`) — in-band `nil` maps to `Clear`.
- `internal/cli` `update` (`update.go:31`) — gains clear-a-property and clear-content,
which it currently cannot do at all.

Callers keep their own wire shapes; only the merge is shared.

## Explicitly out of scope

- **Relations.** Entity properties only.
- **Optimistic concurrency / `If-Match`.** Belongs with the CalDAV work.
- **Any change to read-side ACL semantics.** This changes how writes *merge*, not who may
read what.
- **`cascadeHost.WriteEntity` bypassing the manager** — a real write-path hole, filed
separately. `PatchEntity` neither fixes nor worsens it.
- **`restoreOntoLive`** (`history_restore.go:102`) — a deliberate whole-entity replace;
correct semantics for a restore.
- **`syncHandler.putEntity`** — whole-record replace is the sync contract, guarded by a
mandatory If-Match precondition.
- **dataentry PATCH migration.** Optional follow-up; it is the reference semantics and
already correct. Migrating it is a simplification, not a fix.

## Acceptance criteria

1. `PatchEntity` exists on `Manager`; automations, validation, transitions, unique
checks, and audit all fire (asserted, not assumed).
2. A caller holding a **redacted** read patches one visible property; **every hidden
property is byte-identical afterwards.** This is the inverse test that would
have caught this class originally.
3. `Set` / `Clear` / absent are three distinct behaviours, table-driven over
visible × hidden.
4. Body tri-state: `nil` leaves untouched, `""` clears, non-empty replaces.
5. `lua.ReadDeps.WritePrepStore` is **deleted**; `luaUpdateEntity` compiles without a raw
store handle.
6. MCP and CLI merges are gone, replaced by `PatchEntity` calls; existing MCP nil-deletes
and CLI updates behave unchanged (regression-pinned).
7. CLI's field-write gate is a no-op — an operator can patch any property.
8. `just arch-lint`, `just lint`, `just test`, `just coverage-check` all pass.

## Testing

- Port `TestScriptReads_UpdatePreservesHiddenProperties`
(`internal/lua/aclreads_test.go:250`) to the new API; keep the original green
until its call site is migrated.
- Add the **inverse** test (AC2) — redacted-read caller patches one visible property,
assert all hidden properties survive byte-identical.
- Table-driven: property visible/hidden × set/clear/absent.
- Preserve the RR-J4518A end-to-end coverage: the hidden-property-preservation test must
still run **through the cascade path** (autocascade → LuaScriptRunner → update),
not only against a directly-constructed runtime.
- Replace the `WritePrepStore`-identity guard tests
(`internal/dataentry/luawiring_test.go:104`,
`internal/appbuild/luawiring_test.go:116`) with equivalents that pin the new
invariant — do not simply delete them.
- Include a fixture mirroring a real Apple Reminders VTODO PUT (partial property set:
`SUMMARY`, `STATUS`, `COMPLETED`, `PERCENT-COMPLETE`, `DUE`; entity has hidden
properties) — the exact CalDAV shape this unblocks.

## Open questions for planning

1. Does `PatchEntity` live on `Manager`, or as a narrow consumer-side interface each of
Lua/MCP/CLI/CalDAV declares? Root `CLAUDE.md` favours call-site interfaces; the
manager is where writes must land. Likely both: method on `Manager`, narrow
interface at each consumer.
2. Should `UpdateEntity` be reimplemented in terms of `PatchEntity`? **Suggest no** —
`UpdateEntity` has a documented in-place-mutation contract
(`manager.go:545-549`) that automation depends on, and `ApplyEntity` genuinely
needs whole-entity replace. Let them coexist.
3. Body handling — whole-body replace sufficient? **Suggest yes**, `*string` tri-state
matching dataentry's existing `req.Content`.

## Why now

CalDAV/VTODO inbound sync (Phase 2 of the calendar-feed arc, after TKT-RDM9M5)
adds a new writer. A CalDAV `PUT` from Apple Reminders carries a *partial* view
— verified empirically 2026-08-09: it sends `SUMMARY`, `STATUS`, `COMPLETED`,
`PERCENT-COMPLETE`, `DUE` and drops everything it does not model. Applying that
as a whole-entity save would erase every rela property VTODO has no slot for.

Building this inside the CalDAV PR would shape it around one caller. It has
standalone value and deserves its own design.
