---
id: PLAN-4S2AL4
type: planning-checklist
title: 'Planning: Piles: personal working sets of entities (service, API, scope source, UI)'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- `internal/piles`: the domain model, a `Service`, and a `Store` backend
contract. There are two backends, and both pass `pilestest.RunAll`:
  - `kvpiles`: a node-local file KV for fs, memory and sqlite (desktop
included).
  - `pgpiles`: tables in the tenant schema on postgres.
- Piles never enter `rela.db`. They are about the person, not the content, so
a shipped database must not carry them.
- The REST API under `/api/v1/_piles`.
- A `pile` scope source for prev/next on the entity page.
- Export of a pile through registered transforms.
- An optional `piles:` block in `data-entry.yaml`. It lists the actions and
transforms a pile offers.
- `principal.SystemUser` falls back to `%USERNAME%`, so Windows desktop has an
identity.
- **Explicit faces.** An item is an `entity.Ref{ID, Face}`, so a user can put
one specific face on a pile. Stepping goes to that face.
- **Lua bindings**, an **automation action** (`add_to_pile`) and **MCP tools**
for adding to piles. Scripts and automations may push into another user's pile
by name. Only the owner can read a pile.
- **Full piles drop the oldest item.** A pile is a stack: the newest item shows
first, and at 500 items an add pushes the oldest off the bottom. Adds never fail
on size.
- The SPA:
  - Add to pile from a list selection (bulk bar), from search results ("Add
results to pile"), and from the entity page.
  - A dialog to create a pile with a name and an icon.
  - A Piles sidebar group with counts.
  - A pile panel in the sidebar flyout: rows grouped by type, tick to remove
with Undo, a ⋯ menu (actions, export, rename, delete, copy ids), and Step
through.
  - Stepping through a pile on the entity page.

Out of scope:

- Sharing piles.
- "Turn into entity".
- Batch actions that receive all items in one script call.
- MCP tools and CLI commands for piles.
- Multi-select on kanban and search rows.
- Manual reordering of items.
- Restoring the original position on Undo (re-added items become the newest).
- Reading another user's pile from any surface.

**Acceptance Criteria:**

1. A user selects 3 rows in a list, picks "Add to pile" → "New pile from
selection…", and enters a name and an icon. The pile appears in the sidebar with
count 3.
2. Adding rows that are already on the pile leaves the count unchanged. The API
reports how many were added.
3. Concurrent add requests to one pile lose no items, and concurrent creates
for one owner respect the pile cap. The conformance suite tests this on every
backend.
4. Opening the pile shows its items grouped by type, newest first. Ticking 2 rows and pressing "Remove 2" removes them. Undo adds them
back at the end.
5. "Step through" opens the first item with `?from=pile&pile=<id>`. The entity
page shows "<pile name> · 1 / N". P and N move through the pile. N equals the
panel count in any world.
6. User B cannot read, change, export or step through user A's pile. Each
attempt gets the same 404 as a pile id that does not exist.
7. An item that becomes unreadable to the owner disappears from the panel, the
count, the scope and the export. It is not deleted: when access returns, the
item is back. Removing a hidden, deleted or never-added id gives identical
responses.
8. Renaming an entity keeps it on the pile under the new id. Deleting an entity
removes it from every pile.
9. A pile action listed in `piles.actions` runs once per item (or per ticked
item) through the existing action and PATCH endpoints. A toast reports the
success and failure counts.
10. Exporting a pile with a transform from `piles.export` returns exactly the
panel's items, with redacted fields hidden, in pile order.
11. A `piles:` block naming an unknown action or transform fails config
validation. So does any unknown key in the block.
12. A 51st pile is refused with 409. Adding to a pile at 500 items drops the
oldest items so the pile holds the 500 newest; the response reports `added`,
never what was dropped.
13. Adding `POL-1@draft` puts that face on the pile, and stepping opens
`/entity/policy/POL-1@draft`. Adding a bare id of a faced type with more than
one face is refused as ambiguous, naming the faces. Deleting the `draft` face
removes only that item.
14. An automation `add_to_pile: {pile: Inbox, owner: "{{new.assignee}}"}`
puts the triggering entity (its face included) on the assignee's Inbox pile,
creating the pile if needed. A failed push is logged and never fails the entity
write.
15. Lua `rela.piles.add{...}` and the MCP `add_to_pile` tool add to the
acting user's pile, or to another user's when `owner` is given. Lua
`rela.piles.list()` / `items()` and MCP `list_piles` / `show_pile` return only
the acting user's piles, with only readable items.
16. A push names an owner that must be the acting user or an existing entity
of the ACL's `user_entity_type`. Anything else is refused.
17. When there is no identity (an `unknown` or `system:*` user), the SPA shows
no piles UI, and the API refuses with 403.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design was settled with the user in conversation and is recorded on TKT-K3RJLH; the codebase survey is below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- **Libraries.** None apply. This is a small set store with an ACL-gated read
path built from rela's own pieces.
- **`internal/comments`** is the pattern for the backend contract, the
conformance suite and the rename and delete hook:
  - Comments follow renames and deletes through `entitymanager.AliasRewriter`,
wired by `newAliasFanout` (`internal/appbuild/appbuild.go:2291`).
  - Observers drop errors, so piles use the same hook instead.
- **`internal/userstate`** is per-user state outside the graph. The piles
backend is chosen the way userstate's is: `storeUserStateFor(st)` is a
build-tagged helper that derives the backend from the store
(`appbuild.go:277-296`).
  - Every assembly path agrees, including `SharedBase.Assemble`, which passes
empty `backendOverrides` (`appbuild.go:1949`).
  - It falls back to memory when there is no KV.
  - `kvuserstate` keeps one document under a single key.
- **Scope sources:**
  - `internal/dataentry/scope.go`: `ScopeDescriptor`, `knownScopeSources`,
`resolveScope`, `handleV1EntityPosition`, and `storePosition` (`:269`).
  - On the SPA side: `useScopeNavigation.ts`.
- **Batch visibility reads.** `visibility.Resolver.ResolveIDsErr`
(`visibility/batch.go:98`) reads ids in a world with one header query plus one
gate call per type. A hidden id is absent from the result. Store errors are
returned, not logged away.
- **World gating.** `worldCapablePath` (`internal/dataentry/world.go:349-420`)
and its guard test.
- **Export.** `listTableRenderer` (`export_list.go:303`), `resolveTransform`
and `convertAndWrite` (`export.go`).
- **Actions.** The SPA fans out per row in `useListActions.ts`, calling
`POST /_action/{id}` or the entity PATCH. There is no server-side bulk path, and
piles add none.
- **The fs import.** `classifyRela` (`internal/fsimport/state.go`) decides
which `.rela` keys are copied.
- **SPA building blocks:**
  - `useListSelection` and `RlBulkActionBar` (`EntityList.vue`).
  - `useFlyout` and `SidebarFlyout.vue`.
  - `useNavStatus` for the query pattern.
  - `ExportMenu.vue`.
  - `uiStore.showToast` with Undo (`useBulkDelete.ts`).
  - `RlModal` (`DuplicateModal.vue`).
- **Other projects.** Tornado Notes (DOS) piles are the inspiration: a search
yields a pile you flip through. The feature is also close to browser reading
lists and Things/OmniFocus "perspectives". The mockup is
`frontend/packages/rela-components/src/mockups/PileMockup.vue`.
- **rela concepts.** Browse scope is the direct predecessor. A pile is a third
scope source.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Domain (`internal/piles`):*

- `Pile{ID, Owner, Name, Icon, Created, Updated}` and
`Item{Ref entity.Ref, Added time.Time}`.
- **An item is an `entity.Ref{ID, Face}`.** The face is explicit, so the user
chooses it. Input is always an address (`ID` or `ID@face`):
  - A named face is taken literally.
  - A bare id resolves through the write-target rule
(`visibility.Resolver.WriteTarget`): the one readable face the world admits,
otherwise `*visibility.AmbiguousAddressError` naming the faces. Adding is a
write, so a face is never picked by rank.
  - Refs are built only by the resolver, so the bare-ref allowlist needs no
new entry.
- **Names are unique per owner**, compared case-insensitively. Lua, automation
and MCP address a pile by name.
- **Pile ids** are server-minted (`PIL-` plus an alphanumeric short id) and
format-checked on input.
- **Owner** is `principal.User`. With person mapping configured
(`user_entity_type` / `principal_property`) that is the person entity id, which
is what lets an automation write `owner: "{{new.assignee}}"`. Without mapping
(desktop, plain server) it is the login name.
  - The person type must be faceless (`acl/facedgrants.go:176`), so an owner is
always a bare id.
  - When a person entity is renamed, the rename hook rewrites the owner as well
as items. When it is deleted, that person's piles are dropped, because ids can
be reused.
  - `Service` refuses the owner `unknown`, the empty owner, and any `system:*`
owner with `ErrNoOwner`, as `comments.authorFrom` does.
  - The rule lives in one function, `piles.OwnerFrom(ctx)`, which the HTTP layer
also uses to compute `piles_available`.
- **Validation:**
  - The name is 1–80 runes after trimming, with no control characters.
  - The icon comes from a fixed allowlist of icon names.
- **`Store`** is the backend contract. Its methods:
  - `ListPiles(owner)`, `GetPile(owner, id)`.
  - `CreatePile(p, ids)`, `UpdatePile(owner, id, name, icon)`, `DeletePile(owner, id)`.
  - `Items(owner, id)`.
  - `PileByName(owner, name)`.
  - `AddItems(owner, id, refs, max) (added int)`: inserts as newest, then
evicts the oldest past `max`, atomically. `RemoveItems(owner, id, refs)`.
  - `RenameEntity(old, new)` (items and owners), `DeleteEntity(id)` (items,
and the piles OWNED by id), `DeleteFace(id, face)`.
- Every per-pile method filters on the owner inside the backend query or lock.
A foreign pile behaves exactly like `ErrNotFound`.
- Adds and removes are idempotent set operations. When a rename lands on an id
already on the pile, the two collapse into one item.
- **Limits:**
  - 50 piles per owner (`ErrLimit`), checked atomically with the create.
  - 500 stored items per pile. An add that would exceed it evicts the oldest
items in the same transaction or lock. Eviction counts stored items, hidden ones
included. That is the documented remaining channel: a user may notice that an
old visible item survived longer than expected. It never reveals which items, or
how many.
- **`Service`** wraps `Store`. It validates input and resolves the owner. Its
constructor rejects a nil store.
- It implements `entitymanager.AliasRewriter`:
  - `EntityRenamed` rewrites the id; `EntityDeleted` drops it.
  - `EntityFaceDeleted` drops exactly that ref.
  - These methods are owner-free and are not reachable from HTTP. Like comments,
they run outside any store `Tx`, so they cannot roll back with it. Documented in
the godoc.

*Backends:*

- **`kvpiles`** is used for fs, memory and sqlite (desktop included).
  - It keeps one JSON document under the key `piles.json`, holding every
owner's piles. Owner strings live inside the value, never in a key.
  - A mutex inside the store serializes read-modify-write.
  - It runs over a **node-local file KV**: `buildStateKV(fs, paths)`, the
`.rela` cache dir. On sqlite that is NOT the database's `state_kv`.
  - With no cache dir it falls back to an in-memory KV and logs a startup
warning, as `newUserState` does. Never `nopKV`, which drops writes.
  - The fs tier is single-process by design, so the in-process mutex is
enough. `SharedBase.ForReassembly` carries the predecessor's piles service, like
`attachLocker`, so a reassembled `Services` shares the same instance and mutex.
- **`pgpiles`** (postgres) adds migration `0020_piles.sql` (number re-checked
at merge) with two tables in the tenant schema:
  - `piles(id PK, owner, name, icon, created_at, updated_at)`, indexed on
`owner`.
  - `pile_items(pile_id FK ON DELETE CASCADE, entity_id, face, seq, added_at,
PK(pile_id, entity_id, face))`, indexed on `entity_id`.
  - A unique index on `(owner, lower(name))`.
  - Create and add run in one transaction that first takes
`pg_advisory_xact_lock(hashtext('piles:'||owner))`. The pile cap and the
eviction (`DELETE … WHERE seq <= (the 500th newest seq)`) then see a serialized
view.
  - Adds use `INSERT … ON CONFLICT DO NOTHING`. `seq` draws from a dedicated
sequence; `rela_seq` is never used for this.
  - Rename uses `UPDATE … WHERE entity_id = old`, guarded against a primary-key
clash by deleting the duplicate first, in one transaction.
  - The package declares its own narrow `DBTX` and takes the shared pool.
- **Selection.** A build-tagged `storePilesFor(st)` returns `pgpiles` on a
pgstore and nil elsewhere. `newPiles(st, localKV)` falls back to `kvpiles`.
`assemble` calls it, so `New`, `Discover`, `SharedBase.Assemble` and reassembly
all agree.
- **fs import.** `classifyRela` treats `piles.json` as personal state and does
not copy it, with a report reason. This is covered by a test in
`internal/fsimport`.

*Shared read path (`internal/dataentry/piles_read.go`):*

- `readableItems(ctx, pile) ([]store.EntityHeader, error)` makes one batched
ref resolution over the pile's refs, newest first. A named face is served
literally, so it is not world-ranked. It returns the store error rather than an
empty slice.
  - Today `ResolveHeaders` logs and swallows store errors, so it gets an
error-returning twin, `ResolveHeadersErr`, as `ResolveIDs` has.
  - Under the read gate a face the owner cannot read is absent.
- The panel, the counts, the scope and the export all use it, so they agree by
construction.
- `GET /_piles` makes one `ResolveIDsErr` over the union of all the owner's
items, then counts per pile.

*API (`internal/dataentry/piles_handler.go`, a `pilesHandler` struct like
`commentsHandler`; route `/api/v1/_piles`):*

- `GET /_piles` lists the user's piles with readable counts.
- `POST /_piles` takes `{name, icon, items?: [address]}` and returns 201 with
the pile. A duplicate name gets 409 `pile_name_taken`.
- `GET /_piles/{id}` returns the pile and its readable items, each as
`{id, face, address, type, title}`, newest first.
- `PATCH /_piles/{id}` takes `{name?, icon?}`.
- `DELETE /_piles/{id}` returns 204.
- `POST /_piles/{id}/items` takes `{items: [address]}` and returns `{added}`.
  - Named faces are checked in one batch. Bare ids go through `WriteTarget`,
one header read each, bounded by the 500-id request cap. Bare ids are rare from
the SPA, which sends each row's `_self`.
  - An address that is not served fails the whole request with 404
`item_not_found`, which never names it. An ambiguous bare id gets 409
`ambiguous_address` naming the faces, like the existing write path.
  - The oldest items past 500 are evicted.
- `POST /_piles/{id}/items/_remove` takes `{items: [address]}` and **always
returns 204 with no body**. It does not depend on what was stored, so it cannot
be used to probe for existence. Addresses are parsed, not resolved: a bare id
removes the unfaced ref.
- `GET /_piles/{id}/_export?transform=` is described under Export.
- `piles_available: bool` is added to the existing SPA bootstrap/config
payload. It is true when the service is wired and `OwnerFrom(ctx)` succeeds.
- Error mapping:

  | Error | Response |
  | --- | --- |
  | missing or foreign pile | 404 `pile_not_found` |
  | validation error | 400 |
  | `ErrLimit`, duplicate name, ambiguous address | 409 |
  | `ErrNoOwner` | 403 |
  | store error | 500 with a correlation id |

- Bodies are size-capped (413). Requests are capped at 500 ids (400).
- **World.** `worldCapablePath` admits `_piles` and `_piles/…`, with a
call-site justification: every read goes through `readableItems` in the
request's world. `viewworld_guard_test` is extended.
- Pile writes are not entity writes. They produce no audit record, no
versioning and no automations, and the read-only instance flag does not apply to
them.
- Wire types live in `internal/apiwire/v1`.

*Scope:*

- `ScopeDescriptor` gains `Pile string`, and `knownScopeSources` gains `pile`.
- `source: pile` requires `pile` and refuses every other field.
- `resolveScope` gets a `case "pile"` that calls `readableItems` and converts
the headers.
- `_position` matches the current entity by **Ref** for a pile scope. The `id`
param may be `ID@face`, parsed with `entity.ParseStateRef`. List and search
scopes keep matching by id.
- `v1.PositionRef` gains `address` (`ID` or `ID@face`).
- For a pile scope the SPA passes `servedRef` rather than `bareEntityId`, and
prev/next links use `address` (`/entity/<type>/<ID@face>`). The route already
accepts that form.
- `storePosition` changes to run only when `Source == "list"`.

*Export (`internal/dataentry/export_pile.go`):*

- It resolves the transform with `resolveTransform`. If `piles.export` is set,
the transform must be in it, otherwise 404 `unknown_transform`.
- It takes the items from `readableItems`, converts the headers to body-less
entities (`headerProbe`), and renders them through `listTableRenderer` with the
columns id, type and title.
- Redaction happens once, on the resolve path. `PolicyReader.Filter` is not
used.
- It reuses `convertAndWrite` and the hardened download headers. Export is not
world-scoped today; this one is, through `readableItems`.

*Config (`internal/dataentryconfig/piles.go`):*

- `Config.Piles *PilesConfig{Actions []string, Export []string}`.
- `piles` is added to `validTopLevelKeys`.
- `validatePiles` checks that:
  - every action exists, is not entity-bound, and has a label;
  - every export name exists in the transforms registry;
  - the block has no unknown keys.
- A `set:` action whose properties exist on no type gets a warning.

*Push to another user (shared by Lua, automation and MCP):*

- `piles.Service.Push(ctx, PushRequest{Owner, Pile, Refs, Create})`.
  - The pile is addressed by name and created when `Create` is set.
  - Refs are resolved by the caller, so the service never reads the graph.
- **Owner check.** An empty owner means the acting user (`OwnerFrom(ctx)`).
Any other owner must be the acting user, or an existing entity id of the ACL's
`user_entity_type`. The caller supplies that check as a narrow `OwnerExists(ctx,
id) bool` (a raw header existence read, consumer-side interface). This bounds
owners to real people, so a typo cannot grow the store with ghost owners.
  - Without person mapping, only the acting user is valid.
- **Readability.** A push checks that the ACTING principal can read each ref,
through the same resolution as the API. The target owner's access is re-checked
at read time like every item, so a push can never reveal an entity the owner
cannot read.
- Pushes never read the target's piles and return only `{added}`. A push is
"write-only" to someone else.

*Lua (`internal/lua`):*

- In `WriteDeps`, `Piles PilePusher` (consumer-side: `Push`).
- In `ReadDeps`, `Piles PileReader` (`List`, `Items` for the acting owner
only). Items are resolved through the script's `VisibleReader`, so they are
readable-only.
- Bindings:
  - `rela.piles.add{pile=, entities={addresses|entity tables}, owner=?, create=true}`
returns `added`.
  - `rela.piles.remove{pile=, entities=}` works on own piles only.
  - `rela.piles.list()` and `rela.piles.items(name)` work on own piles only.
- An address resolves through the existing `resolveWriteTarget`, so a bare id
on a multi-face type raises the ambiguity error.
- With no piles service wired (pure CLI runs), the `rela.piles` table is
absent and calling it raises a clear error.
- Scheduled scripts run as `system:*`, so they must pass `owner`.
- Documented in `docs/lua-api.md` (or wherever the bindings are listed).

*Automation (`add_to_pile`):*

- `metamodel.AutomationAction.AddToPile *AddToPileAction{Pile, Owner, Create}`
(`yaml:"add_to_pile"`). Pile and owner support interpolation; `create` defaults
to true.
- Schema load validates it: the pile name is non-empty after interpolation
shape-checking.
- `automation.Engine.executeAction` interpolates the fields and appends to
`Result.PilesToPush` (nothing is written while planning).
- `autocascade.Runner` executes it with the trigger's Ref (`trigger.Ref()`,
face included) through a consumer-side `PilePusher` in `autocascade.Deps`. It is
optional: nil means the action logs a warning once and skips.
- A failed push (unknown owner, pile cap, store error) is logged with the
automation name and never fails the entity write. A pile is a notification
surface, not a system of record.
- On postgres the push runs on the pool, outside any store `Tx`. If the entity
write then rolls back, the pushed ref points at a row that never committed. The
read path drops it, so it is harmless, and it is documented.

