---
id: TKT-72SCPR
type: ticket
title: Render list and global/dashboard commands in the data-entry SPA (view context deferred)
kind: enhancement
priority: medium
effort: m
status: planning
---

## Description

The `commands:` system in `data-entry.yaml` supports four contexts. The backend
implements all four end to end. The frontend renders exactly one.

A user who configures `context: global` or `context: list` gets a command that
validates cleanly at config load, is returned correctly by `GET
/api/v1/_commands`, and executes correctly via `POST /api/command/{id}` — but
has no button anywhere in the UI. The failure is silent: no error, no warning,
just a missing affordance.

**Scope: `list` and `global`/dashboard. `view` is deferred** — see below.

## Evidence

**Backend — all four contexts implemented:**

- `internal/dataentryconfig/validate.go:87` — `validCommandContexts` = `entity`, `list`, `view`, `global`
- `internal/dataentryconfig/config.go:563` — `CommandConfig.Context`, `CommandScope` with `Views` / `Lists` / `EntityTypes` / `Dashboard`
- `internal/dataentry/commands.go:112` — `contextMatchesPage` maps `global` → pageType `dashboard`
- `internal/dataentry/commands.go:77` — `matchesPage` handles `case "dashboard": if scope.Dashboard`
- `internal/dataentry/commands.go:281` — `handleCommandExec` dispatches per context, each with its own stdin builder (`buildEntityInput`, `buildListInput`, `buildViewInput`, `buildGlobalInput`, `commands.go:150`)
- `internal/dataentry/api_v1.go:3246` — `handleV1Commands` accepts `page_type`, `qualifier`, `entity_type`

**Frontend — one call site, hardcoded:**

`frontend/src/components/entity/EntityDetail.vue:316` is the only `getCommands`
call in the entire SPA:

```ts
commands.value = await getCommands(
  { pageType: 'entity', entityType: props.entityType },
  localAbort.signal
)
```

`pageType` is a literal; `qualifier` is never passed.

`frontend/src/views/DashboardView.vue` imports no command code at all — its
entire import set is `vue`, `@/stores`, `searchEntities`/`analyze`, and types.
The list surface likewise never imports commands.

## Gap map

| context | backend resolve | backend exec | Go test coverage | frontend render | this ticket |
|---|---|---|---|---|---|
| `entity` | yes | yes | yes | yes (EntityDetail) | migrate to composable |
| `list` | yes | yes | partial | no | **add** |
| `global`/dashboard | yes | yes | yes | no | **add** |
| `view` | yes | yes | partial | no | **deferred** |

This is a **UI gap, not a correctness gap**. The backend behavior is implemented
and tested; what is missing is the render surface.

## Deferred: view context

`view` is out of scope, aligned with the same deferral in TKT-MJ02AO.

Three reasons:

1. **No distinct UI surface exists.** There is no `/view/:id` route — views
render *inside* `EntityDetail` via `fetchView`. Deferring `view` removes zero
planned mount sites; dashboard and list are unaffected.
2. **It is not gated by TKT-MJ02AO.** That ticket defers `view` because a view
command's payload is the whole traversal closure (`executeView`,
`views.go:19-49`, multi-pass; `buildViewInput`, `commands.go:181-215`, returns
`Collections` + inter-relations) with no read-gate scoping. Adding a UI surface
for a context that is deliberately ungated would be backwards.
3. **It retires an unresolved decision.** RR-N643V4 established that the
planned `available_on.views` fix was a no-op, and that a real fix requires
sending `pageType: 'view'` — which changes which unscoped and `context: view`
commands resolve. Dropping view scope closes that question by construction
rather than leaving a judgment call in the plan.

**Consequence:** the `available_on.views` defect is **not fixed here** and
remains open. It is a genuine pre-existing bug — `matchesPage` `case "entity"`
(`commands.go:94-97`) reads only `scope.EntityTypes`, so `available_on.views`
scoping has never worked. It should be picked up alongside view-context support.

## Latent defect (retained, not fixed here)

`contextMatchesPage` deliberately lets `entity`-context commands surface on view
pages, and `EntityDetail` does call `fetchView` — but it asks with `pageType:
'entity'` and no `qualifier`, so `available_on.views` never matches. Documented
above; out of scope per the deferral.

## Design constraint: avoiding triplication (not the modal)

`CommandModal.vue` takes an `entity-id` prop (`EntityDetail.vue:1185`) and is
imported nowhere else. Planning established this coupling is **one line** —
`CommandModal.vue:42-45` is the only use of `props.entityId`:

```ts
const params = new URLSearchParams()
params.set('entity_id', props.entityId)
const url = `/api/command/${cmd.id}?${params.toString()}`
```

**The actual cost driver is duplication.** `loadCommands` + `runCommand` +
desktop button row + mobile overflow menu + outside-click watcher is ~50 lines
of script and ~35 of template. Copied to each surface, the BUG-6C3V abort
pattern (`EntityDetail.vue:312-331`) would exist in triplicate.

