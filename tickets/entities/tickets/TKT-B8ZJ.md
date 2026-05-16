---
id: TKT-B8ZJ
type: ticket
title: Decouple dataentry from internal/workspace type imports
kind: refactor
priority: medium
effort: s
status: done
---

## Summary

Narrow precursor to TKT-9JEI. `internal/dataentry` imports
`workspace.WatchOptions` / `workspace.ChangeEvent` for the watcher signature.
Define a consumer-side `WatchOptions` in dataentry using `storage.ChangeEvent`
directly; the wiring sites bridge to `workspace.WatchOptions`. After this lands,
`internal/dataentry` no longer imports `internal/workspace` at all (tests still
construct workspace as a fixture — that's fine).

## In scope

- New `dataentry.WatchOptions` type (mirrors `workspace.WatchOptions` but uses `storage.ChangeEvent` for OnChange — consumer-side interfaces at the call site per CLAUDE.md).
- `dataentry.App.startWatching` field signature changes to `func(WatchOptions) error`.
- `cmd/rela-server/main.go` + `cmd/rela-desktop/main.go` wrap `ws.StartWatching` with an adapter that bridges types.
- `internal/dataentry/app.go` drops `internal/workspace` import.
- `.go-arch-lint.yml` removes `dataentry → workspace`.
- Tests pass.

## Out of scope

- Full TKT-9JEI scope (wiring dataentry without `workspace.Discover` at all — that requires extracting the indexer/watcher orchestration). This ticket is the type-decoupling slice.

## Why

Three lifts (TKT-2W0X / TKT-04YA / TKT-B01S) just removed workspace facade
methods. dataentry's remaining tie was the watcher-options type. Cutting it now
keeps the post-lift arch-lint matrix honest (cli ✗ workspace; dataentry ✗
workspace in production code).