*MCP (`internal/mcp`):*

- `list_piles`, `show_pile {pile, world?}`, `add_to_pile {pile, ids,
owner?, create?}`, `remove_from_pile {pile, ids}`.
- The principal comes from the MCP server: stdio uses `SystemUser`, remote uses
the JWT principal.
- Reads use the MCP server's visibility-wrapped reader. ids accept
`ID@face`, and a bare id goes through `WriteTarget`, as update does.
- The tools are registered only when the piles service is wired
(`mcp.Services` gains a narrow `Piles()`).

*Wiring:*

- `Services.Piles()`.
- `dataentrywire` calls `app.SetPiles(svc.Piles())`, which rejects nil. It is
the only new `App` method, checked with `just plimsoll`.
- The service is added to `newAliasFanout`, `lua.WriteDeps`/`ReadDeps`,
`autocascade.Deps` and the MCP server's services.
- Arch-lint gets these components:

  | Component | Deps |
  | --- | --- |
  | `piles` | entity, principal, errors |
  | `kvpiles` | piles, state |
  | `pgpiles` | piles; `canUse: pgx` |

`pilestest` is excluded. `appbuild` and `dataentry` are granted access.

*SPA:*

- `frontend/src/api/piles.ts`.
- `composables/usePiles.ts`, a Pinia Colada query `['piles', world]`. It
refetches after its own mutations and on a debounced `entity:changed`, NOT on
every route change. It is enabled only when `piles_available` is true.
- `components/piles/NewPileDialog.vue`, `AddToPileMenu.vue`, `PilePanel.vue`.
- `EntityList.vue`: rows are selectable when `piles_available`, and
`AddToPileMenu` sits in the bulk bar.
- `SearchView.vue`: an "Add results to pile" button.
- `EntityDetail.vue`: "Add to pile" in its menu, and "Remove from pile" in pile
scope.
- `Sidebar.vue`: a Piles group with `opensFlyout` items and "New pile".
- `useFlyout.ts` and `SidebarFlyout.vue` gain a `pile` panel.
- `useScopeNavigation.ts` handles `from=pile` → `{source:'pile', pile}`, with
the pile's name as the label.
- `useListActions.ts` is generalized to take (address, type) pairs.
- Remove with Undo: the toast's Undo re-adds the removed addresses, which
become the newest.
- Add actions send each row's `_self` address, so the shown face is the one
added. The entity page adds `servedRef`.

