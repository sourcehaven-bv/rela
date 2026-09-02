---
id: IMPL-OTEBIM
type: implementation-checklist
title: 'Implementation: Replace producer-side entitymanager.EntityManager with per-consumer interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A with one exception, below: this
ticket adds no runtime code — it narrows the static type of existing fields. The
"test" for a type-narrowing refactor is the compiler, which rejects a consumer
calling a method it did not declare.)
- [x] Integration tests written (test full flow, not just units) — one added,
and it earns its place: `internal/cli/sync_applier_assert_test.go` pins that
`*entitymanager.Manager` held as the new `entityWriter` still type-asserts to
`syncclient.LocalApplier`. That assertion in `buildSyncEngine` falls back to a
**nil applier**, which disables `rela sync pull` *silently* — so a compile-clean
narrowing could have broken sync with nothing failing. This was the one
genuinely risky spot in the refactor and it is now guarded.
- [x] Happy path implemented — all four subsystems converted, interface deleted.
- [x] Edge cases from planning handled — see Manual Verification.
- [x] ~~Error handling in place~~ (N/A: no new error paths; every method keeps
its identical signature and implementation.)

## Test Quality

- [x] ~~Fixture builders / factories~~ (N/A: the one new test constructs a typed
nil pointer, which is the whole fixture.)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test — the new test asserts one
thing (the assertion still succeeds) and its failure message says what breaks in
production, not just that a bool was false.
- [x] ~~Interpolated values constructed from objects~~ (N/A)
- [x] ~~Property comparisons use original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*AC1 — interface gone.* `grep -rn "entitymanager.EntityManager" internal/ cmd/`
returns **zero** matches, including comments and `_test.go`.

*AC2 — narrow interfaces at each consumer.* Six declared, method counts matching
the measured call sets exactly:

| Interface | Where | Methods |
|---|---|---|
| `attachment.EntityUpdater` | `attachment/attachment.go` | 1 |
| `mcp.EntityWriter` | `mcp/server.go` | 6 |
| `cli.entityWriter` | `cli/cli_wiring.go` | 8 |
| `dataentry.appEntityWriter` | `dataentry/app.go` | 7 |
| `dataentry.entityMutator` | `dataentry/write_handler.go` | 7 |
| `dataentry.entityProvisioner` | `dataentry/provision.go` | 1 |

`appbuild` now hands out the concrete `*entitymanager.Manager`, so every one of
these is satisfied structurally with no adapter and no wiring change.

*AC3 — build + tests.* `go build ./...` clean; **`go test ./...` exit 0, no
failures**. Also built under all four backend tags (default, `postgres`,
`memorybackend`, `sqlite`) since `appbuild`'s recipes differ per tag — all
clean.

*AC4 — arch-lint.* `just arch-lint` → `OK - No warnings found`.

*AC5 — no behaviour change.* `git status` shows **no existing `_test.go`
modified**; the only test file is the new one above. No assertion was adjusted
to accommodate the refactor.

*AC6 — coupling actually dropped.* The concrete win, measured:

```
go list -deps ./internal/attachment | grep -c internal/entitymanager  → 0
go list -deps ./internal/mcp        | grep -c internal/entitymanager  → 0
```

Both packages previously imported it; neither does now.

*End-to-end exercise of every narrowed write path against a real project* (a
copy of `tickets/`), not just compilation:

- **CLI (8/8)** — `create` ✓, `update` (PatchEntity) ✓, `link` (CreateRelation) ✓,
`unlink` (DeleteRelation) ✓, `rename id` (RenameEntity) ✓, `delete` ✓. Relation
update and UpdateEntity are covered by the package tests.
- **MCP (6/6)** — driven over real JSON-RPC stdio against `rela mcp`, full
`initialize` handshake then `tools/call`: `create_entity` ✓, `update_entity`
(PatchEntity) ✓, `create_relation` ✓, `delete_relation` ✓, `rename_entity` ✓,
`delete_entity` ✓. All returned success results, no `isError`.
- **dataentry** — the 25s `internal/dataentry` suite (the widest consumer,
including the CalDAV write path, history restore and the provisioning seam)
passes unchanged.

*Compiler-caught design constraint, and how code review resolved it.* The first
version of `appEntityWriter` omitted `ValidateCreate` and the build failed: App
is not only a *caller* but the *distributor* — it hands the same value to
`writeHandler`, whose `entityMutator` needs `ValidateCreate`. The initial fix
was to widen App's interface to the union (8 methods), which review correctly
called out (RR-NEE4FC) as reintroducing "wide because it is a distributor" one
level down.

Resolved properly instead: `NewApp` now takes the **concrete**
`*entitymanager.Manager` — it is a composition root, and `appbuild` hands it the
concrete type anyway — and passes that to the sub-handler constructors, each of
which narrows at its own field. `appEntityWriter` is now exactly 7 methods,
verified equal to App's 7 direct calls with no slack.

## Quality

- [x] Code follows project patterns — modelled on `lua.Mutator`
(`internal/lua/deps.go`), which narrowed *this same interface* for the Lua
bindings in TKT-IF37, and on dataentry's ~17 existing consumer-side interfaces
(`metaView`, `storeWatcher`, `analyzeReader`). Each new interface's godoc says
which methods are absent **and why**, matching `lua.Mutator`'s "Six methods —
RenameEntity and UpdateRelation are intentionally absent because no Lua binding
invokes them."
- [x] Checked for DRY opportunities — deliberately **not** extracted. The six
interfaces overlap heavily, and a shared base would recreate the producer-side
interface this ticket deletes, one level down. Each consumer declaring its own
list is the point, not duplication to factor out.
- [x] No security issues introduced — narrowing removes methods, never gates.
Every retained method keeps its identical signature and implementation, so ACL,
audit and validation in `entitymanager.Manager` are untouched, and no consumer
gained access to anything it did not already have.
`entitymanagertest.PanicOnUse` deliberately keeps all nine methods so it still
satisfies every narrow interface — dropping one would have silently un-wired
that safety net.
- [x] No silent failures — the one place a silent failure was *possible* (the
sync applier's nil fallback) is now covered by a regression test.
- [x] No debug code left behind.

**Stale comments corrected** (each named the deleted type or called it
"broad"/"wide"): `appbuild.go:433`, `cli/sync.go:104`, `cli/sync/engine.go:15`,
`dataentry/attachment_handler.go:19`, `dataentry/services.go:20`,
`dataentry/settings_handlers.go:85` (a broken `[entitymanager.EntityManager]`
doclink — the `doclink` rule is a blocking CI gate), plus the package doc and
`manager.go`'s compile-time assertion block.