**Surface correction:** `ListView.vue` is an 11-line pass-through (`<EntityList
:list-id="id" />`). The list command row belongs in
`components/lists/EntityList.vue`, which has `props.listId` and an existing
`.list-header` at L653.

## Existing test coverage (do not duplicate)

Per `e2e/tests/AGENTS.md:45` ("API-only assertions belong in Go"),
`request.fetch(...)` and `api.rawRequest(...)` are `no-restricted-syntax` eslint
errors inside `e2e/tests/**/*.spec.ts`.

`internal/dataentry/commands_test.go` (1041 lines) covers
`TestResolveCommands:21`, `TestMatchesPage:836`, `TestBuildListInput:288`,
`TestBuildGlobalInput:342`, `TestHandleCommandExecGlobalContext:683`,
`TestHandleCommandExecListContext:713`, plus `TestV1CommandsEndpoint`
(`api_v1_test.go:2716`).

**Known thinness to close here:** `TestHandleCommandExecListContext:713` asserts
only `w.Code == 200` and never inspects the SSE payload, unlike the entity and
global variants. Strengthen it. (`TestHandleCommandExecViewContext:733` has the
same weakness but is out of scope with `view`.)

## Scope

**In scope:**

- Generalize `CommandModal.vue` to a **closed discriminated union** prop (RR-KNEF4K), not an open map; update `EntityDetail` to pass `{ entity_id: entityId }`
- Extract `composables/useCommands.ts` (abortable list fetch, BUG-6C3V guards verbatim) and `components/commands/CommandButtons.vue` (desktop row, overflow menu, outside-click watcher, modal ref)
- Render commands on `DashboardView.vue` (`pageType: 'dashboard'`, no qualifier)
- Render commands on `components/lists/EntityList.vue` (`pageType: 'list'`, `qualifier` = `props.listId`)
- Fix the command-stream cancellation leak: `AbortController` around the exec stream, aborted in `onBeforeUnmount`; allow Close while running
- Preserve existing behavior: desktop button row + mobile overflow menu, abort-on-navigate, SSE streaming
- Frontend unit tests per surface (`.test.ts`, co-located)
- Strengthen `TestHandleCommandExecListContext` to assert SSE payloads (Go, not Playwright)
- Go test asserting `RELA_ENTITY_ID` is **absent** (not empty) for global input (RR-I3VLJV)
- Extend `e2e/tests/fixtures.ts` `DATA_ENTRY_YAML`; add command-button selectors to page objects; e2e specs clicking commands on dashboard and list

**Out of scope:**

- **`view` context** — deferred, see above; also defers the `available_on.views` defect
- All ACL/permission work → TKT-MJ02AO
- Lua command backend (TKT-XTNED)
- Conditional/permission-based visibility beyond the server-side filter TKT-MJ02AO provides
- Changes to command config schema, SSE protocol, or backend runtime behavior
- API-level Playwright specs — prohibited by `e2e/tests/AGENTS.md:45`, already covered in Go

## Dependency: TKT-MJ02AO

**TKT-MJ02AO must land first.** It adds per-command `permission:` gating, 403s
unauthorized exec, denies all command exec under `ReadOnlyACL`, and filters
`resolveCommands` by held permission.

Consequences here:

1. **Button visibility comes for free** — `GET /_commands` returns only
executable commands, so `CommandButtons.vue` needs no client-side ACL logic. Do
**not** add client-side permission checks; the server filter is the single
source of truth and the 403 is the boundary.
2. **The read-only assertion becomes writable** — this ticket can then assert
that no command buttons render under `--read-only`, which is false today.

Shipping this ticket first would put an ungated shell-exec button on the
dashboard, the first page every user lands on.

## Relationship to TKT-XTNED

TKT-XTNED ("Add Lua backend to data-entry commands") lists "one end-to-end test
per `context` type (entity, list, view, global)" as an acceptance criterion.
Only the **UI-interaction half** depends on this ticket — behavioral coverage
already exists in Go. With `view` deferred here, XTNED's view e2e criterion
stays blocked; narrow it or sequence it behind view support.

## Acceptance criteria

- A command with `context: global` and `available_on.dashboard: true` renders a button on the dashboard and executes with `buildGlobalInput` context
- A command with `context: list` renders on list views; `available_on.lists` scoping filters correctly by list ID (assert the **resolved command list**, never mock call arguments — per RR-N643V4)
- `CommandModal` no longer requires an `entity-id`; dashboard and list commands open it without one
- Existing entity-detail command behavior is unchanged — button row, overflow menu, streaming, abort-on-navigate
- Unmounting mid-stream aborts the exec fetch; Close is enabled while a command runs
- `TestHandleCommandExecListContext` asserts streamed SSE payloads, not just HTTP 200
- `RELA_ENTITY_ID` / `RELA_ENTITY_TYPE` are absent (not empty) for global-context input
- No command buttons render under `rela-server --read-only` (**requires TKT-MJ02AO**)
- Frontend unit tests per surface, plus e2e specs clicking a command button on dashboard and on list