**Files to modify:**

- New:
  - `internal/piles/{piles.go,service.go,owner.go,errors.go}`
  - `internal/piles/pilestest/pilestest.go`
  - `internal/piles/kvpiles/`, `internal/piles/pgpiles/`
  - `internal/store/pgstore/migrations/0020_piles.sql`
  - `internal/appbuild/{piles.go,piles_postgres.go,piles_nostore.go}`
  - `internal/dataentry/{piles_handler.go,piles_wiring.go,piles_read.go,export_pile.go}` and tests
  - `internal/dataentryconfig/piles.go`
  - `frontend/src/api/piles.ts`, `frontend/src/composables/usePiles.ts`
  - `frontend/src/components/piles/*`
- Changed:
  - `internal/apiwire/v1` (pile types, `piles_available`)
  - `internal/principal/principal.go` (`SystemUser` falls back to `USERNAME`)
  - `internal/appbuild/appbuild.go` (assemble, `Services`, alias fanout,
`ForReassembly`)
  - `internal/dataentrywire/wire.go`
  - `internal/dataentry/{scope.go,world.go,api_v1.go,app.go}`
  - `internal/dataentryconfig/{config.go,validate.go}`
  - `internal/visibility/batch.go` (`ResolveHeadersErr`)
  - `internal/metamodel/types.go`, `internal/automation/{types.go,engine.go}`,
