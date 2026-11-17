---
id: TKT-S7KB1WU
type: ticket
title: Wire the ingest finder as a typed dependency
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

TKT-GIAGLQ3 wires the ingest find through `visibility.HolderLookupFor(reader)`.
That function maps the wired `lua.EntityReader` to a holder lookup by its
concrete type: `*visibility.ScriptReader` gives the gated lookup,
`*visibility.UnrestrictedReader` the ungated one, anything else a refusing
lookup. It fails closed, but a future decorator around the script reader would
make every webhook and feed delivery fail with `ErrReaderUnavailable`.

`appbuild.scriptReads` already builds the matching `*visibility.HolderLookup` on
the same tier as the reader. Carry it to the two ingest wiring sites
(`internal/dataentry/webhook_routes.go`, `internal/scheduler/subscriptions.go`)
as a typed dependency, for example a consumer-side finder field next to
`VisibleReader`, and delete `HolderLookupFor`. Every site that sets
`lua.ReadDeps.VisibleReader` (about seven, including
`dataentry.App.luaWriteDeps`) must set it too, which is why it was left out of
TKT-GIAGLQ3.

## Acceptance

- `HolderLookupFor` is gone; the ingest finder comes from the same place as the reader.
- A nil finder still fails `ingest.Deps` validation.
