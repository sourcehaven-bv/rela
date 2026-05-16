---
id: TKT-2MY1
type: ticket
title: dataentry owns its data-entry.yaml watcher (remove WatchOptions indirection)
kind: refactor
priority: medium
effort: s
status: ready
---

## Summary

Remove the `dataentry.WatchOptions` indirection introduced in TKT-B8ZJ.
dataentry already owns part of its watch story (it has its own `stopConfigWatch`
field and a `data-entry.yaml` subscription); finish the job so dataentry
constructs its `storage.NewWatcher` over `data-entry.yaml` directly. The
`startWatching` constructor parameter on `dataentry.NewApp` is removed.

Precursor for TKT-9JEI. Independent and reviewable.

## Background

Workspace's `StartWatching(opts WatchOptions)` accepted a generic
`ExtraFiles`/`ExtraDirs` list with an `OnChange` callback — a god-bundle: any
file, any callback. TKT-B8ZJ moved the type definition into dataentry so
dataentry stopped *importing* workspace, but the indirection itself remained:
dataentry's `App.startWatching` is still a function the wiring site supplies,
which today just bridges to `ws.StartWatching`.

The right architecture: each domain owns its own watcher. `data-entry.yaml`
lives in dataentry's scope; dataentry should construct `storage.NewWatcher`
directly.

## In scope

- `dataentry.App.StartWatching` constructs a `storage.NewWatcher` over `data-entry.yaml` using its own `fs` and `paths`.
- `dataentry.App.startWatching` field deleted.
- `dataentry.WatchOptions` type deleted.
- `dataentry.NewApp` constructor's `startWatching func(WatchOptions) error` parameter removed.
- `cmd/rela-server/main.go` and `cmd/rela-desktop/main.go` adapter closures (introduced in TKT-B8ZJ) deleted.
- `internal/dataentry/test_helpers_test.go::rebindApp` adapter inline deleted.
- Workspace's `StartWatching` may still exist (used by tests + the legacy CLI path); that's out of scope here.
- Tests pass.

## Out of scope

- Metamodel watcher (today doc-comment says live reloads aren't supported; "restart to apply" — no metamodel watcher is needed).
- Workspace's `StartWatching` / `WatchOptions` types themselves. Workspace is going away in TKT-64R3; until then it keeps its legacy surface for tests that still construct workspace as a fixture.
- TKT-9JEI (appbuild migration).

## Why

After this lands:
- Three watchers, three owners: fsstore watches its own store dir; dataentry watches data-entry.yaml; (future) metamodel watches metamodel.yaml.
- No central watch dispatcher.
- `dataentry.NewApp` signature shrinks by one parameter.
- The wiring site at rela-server / rela-desktop stops carrying watcher-bridging boilerplate.

## Risks

- **Goroutine leak.** Today `App.StopWatching` releases `stopConfigWatch`; need to make sure the new owned watcher is also released on shutdown.
- **Test fixtures.** Some dataentry tests today inject a fake `startWatching` to assert behavior; they'll need to adapt to dataentry constructing the watcher itself (or skip watcher behavior in unit tests and exercise it via integration).
- **Per-project lifecycle in rela-desktop.** Desktop loads multiple projects with cancellable contexts; verify dataentry's owned watcher is properly cleaned up per-project.

## Acceptance criteria

1. `dataentry.App.StartWatching` constructs `storage.NewWatcher` over `data-entry.yaml` using `a.fs` and `a.paths`. Observer reactions stay identical to today's behavior.
2. `dataentry.App.startWatching` field deleted.
3. `dataentry.WatchOptions` type deleted.
4. `dataentry.NewApp` signature: no `startWatching` parameter.
5. `cmd/rela-server/main.go` and `cmd/rela-desktop/main.go` adapter closures deleted.
6. `dataentry.App.StopWatching` releases both the config watcher and (no longer) the workspace-supplied one — confirm clean release.
7. `go test -race ./...` clean. `just ci` green.