`internal/autocascade/runner.go`
  - `internal/lua/{deps.go,runtime.go}` and a new `internal/lua/piles.go`
  - `internal/mcp/{tools.go,server.go}` and a new `internal/mcp/tools_piles.go`
  - `internal/cli/mcp_wiring*.go`
  - `internal/fsimport/state.go`
  - `.go-arch-lint.yml`, `.testcoverage.yml`
  - `frontend/src/{components/lists/EntityList.vue,views/SearchView.vue,components/entity/EntityDetail.vue,components/common/Sidebar.vue,composables/useFlyout.ts,components/flyout/SidebarFlyout.vue,composables/useScopeNavigation.ts,composables/useListActions.ts,api/entities.ts}`
  - `docs/data-entry.md`, `docs/postgres-backend.md`, `docs/metamodel.md`
(`add_to_pile`), the Lua API doc, and the MCP tools doc

**Alternatives considered:**

- **One `state.KV` blob on every backend.** Rejected. KV has no
compare-and-swap, so postgres nodes would lose updates (see TKT-DK0X6O).
`pgpiles` is used on postgres; the blob is acceptable only on the single-process
tiers.
- **A `sqlitepiles` table in `rela.db`.** Rejected. Personal sets would ship
inside a shared database file, keyed by an OS username that a recipient may
share.
- **Piles as graph entities.** Rejected. That would bring audit, versioning,
automations and ACL to a personal working set. Sharing stays in the graph.
- **Ids only, with the face picked per world.** Rejected: a user wants a
specific face on a pile. The entity route already accepts `ID@face`, so the cost
is matching position by Ref.
- **Owner = login name (`RawUser`).** Rejected: automations know people as
person entity ids (`{{new.assignee}}`), not login names.
- **Refusing adds on a full pile.** Rejected: automations and scripts cannot
handle a 409 usefully, and a stack naturally drops its oldest items.
- **Push reading the target's pile.** Rejected: a push is write-only, and only
the owner can read.
- **A server-side bulk action endpoint.** Not needed. The SPA fan-out already
goes through entitymanager per item.
- **A store observer for renames.** Rejected. It drops errors.
- **Counts before filtering, a stored-count cap, or a `_remove` that echoes
what it removed.** Rejected. Each is an existence oracle.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Pile name.** Trimmed, 1–80 runes, no control characters; otherwise 400.
Rendered as text only.
- **Icon.** Must be in the allowlist; otherwise 400.
- **Pile id in the URL.** Must match `^PIL-[A-Z0-9]{4,16}$`; otherwise 404.
- **Item ids.** Batch-resolved in the request's world. Any id that is not
served fails with a uniform 404 `item_not_found`. Requests are capped at 500
ids, and the body is size-capped.
- **Scope descriptor.** `source: pile` with any extra field gets 400. A foreign
or missing pile gets 404.
- **`piles:` config.** Validated at load.
- **Owner.** Comes from the principal on ctx (`OwnerFrom`) for every read and
every HTTP write. The HTTP API has no `owner` field at all. Only Lua, automation
and MCP pushes take an explicit owner, validated against `user_entity_type`.

**Security-Sensitive Operations:**

- **Owner isolation.** Enforced in every backend query or lock and tested in
`pilestest`. A foreign pile gets the same 404 as a missing one.
- **Read-out.** Every path (counts, items, scope, export) goes through
`readableItems` in the request's world. Counts are taken after filtering.
- **Push.** Operator-authored scripts and automations, plus MCP callers, can
ADD to another user's pile, never read it. The acting principal must be able to
read each pushed ref, and the owner's read gate is re-checked at display. A
malicious MCP caller could fill a colleague's pile with up to 500 items they can
both read (nuisance only). This is accepted and documented.
- **Existence oracles.**
  - Eviction is the only size behavior; there is no refusal.
  - `_remove` always returns 204 with no body.
  - The `item_not_found` 404 never names an id.
  - The remaining channel is eviction counting hidden items, which is
documented.
- **Export.** Only a registered and allowed transform name is accepted.
Redaction happens once. The download uses the hardened headers.
- **Actions.** No new execution path; each item goes through the existing
endpoints and their checks.
- **Privacy.** Pile names are private user data. They are not logged, not
shared, and not shipped in `rela.db`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1, 2, 4 | `pilestest`: create with items, duplicate add (added=0), remove, order. Handler test: create → list count 3. |
| 3 | `pilestest`: 20 goroutines adding distinct ids lose none; concurrent creates stop at exactly 50. Runs on kv and on pg (gated, with a schema-pinned DSN). |
| 5 | Scope tests: pile `_position` returns current/total/prev/next in pile order; extra fields give 400. Test that the panel count equals the `_position` total in a non-default world. Vitest for `useScopeNavigation` with `from=pile`. |
| 6 | Handler tests: user B on GET, PATCH, DELETE, items, `_remove`, `_export` and scope of A's pile gets the same 404 body as an unknown id. `pilestest` owner isolation. |
| 7 | ACL fixture: hide an item → absent from GET, counts, position and export; restore access → back. `_remove` of a hidden, a deleted and a never-added id gives byte-identical responses. |
| 8 | `pilestest`: rename (including onto an id already present) and delete. An appbuild test: rename through entitymanager → the pile holds the new id. |
| 9 | Vitest for `useListActions` over mixed types. Manual: run a `set:` action from a pile. |
| 10 | Export test: a hidden item is excluded, a redacted field is not rendered, a transform outside `piles.export` gives 404, and the rows equal `GET /_piles/{id}`. |
| 11 | `dataentryconfig` table tests: unknown action, entity-bound action, unknown transform, unknown key. |
| 12 | `pilestest`: the 51st pile; adding to a full pile evicts exactly the oldest, also under concurrency. |
| 14 | `pilestest`: two faces of one id are two items; `DeleteFace` drops one. Handler: an ambiguous bare id gets 409 naming the faces; `_position` with `ID@face` in pile scope steps faces correctly. Vitest: pile scope links use `address`. |
| 15 | automation engine test: plan contains the push with interpolated owner/pile; autocascade test with a fake pusher: called with the trigger Ref; a pusher error does not fail the write. appbuild integration: an assignee update lands on the assignee's Inbox. |
| 16 | Lua runtime tests for add/remove/list/items, including owner push and a bare-id ambiguity error; MCP tool tests for all four tools, including a hidden item absent from `show_pile`. |
| 17 | `pilestest`/service: an unknown owner is refused; without person mapping only the acting user is valid. |
| 13 | Handler test: an `unknown` principal gets 403, and the bootstrap reports `piles_available:false`. Vitest: no piles UI when it is false. Unit test for the `SystemUser` `USERNAME` fallback. |

**Integration:**

- `storetest.Counting` budget tests check that the query count is the same at
10 and 50 items for: `GET /_piles`, `GET /_piles/{id}`, `POST
/_piles/{id}/items`, `POST /_piles`, pile `_position`, and pile export.
- `sharedbase_test`: a pg-assembled `Services` gets `pgpiles`; a reassembled
`Services` shares the piles instance.
- `fsimport` test: `piles.json` is not copied and is reported.
- e2e (Playwright, server with `RELA_DATAENTRY_USER` set): select rows → new
pile → sidebar count → open panel → step through → remove with Undo.

**Edge Cases:**

- A rename where both `X` and `X@draft` are on the pile keeps both, with the
new id.
- A duplicate name in a different case gets 409. A push to an existing name
reuses that pile.
- A push by a principal who cannot read the ref: `item_not_found` (Lua and MCP
error, automation logs).
- A person-entity rename moves their piles; deleting the person drops them.

- An empty pile: empty state, stepping disabled.
- A pile whose items are all hidden: count 0 and no scope nav.
- A faced entity is one item; the world picks the face.
- A rename onto an id already on the pile collapses the two into one item.
- A Unicode name passes. A whitespace-only name or an 81-rune name gets 400.
- Deleting a pile while its panel is open closes the panel; the scope then
returns 404 and the nav hides.
- A store fault gives 500, never an empty pile.
- No cache dir: the in-memory fallback, with a warning.
- A Windows desktop with only `%USERNAME%` set.

**Negative Tests:**

- Malformed JSON gives 400. An oversized body gives 413. More than 500 ids
gives 400.
- An unknown icon gives 400. A malformed pile id gives 404.
- An unknown item id gives 404 `item_not_found`, and nothing is added.
- `unknown` and `system:*` principals get 403.
- An unregistered or disallowed transform gives 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Size (xl).** Mitigation: one branch, built and committed in four stages:
  1. service, backends and wiring;
  2. API, scope, export and config;
  3. Lua, automation and MCP;
  4. SPA.
- **Pushes inside a write path on postgres** run outside the Tx.
Mitigation: the read path drops refs that never committed, and pushes are
best-effort and logged.
- **Always-selectable rows change every list.** Mitigation: rows are selectable
only when `piles_available` is true. Check the e2e tests that count checkboxes.
- **Counting cost on the sidebar.** Mitigation: the caps bound it to one
header batch, budget tests pin it, and no refetch on route change.
- **The kvpiles document grows with users × piles.** Mitigation: the caps bound
it (at most 50 × 2000 ids per user). Acceptable on single-user and small-team fs
and sqlite tiers.
- **Mixed-type `set:` actions fail on types without the property.** Mitigation:
the toast reports failures, and config validation warns.

**Effort:** xl. The Lua, automation and MCP surfaces add about a third.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md`: a Piles section covering usage, the `piles:`
config, limits, privacy, the 2000 ceiling channel, and that Undo appends.
- [x] `docs/postgres-backend.md`: the new tables are in the tenant schema.
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting rule; piles follow the existing
backend and visibility patterns)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 20 findings from a go-architect design review (3
critical, 7 significant, 7 minor, 3 nits), each recorded as a review-response
linked to TKT-K3RJLH.

- 19 are addressed in this plan.
- 1 minor (Undo order) is wont-fix, with the reason recorded.

The main changes:

- `_remove` returns 204 with no body.
- The cap applies to the readable count.
- kvpiles is one document over a node-local KV, never `nopKV`, and never
`rela.db`.
- The owner is `RawUser`.
- `piles_available`.
- `_piles` is world-capable.
- An item is an id.
- Adds are batch-validated.
- One `readableItems` path serves the panel, counts, scope and export.
- The backend is derived from the store.
